package domain

import "time"

// SiklusEkonomi maps to the siklus_ekonomi table.
type SiklusEkonomi struct {
	ID          string     `gorm:"column:id;primaryKey"`
	WilayahKode string     `gorm:"column:wilayah_kode;index:idx_se_wilayah"`
	Siklus      string     `gorm:"column:siklus;type:siklus_ekonomi"`
	MulaiPada   time.Time  `gorm:"column:mulai_pada"`
	AkhirPada   *time.Time `gorm:"column:akhir_pada"`
	DibuatPada  time.Time  `gorm:"column:dibuat_pada;default:now()"`
}

func (SiklusEkonomi) TableName() string {
	return "siklus_ekonomi"
}
