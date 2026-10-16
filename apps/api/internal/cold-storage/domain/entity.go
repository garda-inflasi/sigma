package domain

import "time"

// RisikoTinggiAmbang adalah batas risiko food loss (pecahan 0–1). Lot dengan
// risiko di atas nilai ini dihitung "berisiko tinggi" pada ringkasan stok.
const RisikoTinggiAmbang = 0.6

// FasilitasColdStorage maps to the cold_storage table.
type FasilitasColdStorage struct {
	ID            string   `gorm:"column:id;primaryKey"`
	Nama          string   `gorm:"column:nama"`
	KabupatenKode string   `gorm:"column:kabupaten_kode"`
	Lat           float64  `gorm:"column:lat"`
	Lng           float64  `gorm:"column:lng"`
	KapasitasTon  int      `gorm:"column:kapasitas_ton"`
	SuhuTargetC   float64  `gorm:"column:suhu_target_c"`
	SuhuAktualC   *float64 `gorm:"column:suhu_aktual_c"`
	Status        string   `gorm:"column:status"`
}

func (FasilitasColdStorage) TableName() string {
	return "cold_storage"
}

// StokColdStorage maps to the stok_cold_storage table.
type StokColdStorage struct {
	ID              string    `gorm:"column:id;primaryKey"`
	ColdStorageID   string    `gorm:"column:cold_storage_id;index"`
	KomoditasKode   string    `gorm:"column:komoditas_kode"`
	VolumeTon       float64   `gorm:"column:volume_ton"`
	MasukPada       time.Time `gorm:"column:masuk_pada"`
	KedaluwarsaPada time.Time `gorm:"column:kedaluwarsa_pada"`
	Pemilik         string    `gorm:"column:pemilik"`
	RisikoFoodLoss  float64   `gorm:"column:risiko_food_loss"`
}

func (StokColdStorage) TableName() string {
	return "stok_cold_storage"
}

// FasilitasView adalah fasilitas beserta turunan yang dihitung saat dibaca:
// nama wilayah (join kabupaten) dan volume terisi (jumlah stok fasilitas).
type FasilitasView struct {
	ID            string   `gorm:"column:id"`
	Nama          string   `gorm:"column:nama"`
	KabupatenKode string   `gorm:"column:kabupaten_kode"`
	KabupatenNama string   `gorm:"column:kabupaten_nama"`
	Lat           float64  `gorm:"column:lat"`
	Lng           float64  `gorm:"column:lng"`
	KapasitasTon  int      `gorm:"column:kapasitas_ton"`
	TerisiTon     float64  `gorm:"column:terisi_ton"`
	SuhuTargetC   float64  `gorm:"column:suhu_target_c"`
	SuhuAktualC   *float64 `gorm:"column:suhu_aktual_c"`
	Status        string   `gorm:"column:status"`

	// Diisi usecase.
	UtilisasiPersen float64 `gorm:"-"`
}

// StokView adalah satu lot stok beserta nama fasilitas dan komoditasnya.
type StokView struct {
	ID              string    `gorm:"column:id"`
	ColdStorageID   string    `gorm:"column:cold_storage_id"`
	ColdStorageNama string    `gorm:"column:cold_storage_nama"`
	KomoditasKode   string    `gorm:"column:komoditas_kode"`
	KomoditasNama   string    `gorm:"column:komoditas_nama"`
	VolumeTon       float64   `gorm:"column:volume_ton"`
	MasukPada       time.Time `gorm:"column:masuk_pada"`
	KedaluwarsaPada time.Time `gorm:"column:kedaluwarsa_pada"`
	RisikoFoodLoss  float64   `gorm:"column:risiko_food_loss"`
	Pemilik         string    `gorm:"column:pemilik"`

	// Diisi usecase.
	SisaMasaSimpanHari int `gorm:"-"`
}

// StokSort adalah kolom pengurutan yang diizinkan untuk daftar stok.
type StokSort string

const (
	SortSisaMasaSimpan StokSort = "sisa_masa_simpan"
	SortKomoditas      StokSort = "komoditas"
	SortFasilitas      StokSort = "fasilitas"
	SortVolume         StokSort = "volume"
	SortMasuk          StokSort = "masuk"
	SortRisiko         StokSort = "risiko"
	SortPemilik        StokSort = "pemilik"
)

// StokQuery adalah kriteria daftar stok: filter, urutan, dan halaman.
type StokQuery struct {
	ColdStorageID string
	Search        string
	Sort          StokSort
	Desc          bool
	Page          int
	Limit         int
}

// Offset menghitung posisi baris pertama halaman.
func (q StokQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}

// StokPage adalah satu halaman stok beserta agregat seluruh hasil filter.
type StokPage struct {
	Items          []StokView
	Total          int64
	TotalVolumeTon float64
}

// RingkasanStok adalah agregat seluruh stok, tidak terpengaruh filter tabel.
type RingkasanStok struct {
	TotalLot        int64
	TotalVolumeTon  float64
	RisikoTinggiLot int64
	RisikoTinggiTon float64
}

// BulkAction represents a bulk action on cold storage.
type BulkAction string

const (
	BulkRencanakanDistribusi BulkAction = "rencanakan_distribusi"
	BulkRekomendasiOlahan    BulkAction = "rekomendasi_olahan"
)

// BulkResult adalah hasil aksi massal: lot yang ditemukan dan volumenya.
type BulkResult struct {
	Berhasil  int64
	VolumeTon float64
}

// AuditLog maps to the audit_log table (jejak aksi massal).
type AuditLog struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Waktu     time.Time `gorm:"column:waktu;default:now()"`
	AktorID   *string   `gorm:"column:aktor_id"`
	Aksi      string    `gorm:"column:aksi"`
	Entitas   string    `gorm:"column:entitas"`
	EntitasID string    `gorm:"column:entitas_id"`
	Hasil     string    `gorm:"column:hasil"`
	IP        *string   `gorm:"column:ip"`
	Metadata  *string   `gorm:"column:metadata;type:jsonb"`
}

func (AuditLog) TableName() string {
	return "audit_log"
}
