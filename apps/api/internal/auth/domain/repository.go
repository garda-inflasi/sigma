package domain

import (
	"context"
	"time"
)

type Repository interface {
	// Pengguna
	GetUserByID(ctx context.Context, id string) (*Pengguna, error)
	GetUserByEmail(ctx context.Context, email string) (*Pengguna, error)
	CreateUser(ctx context.Context, user *Pengguna) error
	UpdateUser(ctx context.Context, user *Pengguna) error
	ListUsers(ctx context.Context, page, limit int) ([]Pengguna, int64, error)

	// Refresh Token
	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id int64) error
	RevokeRefreshTokenFamily(ctx context.Context, familyID string) error

	// Login Attempt
	CreateLoginAttempt(ctx context.Context, attempt *LoginAttempt) error
	CountRecentFailedAttempts(ctx context.Context, userID string, since time.Time) (int64, error)

	// Audit Log
	CreateAuditLog(ctx context.Context, log *AuditLog) error
}
