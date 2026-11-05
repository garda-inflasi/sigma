package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

// Pembersihan adalah pekerjaan rumah tangga tabel autentikasi: baris yang sudah mati dihapus agar
// jwt_blacklist, refresh_tokens, password_reset_tokens, dan login_attempts tidak tumbuh tanpa batas.
// Dijalankan berkala oleh worker (job auth_pembersihan). Idempoten: aman diulang atau berjalan terlambat.
//
// Setiap tabel adalah satu Langkah. Menambah tabel baru (mis. tabel sesi) cukup menambah satu
// Langkah di langkahBawaan beserta metode repositorinya; orkestrasi (batch, batas putaran, log,
// isolasi galat antar langkah) tidak perlu diubah.
type Pembersihan struct {
	langkah []Langkah
	cfg     KonfigPembersihan
	log     *logger.Logger
}

const (
	// MinRetensiLoginAttempt adalah lantai mutlak umur login_attempts: baris yang lebih muda tidak pernah
	// dihapus, berapa pun konfigurasinya. Perhitungan penguncian akun membaca jendela waktu pendek
	// (puluhan menit) dari tabel ini dan tidak boleh kehilangan baris di dalamnya karena salah konfigurasi.
	MinRetensiLoginAttempt = 24 * time.Hour

	// RetensiLoginAttemptBawaan menyimpan jejak percobaan masuk 90 hari untuk investigasi insiden
	// (brute force, pola IP) sebelum dibuang.
	RetensiLoginAttemptBawaan = 90 * 24 * time.Hour

	// RetensiResetTokenBawaan menyimpan token atur ulang yang sudah kedaluwarsa atau dipakai selama 7 hari.
	// Alasannya: ResetPassword membedakan "sudah kedaluwarsa" (pesan jelas) dari "tidak valid atau sudah
	// dipakai" bagi tautan yang tidak ditemukan; tautan email yang baru lewat tetap mendapat pesan yang benar
	// selama pengguna masih membukanya, dan baris dipakai berguna sebagai jejak debug saat pengguna
	// mengeluh tautannya tidak bekerja. Lebih lama dari itu tidak ada manfaatnya.
	RetensiResetTokenBawaan = 7 * 24 * time.Hour

	// UkuranBatchBawaan adalah jumlah baris maksimum satu pernyataan DELETE.
	UkuranBatchBawaan = 1000
	// MaksBatchBawaan membatasi jumlah batch per langkah dalam satu putaran (bawaan 500 x 1000 = 500 ribu
	// baris); sisanya dilanjutkan putaran berikutnya agar satu putaran tidak memonopoli basis data.
	MaksBatchBawaan = 500
)

// KonfigPembersihan mengatur retensi dan ukuran kerja. Gunakan DefaultKonfigPembersihan sebagai dasar.
type KonfigPembersihan struct {
	// RetensiLoginAttempt adalah umur minimum login_attempts sebelum dihapus. Nol = tidak pernah dihapus.
	// Nilai positif di bawah MinRetensiLoginAttempt dinaikkan ke lantai itu.
	RetensiLoginAttempt time.Duration
	// RetensiResetToken adalah jeda sesudah token atur ulang kedaluwarsa atau dipakai sebelum dihapus.
	// Nol = langsung dihapus begitu mati.
	RetensiResetToken time.Duration
	// UkuranBatch adalah baris maksimum per DELETE (<= 0 memakai UkuranBatchBawaan).
	UkuranBatch int
	// MaksBatch adalah batch maksimum per langkah per putaran (<= 0 memakai MaksBatchBawaan).
	MaksBatch int
	// UmurKunciLogin adalah umur minimum (sejak diperbarui) baris kunci_login sebelum dihapus. Nilai di
	// bawah UmurKunciLoginMin dinaikkan ke sana.
	UmurKunciLogin time.Duration
}

// UmurKunciLoginMin adalah dua kali durasi kunci: sesudahnya jendela hitungan dan kuncinya pasti sudah
// berakhir, jadi barisnya tidak lagi memengaruhi login.
var UmurKunciLoginMin = 2 * DefaultConfig().LockoutDuration

