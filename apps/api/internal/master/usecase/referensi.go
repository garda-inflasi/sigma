package usecase

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

// ErrPadaTidakValid: parameter pada bukan RFC3339 maupun YYYY-MM-DD.
var ErrPadaTidakValid = errors.New("pada harus RFC3339 (2026-03-01T10:00:00+07:00) atau YYYY-MM-DD")

// wib adalah zona waktu Asia/Jakarta (UTC+7, tanpa DST). Zona tetap dipakai agar tidak bergantung pada tzdata.
var wib = time.FixedZone("WIB", 7*60*60)

var (
	kodeKecamatanRe = regexp.MustCompile(`^[0-9]{7}$`)
	kodeWilayahRe   = regexp.MustCompile(`^[0-9]{4}$`)
)

// maksKode adalah panjang kolom kode komoditas dan id pasar (varchar(255)).
const maksKode = 255

// UseCaseReferensi menangani data referensi master: kecamatan, satuan, pedagang, detail dengan riwayat versi, dan
// penghapusan master yang belum dirujuk.
type UseCaseReferensi struct {
	repo  domain.RepositoryReferensi
	audit audit.Writer
	log   *logger.Logger
}

// NewReferensi membuat usecase data referensi master.
func NewReferensi(repo domain.RepositoryReferensi, auditWriter audit.Writer, log *logger.Logger) *UseCaseReferensi {
	return &UseCaseReferensi{repo: repo, audit: auditWriter, log: log}
}

func (uc *UseCaseReferensi) ListKecamatan(ctx context.Context, kabupatenKode string, q domain.ListQuery) ([]domain.KecamatanRef, int64, error) {
	return uc.repo.ListKecamatan(ctx, kabupatenKode, q)
}

// GetKecamatan mengembalikan ErrTidakDitemukan untuk kode yang salah bentuk tanpa menyentuh basis data.
func (uc *UseCaseReferensi) GetKecamatan(ctx context.Context, kode string) (domain.KecamatanRef, error) {
	if !kodeKecamatanRe.MatchString(kode) {
		return domain.KecamatanRef{}, domain.ErrTidakDitemukan
	}
	return uc.repo.GetKecamatan(ctx, kode)
}

func (uc *UseCaseReferensi) ListSatuan(ctx context.Context, q domain.ListQuery) ([]domain.SatuanRef, int64, error) {
	return uc.repo.ListSatuan(ctx, q)
}

func (uc *UseCaseReferensi) ListPedagang(ctx context.Context, f domain.PedagangFilter, q domain.ListQuery) ([]domain.PedagangRef, int64, error) {
	return uc.repo.ListPedagang(ctx, f, q)
}

// GetKomoditas mengembalikan komoditas beserta riwayat versinya (terbaru dulu).
func (uc *UseCaseReferensi) GetKomoditas(ctx context.Context, kode string) (domain.KomoditasDetail, error) {
	if !BentukKodeValid(kode) {
		return domain.KomoditasDetail{}, domain.ErrTidakDitemukan
	}
	k, err := uc.repo.GetKomoditas(ctx, kode)
	if err != nil {
		return domain.KomoditasDetail{}, err
	}
	riwayat, err := uc.repo.RiwayatVersiKomoditas(ctx, kode)
	if err != nil {
		return domain.KomoditasDetail{}, err
	}
	return domain.KomoditasDetail{Komoditas: k, RiwayatVersi: riwayat}, nil
}

// RiwayatVersi mengembalikan seluruh versi komoditas, terbaru dulu.
func (uc *UseCaseReferensi) RiwayatVersi(ctx context.Context, kode string) ([]domain.KomoditasVersiRef, error) {
	d, err := uc.GetKomoditas(ctx, kode)
	if err != nil {
		return nil, err
	}
	return d.RiwayatVersi, nil
}

// VersiPada mengembalikan versi komoditas yang berlaku pada waktu pada; lihat domain.PilihVersiPada.
func (uc *UseCaseReferensi) VersiPada(ctx context.Context, kode string, pada time.Time) (domain.KomoditasVersiRef, error) {
	riwayat, err := uc.RiwayatVersi(ctx, kode)
	if err != nil {
		return domain.KomoditasVersiRef{}, err
	}
	v, ok := domain.PilihVersiPada(riwayat, pada)
	if !ok {
		return domain.KomoditasVersiRef{}, domain.ErrTidakDitemukan
	}
	return v, nil
}

