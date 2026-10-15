package domain

import "time"

type SumberData struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Nama            string    `gorm:"column:nama"`
	Tipe            string    `gorm:"column:tipe"`
	URL             string    `gorm:"column:url"`
	Status          string    `gorm:"column:status;default:AKTIF"`
	FrekuensiUpdate string    `gorm:"column:frekuensi_update"`
	DibuatPada      time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (SumberData) TableName() string {
	return "sumber_data"
}

// BulkAction represents a bulk action on sumber data pipelines.
type BulkAction string

const (
	BulkJalankan  BulkAction = "jalankan"
	BulkJeda      BulkAction = "jeda"
	BulkLanjutkan BulkAction = "lanjutkan"
)
