package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

var (
	// ErrNotFound dikembalikan bila entri tidak ada.
	ErrNotFound = errors.New("entri harga tidak ditemukan")
	// ErrReferensiTidakAda dikembalikan bila komoditas atau pasar yang dirujuk tidak ada.
	ErrReferensiTidakAda = errors.New("komoditas atau pasar tidak ditemukan")
	// ErrPenggunaTidakAda dikembalikan bila akun pemanggil tidak ada di basis data.
	ErrPenggunaTidakAda = errors.New("pengguna tidak ditemukan")
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
	// Urut: "" (terbaru lebih dulu) atau UrutSkor (skor anomali tertinggi lebih dulu).
	Urut  string
	Page  int
	Limit int
	// Cakupan membatasi baris yang boleh dilihat pemanggil (02-B), digabung (AND) dengan penyaring di atas. Nilai nol
	// tertutup: daftar kosong.
	Cakupan Cakupan
}

// Urutan daftar entri.
const UrutSkor = "skor"

// PenggunaCakupan adalah peran dan kabupaten tugas pengguna yang menentukan Cakupan-nya.
type PenggunaCakupan struct {
	Peran string
	// KabupatenKode kosong untuk pengguna tingkat provinsi.
	KabupatenKode string
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
	ID          idx.ID    `gorm:"column:id"`
	Harga       int       `gorm:"column:harga"`
	DicatatPada time.Time `gorm:"column:dicatat_pada"`
	PetugasID   string    `gorm:"column:petugas_id;type:uuid"`
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
	// GetByID mencari entri menurut ID BIGINT atau kode lama (alias tautan lama); ErrNotFound bila tidak ada.
	GetByID(ctx context.Context, ref idx.RujukanID) (*EntriView, error)
	// Ringkasan menghitung entri di dalam cakupan sejak waktu tertentu.
	Ringkasan(ctx context.Context, sejak time.Time, cakupan Cakupan) (Ringkasan, error)
	// Tren merata-ratakan entri berstatus DIVALIDASI atau DIPUBLIKASI di satu kabupaten.
	Tren(ctx context.Context, komoditasKode, kabupatenKode string, sejak time.Time, interval string) ([]TitikTren, error)
	// SatuanKomoditas mengembalikan satuan komoditas dan memastikan pasar aktif ada.
	SatuanKomoditas(ctx context.Context, komoditasKode, pasarID string) (string, error)
	// KonteksValidasi membaca pembanding validasi otomatis untuk entri yang akan disimpan.
	KonteksValidasi(ctx context.Context, entri *EntriHarga) (KonteksValidasi, error)
	// Simpan menyimpan entri secara idempoten per (petugas, kunci): kiriman ulang
	// mengembalikan entri yang sudah tersimpan. Entri petugas lain yang bentrok
	// menolak penyimpanan kecuali Pengiriman.Pertahankan.
	Simpan(ctx context.Context, kirim Pengiriman) (HasilSimpan, error)
	// Transisi menerapkan aksi dalam satu transaksi pada id yang memenuhi Evaluasi dan
	// mencatat riwayat statusnya. Hasilnya satu per id, urut seperti ids. Entri di luar cakupan tidak dikunci
	// dan tidak diubah; dilaporkan SebabTidakDitemukan, sama dengan entri yang tidak ada.
	Transisi(ctx context.Context, ids []idx.ID, aksi BulkAction, aktorID, alasan string, waktu time.Time, cakupan Cakupan) ([]HasilTransisi, error)
	// PenggunaCakupan membaca peran dan kabupaten tugas pengguna dari basis data (bukan dari klaim token, yang bisa
	// basi sampai access token habis); ErrPenggunaTidakAda bila akunnya tidak ada.
	PenggunaCakupan(ctx context.Context, penggunaID string) (PenggunaCakupan, error)
	// Riwayat mengembalikan perpindahan status entri, yang terlama lebih dulu.
	Riwayat(ctx context.Context, entriID idx.ID) ([]RiwayatView, error)
	// Versi mengembalikan seluruh versi isi entri (tabel entri_harga_versi), nomor terkecil lebih dulu.
	Versi(ctx context.Context, entriID idx.ID) ([]VersiView, error)
	// Lineage melengkapi entri dengan jejak asalnya (pelaku tiap tahap, tugas lapangan, versi, riwayat status, audit).
	Lineage(ctx context.Context, entri EntriView) (Lineage, error)
}
