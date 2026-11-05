package domain

// Provinsi memetakan tabel provinsi.
type Provinsi struct {
	Kode string `gorm:"column:kode;primaryKey"`
	Nama string `gorm:"column:nama"`
}

func (Provinsi) TableName() string {
	return "provinsi"
}

// Komoditas memetakan tabel komoditas.
type Komoditas struct {
	Kode          string `gorm:"column:kode;primaryKey"`
	Nama          string `gorm:"column:nama"`
	Kategori      string `gorm:"column:kategori"`
	Satuan        string `gorm:"column:satuan"`
	HargaAcuan    int    `gorm:"column:harga_acuan"`
	Pilot         bool   `gorm:"column:pilot;default:false"`
	ShelfLifeHari int    `gorm:"column:shelf_life_hari"`
	Versi         int    `gorm:"column:versi;default:1"`
	Status        string `gorm:"column:status;default:AKTIF"`
}

func (Komoditas) TableName() string {
	return "komoditas"
}

// Kabupaten adalah kabupaten/kota pada master wilayah (kode BPS).
type Kabupaten struct {
	Kode         string  `gorm:"column:kode;primaryKey"`
	Nama         string  `gorm:"column:nama"`
	Jenis        string  `gorm:"column:jenis"`
	Ibukota      string  `gorm:"column:ibukota"`
	Lat          float64 `gorm:"column:lat"`
	Lng          float64 `gorm:"column:lng"`
	ProvinsiKode string  `gorm:"column:provinsi_kode"`
}

func (Kabupaten) TableName() string {
	return "kabupaten"
}

// Pasar adalah pasar pada master data, dengan geofence untuk entri lapangan.
type Pasar struct {
	ID            string  `gorm:"column:id;primaryKey"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	Lat           float64 `gorm:"column:lat"`
	Lng           float64 `gorm:"column:lng"`
	RadiusMeter   int     `gorm:"column:radius_meter;default:400"`
	Aktif         bool    `gorm:"column:aktif;default:true"`
}

func (Pasar) TableName() string {
	return "pasar"
}

// BulkAction adalah aksi massal pada master komoditas.
type BulkAction string

const (
	BulkAjukanPerubahan BulkAction = "ajukan_perubahan"
	BulkSetujui         BulkAction = "setujui"
	BulkArsipkan        BulkAction = "arsipkan"
)

// Status master komoditas (enum status_master).
const (
	StatusAktif               = "AKTIF"
	StatusMenungguPersetujuan = "MENUNGGU_PERSETUJUAN"
	StatusDiarsipkan          = "DIARSIPKAN"
)
