package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"gorm.io/gorm"
)

// Lockout akun: satu mekanisme untuk semua tempat yang memeriksa rahasia (kata sandi login, kode OTP
// login, kode pertama setup MFA, kata sandi lama saat ganti kata sandi).
//
//   - Kunci per email ternormalisasi, bukan per pengguna: email terdaftar dan tak terdaftar mendapat
//     jawaban yang sama persis (anti-enumerasi), termasuk waktu jawab (lihat perlambatPalsu).
//   - Setiap percobaan "dipesan" sebelum rahasianya diperiksa (domain.KunciLogin.CatatPercobaan), jadi
//     ledakan permintaan serentak tidak bisa melewati batas. Percobaan ke-MaxFailedAttempts memasang kunci
//     selama LockoutDuration; percobaan selama terkunci ditolak tanpa mengubah apa pun, jadi kunci
//     tidak pernah diperpanjang.
//   - Hitungan dihapus saat login berhasil lengkap (kata sandi dan, bila ada, OTP) dan saat admin
//     membuka kunci. Kata sandi benar yang masih menunggu OTP tidak menghapus hitungan.

// jam adalah sumber waktu lockout; tes menggantinya.
var jam = time.Now

// Alasan kegagalan yang dicatat di login_attempts dan audit.
const (
	alasanAkunTerkunci = "account_locked"
)

// hashPalsu adalah hash bcrypt (cost sama dengan crypto.HashPassword) dari kata sandi yang tidak dimiliki
// siapa pun. Login untuk email tak terdaftar tetap membandingkan kata sandi dengannya, supaya waktu
// jawabnya tidak membedakan email terdaftar dari yang tidak. TestHashPalsuSamaCostDenganHashAsli menjaga
// agar cost-nya selalu sama.
const hashPalsu = "$2a$12$ndVzRYWDt4EzBfTC1f91de8veJZXldubVSgB2guyV8fOWeSACv2xa"

// AkunTerkunciError dikembalikan saat akun terkunci. errors.Is(err, ErrAccountLocked) tetap berlaku.
type AkunTerkunciError struct {
	// Sampai adalah akhir kunci.
	Sampai time.Time
	// SisaDetik adalah sisa kunci pada saat penolakan, dibulatkan ke atas (minimal 1).
	SisaDetik int
}

func (e *AkunTerkunciError) Error() string { return ErrAccountLocked.Error() }

func (e *AkunTerkunciError) Is(target error) bool { return target == ErrAccountLocked }

// sisaDetik membulatkan ke atas sisa waktu sampai akhir kunci; 0 bila sudah lewat.
func sisaDetik(sampai, now time.Time) int {
	sisa := sampai.Sub(now)
	if sisa <= 0 {
		return 0
	}
	return int(math.Ceil(sisa.Seconds()))
}

// NormalisasiEmail adalah bentuk email yang dipakai sebagai kunci lockout: huruf kecil tanpa spasi tepi
// (pencarian pengguna juga tak peka huruf besar-kecil).
func NormalisasiEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// HashEmailKunci adalah kunci baris kunci_login: SHA-256 heksadesimal dari email ternormalisasi.
func HashEmailKunci(email string) string {
	return sha256Hex(NormalisasiEmail(email))
}

// batasKunci mengembalikan batas dan durasi kunci; nilai konfigurasi yang nol atau negatif memakai bawaan
// (lockout tidak bisa dimatikan secara tidak sengaja).
func (uc *UseCase) batasKunci() (int, time.Duration) {
	def := DefaultConfig()
	maks, durasi := uc.cfg.MaxFailedAttempts, uc.cfg.LockoutDuration
	if maks <= 0 {
		maks = def.MaxFailedAttempts
	}
	if durasi <= 0 {
		durasi = def.LockoutDuration
	}
	return maks, durasi
}

// mulaiPercobaan memesan satu percobaan memeriksa rahasia untuk email ini. Mengembalikan hash kunci untuk
// KembalikanPercobaan, atau *AkunTerkunciError bila akun terkunci (rahasia tidak boleh diperiksa).
// Kesalahan penyimpanan menolak percobaan (gagal tertutup).
func (uc *UseCase) mulaiPercobaan(ctx context.Context, email string) (string, error) {
	hash := HashEmailKunci(email)
	maks, durasi := uc.batasKunci()
	now := jam()

	keadaan, boleh, err := uc.repo.MulaiPercobaan(ctx, hash, now, maks, durasi)
	if err != nil {
		return hash, fmt.Errorf("pesan percobaan login: %w", err)
	}
	if !boleh {
		uc.log.Warn("login: akun terkunci, percobaan ditolak", map[string]interface{}{"sampai": keadaan.TerkunciSampai})
		return hash, &AkunTerkunciError{Sampai: *keadaan.TerkunciSampai, SisaDetik: max(1, sisaDetik(*keadaan.TerkunciSampai, now))}
	}
	if keadaan.Terkunci(now) {
		uc.log.Warn("login: akun terkunci karena percobaan gagal berulang", map[string]interface{}{"gagal": keadaan.Gagal, "sampai": keadaan.TerkunciSampai})
	}
	return hash, nil
}

