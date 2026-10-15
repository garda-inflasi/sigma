package domain

import "time"

// Level represents the EWS alert level (hijau/kuning/merah).
type Level string

const (
	LevelHijau  Level = "hijau"
	LevelKuning Level = "kuning"
	LevelMerah  Level = "merah"
)

// EwsAlert maps to the ews_alert table.
type EwsAlert struct {
	ID                   int64      `gorm:"column:id;primaryKey;autoIncrement"`
	KomoditasKode        string     `gorm:"column:komoditas_kode;index:idx_ews_komoditas"`
	WilayahKode          string     `gorm:"column:wilayah_kode;index:idx_ews_wilayah"`
	Level                Level      `gorm:"column:level;type:level_ews"`
	Skor                 float64    `gorm:"column:skor"`
	Deskripsi            string     `gorm:"column:deskripsi"`
	Status               string     `gorm:"column:status;default:BARU"`
	Sumber               string     `gorm:"column:sumber"`
	SkorDampak           int        `gorm:"column:skor_dampak;default:0"`
	DiakuiOlehID         *string    `gorm:"column:diakui_oleh_id"`
	DiakuiPada           *time.Time `gorm:"column:diakui_pada"`
	DirekomendasikanPada *time.Time `gorm:"column:direkomendasikan_pada"`
	DitindaklanjutiPada  *time.Time `gorm:"column:ditindaklanjuti_pada"`
	DibuatPada           time.Time  `gorm:"column:dibuat_pada;default:now()"`
}

func (EwsAlert) TableName() string {
	return "ews_alert"
}

// BulkAction represents a bulk action on EWS alerts.
type BulkAction string

const (
	BulkAkui     BulkAction = "akui"
	BulkEskalasi BulkAction = "eskalasi"
	BulkSelesai  BulkAction = "selesai"
)