func (uc *UseCaseReferensi) GetWilayah(ctx context.Context, kode string) (domain.WilayahDetail, error) {
	if !kodeWilayahRe.MatchString(kode) {
		return domain.WilayahDetail{}, domain.ErrTidakDitemukan
	}
	return uc.repo.GetWilayah(ctx, kode)
}

func (uc *UseCaseReferensi) GetPasar(ctx context.Context, kode string) (domain.PasarDetail, error) {
	if !BentukKodeValid(kode) {
		return domain.PasarDetail{}, domain.ErrTidakDitemukan
	}
	return uc.repo.GetPasar(ctx, kode)
}

// HapusKomoditas menghapus komoditas yang belum dirujuk dan tanpa usulan menunggu, lalu mencatat audit
// HAPUS_MASTER. Galat: ErrTidakDitemukan, ErrAdaUsulan, ErrDirujuk. Pemanggil memeriksa izin master.setujui.
func (uc *UseCaseReferensi) HapusKomoditas(ctx context.Context, kode, aktorID string) error {
	if !BentukKodeValid(kode) {
		return domain.ErrTidakDitemukan
	}
	k, err := uc.repo.HapusKomoditas(ctx, kode)
	if err != nil {
		return err
	}
	uc.catatHapus(ctx, aktorID, domain.EntitasKomoditas, kode, map[string]any{
		"kode": k.Kode, "nama": k.Nama, "kategori": k.Kategori, "satuan": k.Satuan, "harga_acuan": k.HargaAcuan,
		"pilot": k.Pilot, "shelf_life_hari": k.ShelfLifeHari, "versi": k.Versi, "status": k.Status,
	})
	return nil
}

// HapusPasar menghapus pasar yang belum dirujuk dan tanpa usulan menunggu, lalu mencatat audit HAPUS_MASTER.
func (uc *UseCaseReferensi) HapusPasar(ctx context.Context, kode, aktorID string) error {
	if !BentukKodeValid(kode) {
		return domain.ErrTidakDitemukan
	}
	p, err := uc.repo.HapusPasar(ctx, kode)
	if err != nil {
		return err
	}
	uc.catatHapus(ctx, aktorID, domain.EntitasPasar, kode, map[string]any{
		"id": p.ID, "nama": p.Nama, "kabupaten_kode": p.KabupatenKode, "kecamatan_kode": p.KecamatanKode,
		"lat": p.Lat, "lng": p.Lng, "radius_meter": p.RadiusMeter, "aktif": p.Aktif, "versi": p.Versi,
	})
	return nil
}

// catatHapus menulis audit HAPUS_MASTER dengan salinan baris sebelum dihapus. Baris sudah terhapus saat audit
// ditulis, jadi kegagalan mencatat hanya dilaporkan ke log (pola yang sama dengan aksi massal).
func (uc *UseCaseReferensi) catatHapus(ctx context.Context, aktorID, entitas, kode string, sebelum map[string]any) {
	err := uc.audit.Write(ctx, audit.Entry{
		AktorID: aktorID, Aksi: domain.AksiHapusMaster, Entitas: entitas, EntitasID: kode,
		Hasil: audit.HasilBerhasil, Sebelum: sebelum,
	})
	if err != nil {
		uc.log.Warn("master: gagal mencatat audit hapus", map[string]interface{}{"entitas": entitas, "kode": kode, "error": err.Error()})
	}
}

// BentukKodeValid melaporkan apakah kode komoditas atau id pasar mungkin ada: tidak kosong, muat di kolom
// varchar(255), UTF-8 yang sah, tanpa karakter kontrol (NUL ditolak PostgreSQL dengan 22021, jadi disaring di sini).
func BentukKodeValid(kode string) bool {
	if kode == "" || len(kode) > maksKode || !utf8.ValidString(kode) {
		return false
	}
	return !strings.ContainsFunc(kode, unicode.IsControl)
}

// ParsePada membaca parameter pada: RFC3339 apa adanya, atau YYYY-MM-DD yang berarti akhir hari itu menurut WIB
// (23:59:59.999999999 UTC+7).
func ParsePada(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	tanggal, err := time.ParseInLocation("2006-01-02", s, wib)
	if err != nil {
		return time.Time{}, ErrPadaTidakValid
	}
	return time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 23, 59, 59, 999_999_999, wib), nil
}
