package domain

import (
	"context"
	"errors"
	"time"
)

// ToleransiSuhuC adalah selisih bawaan terhadap suhu target untuk batas alarm
// bila fasilitas belum menetapkan suhu_min_c / suhu_maks_c.
const ToleransiSuhuC = 2.0

// Status lot stok.
const (
	StatusLotAktif         = "AKTIF"
	StatusLotPrioritasJual = "PRIORITAS_JUAL"
	StatusLotHabis         = "HABIS"
)

// Status fasilitas.
const (
	StatusFasilitasOperasional = "OPERASIONAL"
	StatusFasilitasPerawatan   = "PERAWATAN"
	StatusFasilitasRencana     = "RENCANA"
)

// Jenis transaksi stok.
const (
	TransaksiMasuk  = "MASUK"
	TransaksiKeluar = "KELUAR"
	TransaksiPindah = "PINDAH"
)

// Sumber pembacaan telemetri dan transaksi.
const (
	SumberSensor = "SENSOR"
	SumberManual = "MANUAL"
	SumberDemo   = "DEMO"
	SumberApp    = "APLIKASI"
)

// RisikoAwalLot adalah risiko food loss lot baru; penilaian lanjut milik 13-C.
const RisikoAwalLot = 0.05

// Galat operasi. Repositori mengembalikan galat ini apa adanya agar lapisan
// HTTP dapat memetakannya ke status yang tepat.
var (
	ErrFasilitasTidakAda    = errors.New("fasilitas tidak ditemukan")
	ErrFasilitasTidakAktif  = errors.New("fasilitas tidak beroperasi")
	ErrLotTidakAda          = errors.New("lot tidak ditemukan")
	ErrLotHabis             = errors.New("lot sudah habis")
	ErrKomoditasTidakAda    = errors.New("komoditas tidak ditemukan")
	ErrStokTidakCukup       = errors.New("volume melebihi stok lot")
	ErrKapasitasTidakCukup  = errors.New("kapasitas fasilitas tidak cukup")
	ErrKapasitasDiBawahStok = errors.New("kapasitas lebih kecil dari stok tersimpan")
	ErrFasilitasSama        = errors.New("fasilitas asal dan tujuan sama")
	ErrBatasSuhuTidakValid  = errors.New("suhu minimum harus lebih kecil dari suhu maksimum")
)

// BatasSuhu menghitung batas alarm efektif sebuah fasilitas.
func BatasSuhu(target float64, min, maks *float64) (lo, hi float64) {
	lo, hi = target-ToleransiSuhuC, target+ToleransiSuhuC
	if min != nil {
		lo = *min
	}
	if maks != nil {
		hi = *maks
	}
	return lo, hi
}

// DiLuarRentang melaporkan apakah suhu keluar dari [lo, hi].
func DiLuarRentang(suhu, lo, hi float64) bool {
	return suhu < lo || suhu > hi
}

// TelemetriSuhu memetakan tabel telemetri_suhu.
type TelemetriSuhu struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	FasilitasID   string    `gorm:"column:fasilitas_id"`
	Waktu         time.Time `gorm:"column:waktu"`
	SuhuC         float64   `gorm:"column:suhu_c"`
	KelembapanPct *float64  `gorm:"column:kelembapan_pct"`
	Sumber        string    `gorm:"column:sumber"`
}

func (TelemetriSuhu) TableName() string { return "telemetri_suhu" }

// TransaksiStok memetakan tabel transaksi_stok.
type TransaksiStok struct {
	ID                string    `gorm:"column:id;primaryKey"`
	LotID             string    `gorm:"column:lot_id"`
	LotBaruID         *string   `gorm:"column:lot_baru_id"`
	Jenis             string    `gorm:"column:jenis"`
	VolumeTon         float64   `gorm:"column:volume_ton"`
	FasilitasAsalID   *string   `gorm:"column:fasilitas_asal_id"`
	FasilitasTujuanID *string   `gorm:"column:fasilitas_tujuan_id"`
	OlehID            *string   `gorm:"column:oleh_id"`
	Waktu             time.Time `gorm:"column:waktu"`
	Alasan            string    `gorm:"column:alasan"`
	Sumber            string    `gorm:"column:sumber;default:APLIKASI"`
}

func (TransaksiStok) TableName() string { return "transaksi_stok" }

