package domain

type Komoditas struct {
	Kode     string `gorm:"column:kode;primaryKey"`
	Nama     string `gorm:"column:nama"`
	Kategori string `gorm:"column:kategori"`
	Satuan   string `gorm:"column:satuan"`
	Pilot    bool   `gorm:"column:pilot;default:false"`
	Status   string `gorm:"column:status;default:AKTIF"`
}

func (Komoditas) TableName() string {
	return "komoditas"
}

// Kabupaten represents a regency/city in the master data.
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

// Pasar represents a market in the master data.
type Pasar struct {
	ID            string  `gorm:"column:id;primaryKey"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	Lat           float64 `gorm:"column:lat"`
	Lng           float64 `gorm:"column:lng"`
	RadiusMeter   int     `gorm:"column:radius_meter"`
	Aktif         bool    `gorm:"column:aktif"`
}

func (Pasar) TableName() string {
	return "pasar"
}

// BulkAction represents a bulk action on komoditas master data.
type BulkAction string

const (
	BulkAjukanPerubahan BulkAction = "ajukan_perubahan"
	BulkSetujui         BulkAction = "setujui"
	BulkArsipkan        BulkAction = "arsipkan"
)
