package domain

import "time"

// EntriHarga maps to the entri_harga table.
type EntriHarga struct {
	ID            string    `gorm:"column:id;primaryKey"`
	KomoditasKode string    `gorm:"column:komoditas_kode;index:idx_eh_komoditas"`
	WilayahKode   string    `gorm:"column:wilayah_kode;index:idx_eh_wilayah"`
	Harga         float64   `gorm:"column:harga;type:numeric(18,2)"`
	Satuan        string    `gorm:"column:satuan"`
	Sumber        string    `gorm:"column:sumber"`
	Periode       string    `gorm:"column:periode;index:idx_eh_periode"`
	DibuatPada    time.Time `gorm:"column:dibuat_pada;default:now()"`
	DiupdatePada  time.Time `gorm:"column:diupdate_pada;default:now()"`
}

func (EntriHarga) TableName() string {
	return "entri_harga"
}