// TransaksiView adalah baris buku besar beserta nama komoditas dan pelaku.
type TransaksiView struct {
	ID                string    `gorm:"column:id"`
	LotID             string    `gorm:"column:lot_id"`
	LotBaruID         *string   `gorm:"column:lot_baru_id"`
	Jenis             string    `gorm:"column:jenis"`
	VolumeTon         float64   `gorm:"column:volume_ton"`
	KomoditasKode     string    `gorm:"column:komoditas_kode"`
	KomoditasNama     string    `gorm:"column:komoditas_nama"`
	FasilitasAsalID   *string   `gorm:"column:fasilitas_asal_id"`
	FasilitasTujuanID *string   `gorm:"column:fasilitas_tujuan_id"`
	OlehNama          *string   `gorm:"column:oleh_nama"`
	Waktu             time.Time `gorm:"column:waktu"`
	Alasan            string    `gorm:"column:alasan"`
}

// TransaksiInput adalah satu transaksi stok yang sudah tervalidasi bentuknya.
// ID dan LotBaruID dibuat pemanggil agar repositori deterministik dan mudah diuji.
type TransaksiInput struct {
	ID     string
	Jenis  string
	Waktu  time.Time
	OlehID string
	Alasan string

	// MASUK: FasilitasID, KomoditasKode, Pemilik, LotID (lot baru), KedaluwarsaPada opsional.
	// KELUAR: LotID, VolumeTon atau Semua.
	// PINDAH: LotID, FasilitasTujuanID, VolumeTon atau Semua; LotBaruID dipakai pada pindah sebagian.
	LotID             string
	LotBaruID         string
	FasilitasID       string
	FasilitasTujuanID string
	KomoditasKode     string
	Pemilik           string
	KedaluwarsaPada   *time.Time
	VolumeTon         float64
	Semua             bool
}

// LotSnapshot adalah keadaan lot yang dicatat di audit trail.
type LotSnapshot struct {
	FasilitasID string  `json:"fasilitas_id"`
	VolumeTon   float64 `json:"volume_ton"`
	Status      string  `json:"status"`
}

// TransaksiHasil adalah akibat satu transaksi.
type TransaksiHasil struct {
	TransaksiID string
	Jenis       string
	LotID       string
	// LotBaruID terisi pada pindah sebagian.
	LotBaruID *string
	// VolumeTon adalah volume yang bergerak; VolumeSisaTon sisa pada lot sumber.
	VolumeTon     float64
	VolumeSisaTon float64
	StatusLot     string
	// Sebelum nil pada MASUK.
	Sebelum *LotSnapshot
	Sesudah LotSnapshot
}

// FasilitasPatch adalah perubahan parsial fasilitas; field nil tidak diubah.
type FasilitasPatch struct {
	KapasitasTon *int
	SuhuMinC     *float64
	SuhuMaksC    *float64
	Status       *string
}

// BarisBulk adalah hasil aksi massal untuk satu lot.
type BarisBulk struct {
	ID        string
	Berhasil  bool
	Pesan     string
	VolumeTon float64
	// TransaksiID terisi bila aksi menghasilkan transaksi stok.
	TransaksiID string
}

// OperasiRepository adalah akses data telemetri dan transaksi stok. Metode
// yang mengubah stok bersifat atomik dan mengunci baris yang terlibat.
type OperasiRepository interface {
	GetFasilitas(ctx context.Context, id string) (*FasilitasView, error)
	// UpdateFasilitas menerapkan patch dan mengembalikan keadaan sebelum dan sesudah.
	UpdateFasilitas(ctx context.Context, id string, patch FasilitasPatch) (before, after FasilitasView, err error)
	// SimpanTelemetri menyimpan pembacaan (duplikat fasilitas+waktu dilewati) dan
	// memperbarui suhu/kelembapan aktual fasilitas dari baris terbaru. Mengembalikan
	// jumlah baris yang benar-benar baru.
	SimpanTelemetri(ctx context.Context, rows []TelemetriSuhu) (int64, error)
	// ListTelemetri mengembalikan pembacaan sejak waktu `sejak`, terbaru lebih dulu.
	ListTelemetri(ctx context.Context, fasilitasID string, sejak time.Time, limit int) ([]TelemetriSuhu, error)
	// StokFasilitas mengembalikan lot tidak-habis pada fasilitas, kedaluwarsa terdekat lebih dulu.
	StokFasilitas(ctx context.Context, fasilitasID string) ([]StokView, error)
	// ListTransaksi mengembalikan transaksi yang menyentuh fasilitas, terbaru lebih dulu.
	ListTransaksi(ctx context.Context, fasilitasID string, limit int) ([]TransaksiView, error)
	CatatTransaksi(ctx context.Context, in TransaksiInput) (TransaksiHasil, error)
	// TandaiPrioritasJual menandai lot tidak-habis dan mengembalikan yang berhasil.
	TandaiPrioritasJual(ctx context.Context, ids []string) ([]BarisBulk, error)
}
