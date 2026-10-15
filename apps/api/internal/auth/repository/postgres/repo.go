package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ── Pengguna ──────────────────────────────────────────────────────────

func (r *Repository) GetUserByID(ctx context.Context, id string) (*domain.Pengguna, error) {
	var user domain.Pengguna
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*domain.Pengguna, error) {
	var user domain.Pengguna
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *domain.Pengguna) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *Repository) UpdateUser(ctx context.Context, user *domain.Pengguna) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *Repository) ListUsers(ctx context.Context, page, limit int) ([]domain.Pengguna, int64, error) {
	var users []domain.Pengguna
	var total int64

	offset := (page - 1) * limit

	if err := r.db.WithContext(ctx).Model(&domain.Pengguna{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	if err := r.db.WithContext(ctx).Offset(offset).Limit(limit).Order("dibuat_pada DESC").Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	return users, total, nil
}

// ── Refresh Token ─────────────────────────────────────────────────────

func (r *Repository) CreateRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *Repository) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).First(&token).Error; err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &token, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("id = ?", id).Update("revoked_at", now).Error
}

func (r *Repository) RevokeRefreshTokenFamily(ctx context.Context, familyID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("family_id = ?", familyID).Update("revoked_at", now).Error
}

// ── Login Attempt ─────────────────────────────────────────────────────

func (r *Repository) CreateLoginAttempt(ctx context.Context, attempt *domain.LoginAttempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *Repository) CountRecentFailedAttempts(ctx context.Context, userID string, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.LoginAttempt{}).
		Where("user_id = ? AND success = ? AND attempted_at >= ?", userID, false, since).
		Count(&count).Error
	return count, err
}

// ── Audit Log ─────────────────────────────────────────────────────────

func (r *Repository) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

