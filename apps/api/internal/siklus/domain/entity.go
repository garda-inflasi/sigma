package domain

import "time"

// Siklus ekonomi (enum siklus).
const (
	SiklusNormal  = "NORMAL"
	SiklusTekanan = "TEKANAN"
	SiklusSurplus = "SURPLUS"
)

// WilayahBawaan adalah kode provinsi Aceh: siklus dominan yang mewarnai seluruh antarmuka.
const WilayahBawaan = "11"

// SiklusEkonomi memetakan tabel siklus_ekonomi.
type SiklusEkonomi struct {
	ID          string     `gorm:"column:id;primaryKey"`
	WilayahKode string     `gorm:"column:wilayah_kode"`
	Siklus      string     `gorm:"column:siklus"`
	MulaiPada   time.Time  `gorm:"column:mulai_pada"`
	AkhirPada   *time.Time `gorm:"column:akhir_pada"`
	DibuatPada  time.Time  `gorm:"column:dibuat_pada;default:now()"`
}

func (SiklusEkonomi) TableName() string {
	return "siklus_ekonomi"
}
