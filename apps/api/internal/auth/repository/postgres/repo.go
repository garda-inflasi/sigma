package postgres

import (
	"context"
	"fmt"
	"strings"
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

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&domain.Pengguna{}).Error
}

func (r *Repository) ListUsers(ctx context.Context, page, limit int, search string) ([]domain.Pengguna, int64, error) {
	var users []domain.Pengguna
	var total int64

	offset := (page - 1) * limit

	query := r.db.WithContext(ctx).Model(&domain.Pengguna{})
	if search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		// peran bertipe enum PostgreSQL, jadi harus di-cast ke text sebelum LOWER().
		query = query.Where(
			"LOWER(nama) LIKE ? OR LOWER(email) LIKE ? OR LOWER(peran::text) LIKE ? OR LOWER(lembaga) LIKE ?",
			pattern, pattern, pattern, pattern,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	if err := query.Offset(offset).Limit(limit).Order("dibuat_pada DESC").Find(&users).Error; err != nil {
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

// ── JWT Blacklist ──────────────────────────────────────────────────────

func (r *Repository) BlacklistJWT(ctx context.Context, jti string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO jwt_blacklist (jti, expires_at) VALUES (?, ?) ON CONFLICT (jti) DO NOTHING",
		jti, expiresAt,
	).Error
}

func (r *Repository) IsJWTBlacklisted(ctx context.Context, jti string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&struct{}{}).
		Table("jwt_blacklist").
		Where("jti = ? AND expires_at > now()", jti).
		Count(&count).Error
	return count > 0, err
}

// ── Password Reset Token ──────────────────────────────────────────────

func (r *Repository) SaveResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)",
		userID, tokenHash, expiresAt,
	).Error
}

func (r *Repository) GetResetToken(ctx context.Context, tokenHash string) (*domain.ResetToken, error) {
	var rt domain.ResetToken
	if err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&rt).Error; err != nil {
		return nil, fmt.Errorf("get reset token: %w", err)
	}
	return &rt, nil
}

func (r *Repository) MarkResetTokenUsed(ctx context.Context, id int64, usedAt time.Time) error {
	return r.db.WithContext(ctx).Exec(
		"UPDATE password_reset_tokens SET used_at = ? WHERE id = ?",
		usedAt, id,
	).Error
}

// ── Revoke All User Refresh Tokens ────────────────────────────────────

func (r *Repository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

// ── Audit Log ─────────────────────────────────────────────────────────

func (r *Repository) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ── Bulk User Actions ─────────────────────────────────────────────────

func (r *Repository) BulkUpdateUsers(ctx context.Context, ids []string, action domain.BulkUserAction) (int64, error) {
	tx := r.db.WithContext(ctx).Model(&domain.Pengguna{}).Where("id IN ?", ids)

	switch action {
	case domain.BulkAktifkan:
		tx = tx.Update("status", "AKTIF")
	case domain.BulkNonaktifkan:
		tx = tx.Update("status", "NONAKTIF")
	case domain.BulkWajibkanMFA:
		tx = tx.Update("mfa_wajib", true)
	case domain.BulkAkhiriSesi:
		// Revoke all refresh tokens for these users
		now := time.Now()
		if err := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
			Where("user_id IN ? AND revoked_at IS NULL", ids).
			Update("revoked_at", now).Error; err != nil {
			return 0, fmt.Errorf("revoke sessions: %w", err)
		}
		return int64(len(ids)), nil
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}

	if tx.Error != nil {
		return 0, fmt.Errorf("bulk action %s: %w", action, tx.Error)
	}

	return tx.RowsAffected, nil
}
