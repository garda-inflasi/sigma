package domain

import "time"

// ── Entity ────────────────────────────────────────────────────────────

// Pengguna maps to the pengguna table.
type Pengguna struct {
	ID            string     `gorm:"column:id;primaryKey"`
	Nama          string     `gorm:"column:nama"`
	Email         string     `gorm:"column:email;uniqueIndex"`
	PasswordHash  *string    `gorm:"column:password_hash"`
	Peran         string     `gorm:"column:peran;type:peran"`
	Lapis         string     `gorm:"column:lapis;type:lapis"`
	Lembaga       string     `gorm:"column:lembaga"`
	KabupatenKode *string    `gorm:"column:kabupaten_kode"`
	MfaAktif      bool       `gorm:"column:mfa_aktif;default:false"`
	Status        string     `gorm:"column:status;type:status_akun;default:AKTIF"`
	TerakhirMasuk *time.Time `gorm:"column:terakhir_masuk"`
	DibuatPada    time.Time  `gorm:"column:dibuat_pada;default:now()"`
}

func (Pengguna) TableName() string {
	return "pengguna"
}

// RefreshToken maps to the refresh_tokens table.
type RefreshToken struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    string     `gorm:"column:user_id;index:idx_rt_user"`
	TokenHash string     `gorm:"column:token_hash;uniqueIndex"`
	FamilyID  string     `gorm:"column:family_id;type:uuid;index:idx_rt_family"`
	Device    *string    `gorm:"column:device"`
	IP        *string    `gorm:"column:ip"`
	ExpiresAt time.Time  `gorm:"column:expires_at"`
	RevokedAt *time.Time `gorm:"column:revoked_at"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// LoginAttempt maps to the login_attempts table.
type LoginAttempt struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID        *string    `gorm:"column:user_id;index:idx_la_user_time"`
	IP            *string    `gorm:"column:ip;index:idx_la_ip_time"`
	Success       bool       `gorm:"column:success"`
	FailureReason *string    `gorm:"column:failure_reason"`
	AttemptedAt   time.Time  `gorm:"column:attempted_at;default:now()"`
}

func (LoginAttempt) TableName() string {
	return "login_attempts"
}

// AuditLog maps to the audit_log table.
type AuditLog struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Waktu     time.Time `gorm:"column:waktu;default:now();index:idx_audit_log_waktu"`
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

