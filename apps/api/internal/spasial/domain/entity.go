package domain

type Wilayah struct {
	Kode  string  `gorm:"column:kode;primaryKey"`
	Nama  string  `gorm:"column:nama"`
	Lat   float64 `gorm:"column:lat"`
	Lng   float64 `gorm:"column:lng"`
	Level string  `gorm:"column:level"`
}

func (Wilayah) TableName() string {
	return "wilayah"
}
