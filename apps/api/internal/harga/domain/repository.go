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

// JendelaBentrok adalah selisih waktu pencatatan terjauh antara dua entri petugas
// berbeda pada pasar, komoditas, dan tipe yang sama agar dianggap membahas harga
// yang sama: harga berbeda di dalam jendela ini adalah bentrok.
const JendelaBentrok = 3 * time.Hour

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

// Pengiriman adalah entri baru beserta cara menanganinya.
type Pengiriman struct {
	Entri *EntriHarga
	// Pertahankan menyimpan entri walau bentrok dengan entri petugas lain
	// (petugas memilih mempertahankan versinya).
	Pertahankan bool
}

// Bentrok adalah entri petugas lain yang harganya berbeda pada pasar, komoditas,
// dan tipe yang sama dalam JendelaBentrok.
type Bentrok struct {
	ID          string    `gorm:"column:id"`
	Harga       int       `gorm:"column:harga"`
	DicatatPada time.Time `gorm:"column:dicatat_pada"`
	PetugasID   string    `gorm:"column:petugas_id"`
	PetugasNama string    `gorm:"column:petugas_nama"`
}

// HasilSimpan melaporkan hasil Repository.Simpan; tepat satu dari tiga keadaan:
// entri baru tersimpan, kiriman ulang (Diulang), atau bentrok.
type HasilSimpan struct {
	Entri   *EntriHarga
	Diulang bool
	Bentrok *Bentrok
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
	// Simpan menyimpan entri secara idempoten per (petugas, kunci): kiriman ulang
	// mengembalikan entri yang sudah tersimpan. Entri petugas lain yang bentrok
	// menolak penyimpanan kecuali Pengiriman.Pertahankan.
	Simpan(ctx context.Context, kirim Pengiriman) (HasilSimpan, error)
	// Transisi menerapkan aksi dalam satu transaksi pada id yang memenuhi Evaluasi dan
	// mencatat riwayat statusnya. Hasilnya satu per id, urut seperti ids.
	Transisi(ctx context.Context, ids []string, aksi BulkAction, aktorID, alasan string, waktu time.Time) ([]HasilTransisi, error)
	// Riwayat mengembalikan perpindahan status entri, yang terlama lebih dulu.
	Riwayat(ctx context.Context, entriID string) ([]RiwayatView, error)
}
