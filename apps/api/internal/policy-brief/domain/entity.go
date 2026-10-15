package domain

import "time"

type PolicyBrief struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Judul       string    `gorm:"column:judul"`
	Periode     string    `gorm:"column:periode"`
	Ringkasan   string    `gorm:"column:ringkasan;type:text"`
	Status      string    `gorm:"column:status;default:DRAFT"`
	DibuatPada  time.Time `gorm:"column:dibuat_pada;default:now()"`
	DiupdatePada time.Time `gorm:"column:diupdate_pada;default:now()"`
}

func (PolicyBrief) TableName() string {
	return "policy_brief"
}
