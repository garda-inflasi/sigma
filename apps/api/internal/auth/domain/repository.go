package domain

import (
	"context"
	"time"
)

type Repository interface {
	// KunciRepository menyimpan lockout login: satu-satunya mekanisme penghitung percobaan gagal.
	KunciRepository
	GetUserByID(ctx context.Context, id string) (*Pengguna, error)
	GetUserByEmail(ctx context.Context, email string) (*Pengguna, error)
	CreateUser(ctx context.Context, user *Pengguna) error
	UpdateUser(ctx context.Context, user *Pengguna) error
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context, page, limit int, search string) ([]Pengguna, int64, error)
	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id int64) error
	RevokeRefreshTokenFamily(ctx context.Context, familyID string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
	BlacklistJWT(ctx context.Context, jti string, ttl time.Duration) error
	IsJWTBlacklisted(ctx context.Context, jti string) (bool, error)
	CreateLoginAttempt(ctx context.Context, attempt *LoginAttempt) error
	SaveResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetResetToken(ctx context.Context, tokenHash string) (*ResetToken, error)
	// ConsumeResetToken memakai token satu kali secara atomik: berhasil hanya bila token belum dipakai dan
	// belum kedaluwarsa pada now, lalu menghanguskan token lain milik pengguna yang sama. Mengembalikan
	// false bila token sudah dipakai (mis. oleh permintaan serentak) atau kedaluwarsa.
	ConsumeResetToken(ctx context.Context, id int64, now time.Time) (bool, error)
	BulkUpdateUsers(ctx context.Context, ids []string, action BulkUserAction) (int64, error)
	// ActivateMFA mengaktifkan MFA dan mengganti seluruh kode cadangan (hash) dalam satu transaksi.
	ActivateMFA(ctx context.Context, userID string, kodeHash []string, langkah int64) error
	// ReplaceBackupCodes mengganti seluruh kode cadangan (hash) milik pengguna.
	ReplaceBackupCodes(ctx context.Context, userID string, kodeHash []string) error
	// UseBackupCode menandai satu kode cadangan terpakai; true hanya bila kode ada dan belum terpakai.
	UseBackupCode(ctx context.Context, userID, kodeHash string, now time.Time) (bool, error)
	// CountBackupCodes menghitung kode cadangan yang belum terpakai.
	CountBackupCodes(ctx context.Context, userID string) (int64, error)
	// AdvanceMFAStep memajukan langkah TOTP terakhir; false bila langkah itu sudah pernah dipakai.
	AdvanceMFAStep(ctx context.Context, userID string, langkah int64) (bool, error)
	// ClearMFA menonaktifkan MFA: rahasia, langkah, dan semua kode cadangan dihapus.
	ClearMFA(ctx context.Context, userID string) error
}
