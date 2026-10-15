package domain

import "time"

// ProyeksiXai maps to the proyeksi_xai table.
type ProyeksiXai struct {
	ID               int64     `gorm:"column:id;primaryKey;autoIncrement"`
	KomoditasKode    string    `gorm:"column:komoditas_kode;index"`
	ProyeksiHarga    float64   `gorm:"column:proyeksi_harga"`
	IntervalAtas     float64   `gorm:"column:interval_atas"`
	IntervalBawah    float64   `gorm:"column:interval_bawah"`
	FaktorDominan    []string  `gorm:"column:faktor_dominan;type:jsonb"`
	TingkatKepercayaan float64 `gorm:"column:tingkat_kepercayaan"`
	Periode          string    `gorm:"column:periode"`
	DibuatPada       time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (ProyeksiXai) TableName() string {
	return "proyeksi_xai"
}
