package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/garda-inflasi/api/internal/audit/arsip"
	"github.com/garda-inflasi/api/internal/audit/domain"
)

// Batas kebijakan retensi dan jendela partisi.
const (
	// RetensiMinimalBulan sama dengan batas di audit_log_lepas_partisi.
	RetensiMinimalBulan  = 12
	RetensiMaksimalBulan = 1200
	PartisiKeDepanMaks   = 24
)

// Integritas memelihara rantai hash, partisi, dan arsip audit_log (03-C). Dipakai worker (job berkala) dan API
// (ringkasan status).
type Integritas struct {
	repo   domain.RepositoryIntegritas
	writer domain.Writer
	now    func() time.Time
}

// NewIntegritas membuat usecase integritas; writer dipakai untuk mencatat temuan dan arsip ke audit trail.
func NewIntegritas(repo domain.RepositoryIntegritas, writer domain.Writer) *Integritas {
	return &Integritas{repo: repo, writer: writer, now: time.Now}
}

// Verifikasi menghitung ulang seluruh rantai hash (termasuk sambungannya ke segmen yang sudah diarsipkan), menyimpan
// hasilnya, dan mencatat temuan perusakan ke audit trail. Kegagalan menjalankan verifikasi juga disimpan (GAGAL) lalu
// dikembalikan sebagai galat.
func (uc *Integritas) Verifikasi(ctx context.Context, pemicu string) (domain.Verifikasi, error) {
	mulai := uc.now()
	var v *domain.Verifikator
	errPindai := uc.repo.DenganKunciPemeliharaan(ctx, func(ctx context.Context) error {
		return uc.repo.PindaiRantai(ctx,
			func(segmen []domain.SegmenArsip) error {
				v = domain.NewVerifikator(segmen)
				return nil
			},
			func(row domain.AuditLog) error { return v.Periksa(row) })
	})

	hasil := domain.Verifikasi{Mulai: mulai, Pemicu: pemicu}
	if errPindai != nil {
		hasil.Status = domain.StatusGagal
		hasil.Keterangan = new(errPindai.Error())
	} else {
		head := v.Selesai()
		hasil.Status = domain.StatusValid
		if v.JumlahMasalah > 0 {
			hasil.Status = domain.StatusRusak
		}
		hasil.JumlahBaris, hasil.JumlahSegmen, hasil.JumlahMasalah = v.JumlahBaris, v.JumlahSegmen, v.JumlahMasalah
		if v.JumlahBaris > 0 {
			hasil.IDAwal, hasil.IDAkhir = new(v.IDAwal), new(v.IDAkhir)
		}
		hasil.HashAkhir = head
		if len(v.Masalah) > 0 {
			raw, err := json.Marshal(v.Masalah)
			if err != nil {
				return hasil, fmt.Errorf("encode masalah verifikasi: %w", err)
			}
			hasil.Masalah = new(string(raw))
		}
	}
	hasil.Selesai = uc.now()

	// Simpan dengan context terpisah: hasil verifikasi yang terpotong timeout tetap tercatat sebagai GAGAL.
	simpanCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := uc.repo.SimpanVerifikasi(simpanCtx, &hasil); err != nil {
		return hasil, errors.Join(errPindai, err)
	}
	if hasil.Status == domain.StatusRusak {
		metadata := map[string]any{"verifikasi_id": hasil.ID.String(), "jumlah_masalah": hasil.JumlahMasalah}
		if len(v.Masalah) > 0 {
			metadata["masalah_pertama"] = v.Masalah[0]
		}
		if err := uc.writer.Write(simpanCtx, domain.Entry{
			Aksi: "VERIFIKASI_AUDIT", Entitas: "audit_log", EntitasID: hasil.ID.String(),
			Hasil: domain.HasilGagal, Metadata: metadata,
		}); err != nil {
			return hasil, fmt.Errorf("catat temuan verifikasi: %w", err)
		}
	}
	return hasil, errPindai
}

// SiapkanPartisi memastikan partisi bulan ini dan keDepan bulan berikutnya ada; mengembalikan nama yang baru dibuat.
func (uc *Integritas) SiapkanPartisi(ctx context.Context, keDepan int) ([]string, error) {
	if keDepan < 0 || keDepan > PartisiKeDepanMaks {
		return nil, fmt.Errorf("partisi ke depan harus 0..%d bulan", PartisiKeDepanMaks)
	}
	return uc.repo.SiapkanPartisi(ctx, uc.now(), keDepan+1)
}

