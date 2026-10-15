package domain

import "time"

// Pengguna maps to the pengguna table.
type Pengguna struct {
	ID             string     `gorm:"column:id;primaryKey"`
	Nama           string     `gorm:"column:nama"`
	Email          string     `gorm:"column:email;uniqueIndex:idx_pengguna_email"`
	PasswordHash   *string    `gorm:"column:password_hash"`
	Peran          string     `gorm:"column:peran"`
	Lapis          string     `gorm:"column:lapis"`
	Lembaga        string     `gorm:"column:lembaga"`
	KabupatenKode  *string    `gorm:"column:kabupaten_kode"`
	MfaAktif       bool       `gorm:"column:mfa_aktif;default:false"`
	MfaSecret      *string    `gorm:"column:mfa_secret"`
	MfaBackupCodes *string    `gorm:"column:mfa_backup_codes"`
	Status         string     `gorm:"column:status;default:AKTIF"`
	TerakhirMasuk  *time.Time `gorm:"column:terakhir_masuk"`
	DibuatPada     time.Time  `gorm:"column:dibuat_pada;default:now()"`
	DiupdatePada   time.Time  `gorm:"column:diupdate_pada;default:now()"`
}

func (Pengguna) TableName() string {
	return "pengguna"
}

// RefreshToken represents a stored refresh token.
type RefreshToken struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    string     `gorm:"column:user_id;index:idx_rt_user"`
	TokenHash string     `gorm:"column:token_hash;uniqueIndex:idx_rt_hash"`
	FamilyID  string     `gorm:"column:family_id;index:idx_rt_family"`
	IP        *string    `gorm:"column:ip"`
	ExpiresAt time.Time  `gorm:"column:expires_at"`
	RevokedAt *time.Time `gorm:"column:revoked_at"`
	CreatedAt time.Time  `gorm:"column:created_at;default:now()"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// LoginAttempt represents a login attempt record.
type LoginAttempt struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID        *string   `gorm:"column:user_id;index:idx_la_user"`
	IP            *string   `gorm:"column:ip"`
	Success       bool      `gorm:"column:success"`
	FailureReason *string   `gorm:"column:failure_reason"`
	AttemptedAt   time.Time `gorm:"column:attempted_at;default:now()"`
}

func (LoginAttempt) TableName() string {
	return "login_attempts"
}

// ResetToken represents a password reset token.
type ResetToken struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    string     `gorm:"column:user_id"`
	TokenHash string     `gorm:"column:token_hash;uniqueIndex:idx_rt_hash"`
	ExpiresAt time.Time  `gorm:"column:expires_at"`
	UsedAt    *time.Time `gorm:"column:used_at"`
}

func (ResetToken) TableName() string {
	return "password_reset_tokens"
}

// AuditLog represents an audit log entry.
type AuditLog struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Waktu     time.Time `gorm:"column:waktu;default:now()"`
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

// BulkUserAction represents a bulk action on users.
type BulkUserAction string

const (
	BulkAktifkan    BulkUserAction = "aktifkan"
	BulkNonaktifkan BulkUserAction = "nonaktifkan"
	BulkWajibkanMFA BulkUserAction = "wajibkan_mfa"
	BulkAkhiriSesi  BulkUserAction = "akhiri_sesi"
)
