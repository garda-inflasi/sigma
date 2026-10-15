package domain

import "time"

// TahapEnum represents the stage of an intervention card.
type TahapEnum string

const (
	TahapUsulan  TahapEnum = "usulan"
	TahapKaji    TahapEnum = "kaji"
	TahapSetuju  TahapEnum = "setuju"
	TahapLaksana TahapEnum = "laksana"
	TahapSelesai TahapEnum = "selesai"
	TahapBatal   TahapEnum = "batal"
)

// KartuIntervensi maps to the kartu_intervensi table.
type KartuIntervensi struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Judul         string    `gorm:"column:judul"`
	Deskripsi     string    `gorm:"column:deskripsi"`
	Tahap         TahapEnum `gorm:"column:tahap;type:tahap_intervensi;default:usulan"`
	Urutan        int       `gorm:"column:urutan;default:0"`
	Prioritas     string    `gorm:"column:prioritas"`
	PIC           string    `gorm:"column:pic"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	WilayahKode   string    `gorm:"column:wilayah_kode"`
	DibuatOlehID  string    `gorm:"column:dibuat_oleh_id"`
	DibuatPada    time.Time `gorm:"column:dibuat_pada;default:now()"`
	DiupdatePada  time.Time `gorm:"column:diupdate_pada;default:now()"`
}

func (KartuIntervensi) TableName() string {
	return "kartu_intervensi"
}

// BulkAction for intervensi.
type BulkAction string

const (
	BulkArsip BulkAction = "arsip"
	BulkHapus BulkAction = "hapus"
)
