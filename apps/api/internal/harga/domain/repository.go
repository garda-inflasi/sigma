package domain

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound dikembalikan bila entri tidak ada.
	ErrNotFound = errors.New("entri harga tidak ditemukan")
	// ErrReferensiTidakAda dikembalikan bila komoditas atau pasar yang dirujuk tidak ada.
	ErrReferensiTidakAda = errors.New("komoditas atau pasar tidak ditemukan")
)

// WIB adalah zona waktu Aceh; hari pencatatan harga dihitung menurutnya.
var WIB = time.FixedZone("WIB", 7*60*60)

// ListFilter menyaring daftar entri; field kosong berarti tanpa penyaring.
type ListFilter struct {
	Status        string
	KabupatenKode string
	KomoditasKode string
	PasarID       string
	PetugasID     string
	Dari          *time.Time // inklusif
	Sampai        *time.Time // eksklusif
	Page          int
	Limit         int
}

// Ringkasan menghitung entri per status dalam satu jendela waktu.
type Ringkasan struct {
	Draf        int64
	Diajukan    int64
	Divalidasi  int64
	Dipublikasi int64
	Ditolak     int64
	// Anomali adalah entri dengan deviasi mutlak di atas AmbangAnomaliPct.
	Anomali int64
}

// AmbangAnomaliPct adalah deviasi (persen) yang dihitung anomali pada ringkasan.
const AmbangAnomaliPct = 25

// TitikTren adalah rata-rata harga satu periode, dipisah menurut tipe harga.
type TitikTren struct {
	Tanggal  time.Time
	Produsen *float64
	Konsumen *float64
}

// Interval pengelompokan tren.
const (
	IntervalHarian   = "harian"
	IntervalMingguan = "mingguan"
)

type Repository interface {
	List(ctx context.Context, filter ListFilter) ([]EntriView, int64, error)
	GetByID(ctx context.Context, id string) (*EntriView, error)
	Ringkasan(ctx context.Context, sejak time.Time) (Ringkasan, error)
	// Tren merata-ratakan entri berstatus DIVALIDASI atau DIPUBLIKASI di satu kabupaten.
	Tren(ctx context.Context, komoditasKode, kabupatenKode string, sejak time.Time, interval string) ([]TitikTren, error)
	// SatuanKomoditas mengembalikan satuan komoditas dan memastikan pasar aktif ada.
	SatuanKomoditas(ctx context.Context, komoditasKode, pasarID string) (string, error)
	Create(ctx context.Context, entri *EntriHarga) error
	// Transisi menerapkan aksi pada id yang statusnya memenuhi syarat (dan
	// memenuhi pemisahan tugas) lalu mengembalikan id yang benar-benar berubah.
	Transisi(ctx context.Context, ids []string, aksi BulkAction, aktorID string, waktu time.Time) ([]string, error)
}
