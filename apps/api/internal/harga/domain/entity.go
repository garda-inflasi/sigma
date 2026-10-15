package domain

import "time"

// EntriHarga maps to the entri_harga table.
type EntriHarga struct {
	ID              string     `gorm:"column:id;primaryKey"`
	KomoditasKode   string     `gorm:"column:komoditas_kode;index:idx_eh_komoditas"`
	PasarID         string     `gorm:"column:pasar_id;index:idx_eh_pasar"`
	Harga           float64    `gorm:"column:harga;type:numeric(18,2)"`
	Satuan          string     `gorm:"column:satuan"`
	Sumber          string     `gorm:"column:sumber"`
	Periode         string     `gorm:"column:periode;index:idx_eh_periode"`
	Kualitas        string     `gorm:"column:kualitas;default:MEDIUM"`
	Status          string     `gorm:"column:status;default:DIAJUKAN"`
	DeviasiPct      float64    `gorm:"column:deviasi_pct;default:0"`
	SkorAnomali     float64    `gorm:"column:skor_anomali;default:0"`
	Lat             *float64   `gorm:"column:lat"`
	Lng             *float64   `gorm:"column:lng"`
	DalamGeofence   bool       `gorm:"column:dalam_geofence;default:true"`
	FotoURL         *string    `gorm:"column:foto_url"`
	Catatan         *string    `gorm:"column:catatan"`
	PetugasID       *string    `gorm:"column:petugas_id"`
	DivalidasiOleh  *string    `gorm:"column:divalidasi_oleh"`
	DivalidasiPada  *time.Time `gorm:"column:divalidasi_pada"`
	DipublikasiOleh *string    `gorm:"column:dipublikasi_oleh"`
	DipublikasiPada *time.Time `gorm:"column:dipublikasi_pada"`
	DibuatPada      time.Time  `gorm:"column:dibuat_pada;default:now()"`
	DiupdatePada    time.Time  `gorm:"column:diupdate_pada;default:now()"`
}

func (EntriHarga) TableName() string {
	return "entri_harga"
}

// BulkAction represents a bulk action on harga entries.
type BulkAction string

const (
	BulkValidasi  BulkAction = "validasi"
	BulkPublikasi BulkAction = "publikasi"
	BulkTolak     BulkAction = "tolak"
)
