package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Siklus ekonomi (enum siklus).
const (
	SiklusNormal  = "NORMAL"
	SiklusTekanan = "TEKANAN"
	SiklusSurplus = "SURPLUS"
)

// WilayahBawaan adalah kode provinsi Aceh: siklus dominan yang mewarnai seluruh antarmuka.
const WilayahBawaan = "11"

// SiklusEkonomi memetakan tabel siklus_ekonomi. PK-nya ID BIGINT identity; Kode memuat ID lama
// (mis. SE-11-DEMO) dan menjadi kunci idempotensi seeder demo. wilayah_kode -> provinsi (RESTRICT).
type SiklusEkonomi struct {
	ID          idx.ID     `gorm:"column:id;primaryKey;autoIncrement"`
	Kode        *string    `gorm:"column:kode"`
	WilayahKode string     `gorm:"column:wilayah_kode"`
	Siklus      string     `gorm:"column:siklus"`
	MulaiPada   time.Time  `gorm:"column:mulai_pada"`
	AkhirPada   *time.Time `gorm:"column:akhir_pada"`
	DibuatPada  time.Time  `gorm:"column:dibuat_pada;default:now()"`
}

func (SiklusEkonomi) TableName() string {
	return "siklus_ekonomi"
}
