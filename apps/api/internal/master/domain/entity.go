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