// DefaultKonfigPembersihan mengembalikan konfigurasi bawaan.
func DefaultKonfigPembersihan() KonfigPembersihan {
	return KonfigPembersihan{
		RetensiLoginAttempt: RetensiLoginAttemptBawaan,
		RetensiResetToken:   RetensiResetTokenBawaan,
		UkuranBatch:         UkuranBatchBawaan,
		MaksBatch:           MaksBatchBawaan,
		UmurKunciLogin:      UmurKunciLoginMin,
	}
}

// Normalisasi mengisi bawaan dan menegakkan lantai retensi.
func (c KonfigPembersihan) Normalisasi() KonfigPembersihan {
	if c.UkuranBatch <= 0 {
		c.UkuranBatch = UkuranBatchBawaan
	}
	if c.MaksBatch <= 0 {
		c.MaksBatch = MaksBatchBawaan
	}
	if c.UmurKunciLogin < UmurKunciLoginMin {
		c.UmurKunciLogin = UmurKunciLoginMin
	}
	if c.RetensiResetToken < 0 {
		c.RetensiResetToken = 0
	}
	if c.RetensiLoginAttempt < 0 || (c.RetensiLoginAttempt > 0 && c.RetensiLoginAttempt < MinRetensiLoginAttempt) {
		c.RetensiLoginAttempt = MinRetensiLoginAttempt
	}
	return c
}

// Langkah membersihkan satu tabel.
type Langkah struct {
	// Nama dipakai di log dan hasil (nama tabel).
	Nama string
	// HapusBatch menghapus paling banyak batas baris yang sudah mati pada waktu now dan mengembalikan
	// jumlahnya. Dipanggil berulang sampai jumlahnya kurang dari batas.
	HapusBatch func(ctx context.Context, now time.Time, batas int) (int64, error)
}

// HasilLangkah adalah ringkasan satu Langkah dalam satu putaran.
type HasilLangkah struct {
	Nama    string
	Dihapus int64
	Batch   int
	// Terpotong: putaran berhenti karena MaksBatch tercapai; sisa baris (bila ada) dihapus putaran berikutnya.
	Terpotong bool
	// Galat tidak nil bila langkah berhenti karena kegagalan atau ctx selesai; langkah lain tetap dijalankan.
	Galat error
}

// HasilPembersihan adalah ringkasan satu putaran.
type HasilPembersihan struct {
	Langkah []HasilLangkah
}

// Total menjumlahkan baris terhapus semua langkah.
func (h HasilPembersihan) Total() int64 {
	var n int64
	for _, l := range h.Langkah {
		n += l.Dihapus
	}
	return n
}

// NewPembersihan membuat pembersih dengan langkah bawaan (empat tabel) ditambah tambahan bila ada.
// Langkah login_attempts hanya ada bila cfg.RetensiLoginAttempt > 0.
func NewPembersihan(repo domain.PembersihanRepository, cfg KonfigPembersihan, log *logger.Logger, tambahan ...Langkah) *Pembersihan {
	cfg = cfg.Normalisasi()
	langkah := append(langkahBawaan(repo, cfg), tambahan...)
	return &Pembersihan{langkah: langkah, cfg: cfg, log: log}
}

