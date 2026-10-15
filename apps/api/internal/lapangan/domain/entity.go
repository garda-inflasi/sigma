package domain

import "time"

type TugasLapangan struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Judul      string    `gorm:"column:judul"`
	Lokasi     string    `gorm:"column:lokasi"`
	Tipe       string    `gorm:"column:tipe"`
	Status     string    `gorm:"column:status;default:BARU"`
	PIC        string    `gorm:"column:pic"`
	Tenggat    *time.Time `gorm:"column:tenggat"`
	DibuatPada time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (TugasLapangan) TableName() string {
	return "tugas_lapangan"
}
