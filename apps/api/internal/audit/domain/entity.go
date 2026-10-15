package domain

import "time"

type AuditLog struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Waktu     time.Time `gorm:"column:waktu;default:now();index:idx_audit_waktu"`
	AktorID   *string   `gorm:"column:aktor_id"`
	Aksi      string    `gorm:"column:aksi"`
	Entitas   string    `gorm:"column:entitas"`
	EntitasID string    `gorm:"column:entitas_id"`
	Hasil     string    `gorm:"column:hasil"`
	IP        *string   `gorm:"column:ip"`
	Metadata  *string   `gorm:"column:metadata;type:jsonb"`
}

func (AuditLog) TableName() string {
	return "audit_log"
}