// langkahBawaan adalah daftar langkah pembersihan. Tambahkan tabel baru di sini.
func langkahBawaan(repo domain.PembersihanRepository, cfg KonfigPembersihan) []Langkah {
	langkah := []Langkah{
		{
			Nama: "jwt_blacklist",
			// Pembacaan (IsJWTBlacklisted) hanya menghitung baris yang belum kedaluwarsa, jadi menghapus yang
			// sudah lewat tidak mengubah perilaku apa pun.
			HapusBatch: func(ctx context.Context, now time.Time, batas int) (int64, error) {
				return repo.HapusBlacklistKedaluwarsa(ctx, now, batas)
			},
		},
		{
			Nama: "refresh_tokens",
			// Hanya yang sudah lewat expires_at. Baris dicabut tetapi belum kedaluwarsa dipertahankan karena
			// Refresh memakainya mendeteksi pemakaian ulang token (dan mencabut seluruh keluarga).
			HapusBatch: func(ctx context.Context, now time.Time, batas int) (int64, error) {
				return repo.HapusRefreshTokenKedaluwarsa(ctx, now, batas)
			},
		},
		{
			Nama: "password_reset_tokens",
			HapusBatch: func(ctx context.Context, now time.Time, batas int) (int64, error) {
				return repo.HapusResetTokenMati(ctx, now.Add(-cfg.RetensiResetToken), batas)
			},
		},
	}
	langkah = append(langkah, Langkah{
		Nama: "sso_login_state",
		// State yang belum kedaluwarsa bisa sedang menunggu callback penyedia SSO; hanya yang lewat dihapus.
		HapusBatch: func(ctx context.Context, now time.Time, batas int) (int64, error) {
			return repo.HapusSSOStateKedaluwarsa(ctx, now, batas)
		},
	})
	langkah = append(langkah, Langkah{
		Nama: "kunci_login",
		// Kunci yang masih berlaku tidak pernah dihapus, berapa pun umurnya.
		HapusBatch: func(ctx context.Context, now time.Time, batas int) (int64, error) {
			return repo.HapusKunciLoginUsang(ctx, now.Add(-cfg.UmurKunciLogin), now, batas)
		},
	})
	if cfg.RetensiLoginAttempt > 0 {
		langkah = append(langkah, Langkah{
			Nama: "login_attempts",
			HapusBatch: func(ctx context.Context, now time.Time, batas int) (int64, error) {
				return repo.HapusLoginAttemptLama(ctx, now.Add(-cfg.RetensiLoginAttempt), batas)
			},
		})
	}
	return langkah
}

// Jalankan membersihkan semua tabel terhadap waktu sekarang.
func (p *Pembersihan) Jalankan(ctx context.Context) (HasilPembersihan, error) {
	return p.JalankanPada(ctx, time.Now())
}

// JalankanPada seperti Jalankan dengan waktu acuan eksplisit (untuk tes). Kegagalan satu langkah tidak
// menghentikan langkah lain; galatnya digabung pada nilai kembalian.
func (p *Pembersihan) JalankanPada(ctx context.Context, now time.Time) (HasilPembersihan, error) {
	var hasil HasilPembersihan
	var galat []error
	for _, l := range p.langkah {
		h := p.jalankanLangkah(ctx, l, now)
		hasil.Langkah = append(hasil.Langkah, h)
		if h.Galat != nil {
			galat = append(galat, fmt.Errorf("%s: %w", l.Nama, h.Galat))
		}
	}
	p.catat(hasil)
	return hasil, errors.Join(galat...)
}

func (p *Pembersihan) jalankanLangkah(ctx context.Context, l Langkah, now time.Time) HasilLangkah {
	h := HasilLangkah{Nama: l.Nama}
	for h.Batch < p.cfg.MaksBatch {
		if err := ctx.Err(); err != nil {
			h.Galat = err
			return h
		}
		n, err := l.HapusBatch(ctx, now, p.cfg.UkuranBatch)
		if err != nil {
			h.Galat = err
			return h
		}
		h.Batch++
		h.Dihapus += n
		if n < int64(p.cfg.UkuranBatch) {
			return h // habis
		}
	}
	h.Terpotong = true
	return h
}

func (p *Pembersihan) catat(hasil HasilPembersihan) {
	if p.log == nil {
		return
	}
	fields := map[string]interface{}{"total": hasil.Total()}
	var terpotong, gagal []string
	for _, l := range hasil.Langkah {
		fields[l.Nama] = l.Dihapus
		if l.Terpotong {
			terpotong = append(terpotong, l.Nama)
		}
		if l.Galat != nil {
			gagal = append(gagal, l.Nama)
		}
	}
	if len(terpotong) > 0 {
		fields["terpotong"] = terpotong
	}
	if len(gagal) > 0 {
		fields["gagal"] = gagal
		p.log.Warn("auth: pembersihan selesai sebagian", fields)
		return
	}
	if hasil.Total() > 0 {
		p.log.Info("auth: pembersihan selesai", fields)
		return
	}
	p.log.Debug("auth: pembersihan selesai, tidak ada baris untuk dihapus", fields)
}
