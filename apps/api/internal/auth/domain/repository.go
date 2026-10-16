package domain

import (
	"context"
	"time"
)

type Repository interface {
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
	CountRecentFailedAttempts(ctx context.Context, userID string, since time.Time) (int64, error)
	SaveResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetResetToken(ctx context.Context, tokenHash string) (*ResetToken, error)
	MarkResetTokenUsed(ctx context.Context, id int64, usedAt time.Time) error
	CreateAuditLog(ctx context.Context, log *AuditLog) error
	BulkUpdateUsers(ctx context.Context, ids []string, action BulkUserAction) (int64, error)
}