// kembalikanPercobaan membatalkan percobaan yang dipesan tetapi ternyata bukan tebakan gagal. Kegagalan
// hanya dilaporkan di log: paling buruk hitungan lebih tinggi satu.
func (uc *UseCase) kembalikanPercobaan(ctx context.Context, hash string) {
	maks, _ := uc.batasKunci()
	if err := uc.repo.KembalikanPercobaan(ctx, hash, maks, jam()); err != nil {
		uc.log.Warn("login: gagal mengembalikan percobaan", map[string]interface{}{"error": err.Error()})
	}
}

// resetKunci menghapus hitungan percobaan gagal setelah login berhasil lengkap.
func (uc *UseCase) resetKunci(ctx context.Context, email string) {
	if _, err := uc.repo.HapusKunci(ctx, HashEmailKunci(email)); err != nil {
		uc.log.Warn("login: gagal mereset hitungan percobaan", map[string]interface{}{"error": err.Error()})
	}
}

// auditTerkunci mencatat percobaan yang ditolak karena terkunci. Akunnya dicari hanya untuk mengisi aktor
// audit (jejak per pengguna); pencarian ini dilakukan untuk email terdaftar maupun tidak, jadi tidak
// membedakan keduanya.
func (uc *UseCase) auditTerkunci(ctx context.Context, email string) {
	aktorID := ""
	if user, err := uc.repo.GetUserByEmail(ctx, email); err == nil {
		aktorID = user.ID
	}
	uc.recordFailedLogin(ctx, aktorID, email, alasanAkunTerkunci)
}

// perlambatPalsu membandingkan kata sandi dengan hash tak bertuan: biayanya sama dengan memeriksa kata
// sandi pengguna sungguhan.
func perlambatPalsu(password string) {
	_ = crypto.VerifyPassword(password, hashPalsu)
}

// ── Admin ─────────────────────────────────────────────────────────────

// BukaKunciResult adalah hasil buka-kunci oleh admin.
type BukaKunciResult struct {
	// TerkunciSebelumnya true bila akun memang sedang terkunci ketika dibuka.
	TerkunciSebelumnya bool
}

// BukaKunciPengguna membuka kunci login satu pengguna dan menghapus hitungan percobaan gagalnya. Aman
// dipanggil berulang: akun yang tidak terkunci tidak berubah selain hitungan percobaannya.
func (uc *UseCase) BukaKunciPengguna(ctx context.Context, id, actorID string) (*BukaKunciResult, error) {
	user, err := uc.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	dihapus, err := uc.repo.HapusKunci(ctx, HashEmailKunci(user.Email))
	if err != nil {
		return nil, fmt.Errorf("buka kunci: %w", err)
	}
	hasil := &BukaKunciResult{TerkunciSebelumnya: dihapus != nil && dihapus.Terkunci(jam())}

	metadata := map[string]any{"terkunci_sebelumnya": hasil.TerkunciSebelumnya}
	if dihapus != nil {
		metadata["gagal_dihapus"] = dihapus.Gagal
	}
	uc.writeAudit(ctx, audit.Entry{
		AktorID: actorID, Aksi: "UNLOCK_USER", Entitas: "pengguna", EntitasID: user.ID,
		Hasil: audit.HasilBerhasil, Metadata: metadata,
	})
	uc.log.Info("buka kunci akun", map[string]interface{}{"user_id": user.ID, "aktor": actorID, "terkunci": hasil.TerkunciSebelumnya})
	return hasil, nil
}

// KunciPengguna mengembalikan akhir kunci login untuk pengguna yang sedang terkunci, dengan id pengguna
// sebagai kunci peta; pengguna yang tidak terkunci tidak ada di peta.
func (uc *UseCase) KunciPengguna(ctx context.Context, users []domain.Pengguna) (map[string]time.Time, error) {
	if len(users) == 0 {
		return map[string]time.Time{}, nil
	}
	hashes := make([]string, len(users))
	idDariHash := make(map[string]string, len(users))
	for i := range users {
		hashes[i] = HashEmailKunci(users[i].Email)
		idDariHash[hashes[i]] = users[i].ID
	}
	aktif, err := uc.repo.KunciAktif(ctx, hashes, jam())
	if err != nil {
		return nil, fmt.Errorf("baca kunci pengguna: %w", err)
	}
	hasil := make(map[string]time.Time, len(aktif))
	for hash, sampai := range aktif {
		hasil[idDariHash[hash]] = sampai
	}
	return hasil, nil
}