// KonfigRetensi adalah kebijakan masa simpan audit (UU PDP: simpan selama diperlukan, lalu pindahkan).
type KonfigRetensi struct {
	// Bulan adalah masa simpan di basis data; partisi yang seluruh rentangnya lebih tua dilepas ke arsip.
	Bulan int
	// Dir adalah direktori penyimpanan dingin untuk berkas arsip.
	Dir string
}

// Validasi memeriksa kebijakan retensi.
func (k KonfigRetensi) Validasi() error {
	if k.Bulan < RetensiMinimalBulan || k.Bulan > RetensiMaksimalBulan {
		return fmt.Errorf("masa retensi audit harus %d..%d bulan", RetensiMinimalBulan, RetensiMaksimalBulan)
	}
	if k.Dir == "" {
		return errors.New("direktori arsip audit wajib diisi")
	}
	return nil
}

// HasilArsip melaporkan satu partisi yang dilepas ke arsip.
type HasilArsip struct {
	ArsipID     int64
	Partisi     string
	JumlahBaris int64
	Berkas      string
}

// Retensi mengarsipkan setiap partisi yang seluruh rentangnya lebih tua dari masa retensi, terlama lebih dulu:
// baris diekspor ke berkas (hash tiap baris diperiksa ulang), basis data memeriksa ulang bukti ekspor lalu melepas
// partisinya, kemudian manifest ditulis. Manifest yang belum sempat ditulis pada putaran sebelumnya dilengkapi dulu.
func (uc *Integritas) Retensi(ctx context.Context, cfg KonfigRetensi) ([]HasilArsip, error) {
	if err := cfg.Validasi(); err != nil {
		return nil, err
	}
	var hasil []HasilArsip
	err := uc.repo.DenganKunciPemeliharaan(ctx, func(ctx context.Context) error {
		if err := uc.lengkapiManifest(ctx, cfg.Dir); err != nil {
			return err
		}
		partisi, err := uc.repo.DaftarPartisi(ctx)
		if err != nil {
			return err
		}
		batas := awalBulanUTC(uc.now()).AddDate(0, -cfg.Bulan, 0)
		// DaftarPartisi terbaru lebih dulu; arsipkan dari yang terlama.
		for i := len(partisi) - 1; i >= 0; i-- {
			p := partisi[i]
			if p.Sampai == nil || p.Sampai.After(batas) {
				continue
			}
			h, err := uc.arsipkan(ctx, p, cfg)
			if err != nil {
				return err
			}
			hasil = append(hasil, h)
		}
		return nil
	})
	return hasil, err
}

func (uc *Integritas) arsipkan(ctx context.Context, p domain.Partisi, cfg KonfigRetensi) (HasilArsip, error) {
	namaData := arsip.NamaBerkasData(p.Nama)
	penulis, err := arsip.BuatPenulis(filepath.Join(cfg.Dir, namaData))
	if err != nil {
		return HasilArsip{}, err
	}
	err = uc.repo.BarisRentang(ctx, *p.Dari, *p.Sampai, func(row domain.AuditLog) error {
		// Baris yang sudah dirusak tidak diarsipkan: perusakan harus diselidiki selagi barisnya masih di basis data.
		if !bytes.Equal(domain.HitungHash(row.HashSebelumnya, row), row.Hash) {
			return fmt.Errorf("baris audit %s di %s tidak cocok dengan hash-nya; arsip dibatalkan", row.ID, p.Nama)
		}
		return penulis.Tulis(row)
	})
	if err != nil {
		penulis.Batal()
		return HasilArsip{}, err
	}
	sha, err := penulis.Tutup()
	if err != nil {
		return HasilArsip{}, err
	}

	arsipID, err := uc.repo.LepasPartisi(ctx, domain.LepasPartisi{
		Partisi: p.Nama, RetensiBulan: cfg.Bulan, JumlahBaris: penulis.JumlahBaris(), Ringkasan: penulis.Ringkasan(),
		Berkas: namaData, SHA256Berkas: sha,
	})
	if err != nil {
		// Partisi tetap di basis data; berkas data ditimpa pada percobaan berikutnya.
		return HasilArsip{}, err
	}
	if err := uc.tulisManifest(ctx, cfg.Dir, arsipID); err != nil {
		return HasilArsip{}, err
	}
	h := HasilArsip{ArsipID: arsipID, Partisi: p.Nama, JumlahBaris: penulis.JumlahBaris(), Berkas: namaData}
	if err := uc.writer.Write(ctx, domain.Entry{
		Aksi: "ARSIP_AUDIT", Entitas: "audit_log", EntitasID: p.Nama, Hasil: domain.HasilBerhasil,
		Metadata: map[string]any{
			"arsip_id": fmt.Sprint(arsipID), "berkas": namaData, "sha256_berkas": sha,
			"jumlah_baris": penulis.JumlahBaris(), "retensi_bulan": cfg.Bulan,
		},
	}); err != nil {
		return h, fmt.Errorf("catat arsip audit: %w", err)
	}
	return h, nil
}

// lengkapiManifest menulis manifest arsip yang belum ada di direktori (mis. proses mati sesudah partisi dilepas).
func (uc *Integritas) lengkapiManifest(ctx context.Context, dir string) error {
	daftar, err := uc.repo.DaftarArsip(ctx)
	if err != nil {
		return err
	}
	for _, a := range daftar {
		if _, err := os.Stat(filepath.Join(dir, arsip.NamaBerkasManifest(a.Partisi))); err == nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, a.Berkas)); err != nil {
			continue // berkas data dipindahkan ke tempat lain; tidak ada yang bisa dilengkapi di sini
		}
		if err := uc.tulisManifest(ctx, dir, int64(a.ID)); err != nil {
			return err
		}
	}
	return nil
}

func (uc *Integritas) tulisManifest(ctx context.Context, dir string, arsipID int64) error {
	daftar, err := uc.repo.DaftarArsip(ctx)
	if err != nil {
		return err
	}
	for _, a := range daftar {
		if int64(a.ID) != arsipID {
			continue
		}
		segmen, err := uc.repo.SegmenArsip(ctx, arsipID)
		if err != nil {
			return err
		}
		return arsip.TulisManifest(filepath.Join(dir, arsip.NamaBerkasManifest(a.Partisi)), arsip.ManifestDari(a, segmen))
	}
	return fmt.Errorf("arsip audit %d tidak ditemukan", arsipID)
}

// RingkasanIntegritas adalah status integritas audit untuk API.
type RingkasanIntegritas struct {
	VerifikasiTerakhir *domain.Verifikasi
	Partisi            []domain.Partisi
	Arsip              []domain.Arsip
}

// Ringkasan mengembalikan verifikasi terakhir, partisi aktif, dan arsip.
func (uc *Integritas) Ringkasan(ctx context.Context) (RingkasanIntegritas, error) {
	v, err := uc.repo.VerifikasiTerakhir(ctx)
	if err != nil {
		return RingkasanIntegritas{}, err
	}
	partisi, err := uc.repo.DaftarPartisi(ctx)
	if err != nil {
		return RingkasanIntegritas{}, err
	}
	daftar, err := uc.repo.DaftarArsip(ctx)
	if err != nil {
		return RingkasanIntegritas{}, err
	}
	return RingkasanIntegritas{VerifikasiTerakhir: v, Partisi: partisi, Arsip: daftar}, nil
}

// MasalahVerifikasi membaca daftar masalah yang tersimpan pada satu verifikasi.
func MasalahVerifikasi(v domain.Verifikasi) []domain.Masalah {
	if v.Masalah == nil {
		return nil
	}
	var out []domain.Masalah
	if err := json.Unmarshal([]byte(*v.Masalah), &out); err != nil {
		return nil
	}
	return out
}

// PerluVerifikasi melaporkan apakah verifikasi terjadwal perlu dijalankan: belum pernah, terakhir GAGAL, atau yang
// terakhir dimulai lebih dari jeda yang lalu. Mencegah pemindaian penuh berulang setiap worker dinyalakan ulang.
func (uc *Integritas) PerluVerifikasi(ctx context.Context, jeda time.Duration) (bool, error) {
	v, err := uc.repo.VerifikasiTerakhir(ctx)
	if err != nil {
		return false, err
	}
	return v == nil || v.Status == domain.StatusGagal || uc.now().Sub(v.Mulai) >= jeda, nil
}

func awalBulanUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
