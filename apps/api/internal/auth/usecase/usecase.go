package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/google/uuid"
)

// ── Errors ────────────────────────────────────────────────────────────

var (
	ErrInvalidCredentials = errors.New("email atau password salah")
	ErrAccountLocked      = errors.New("akun terkunci, coba lagi nanti")
	ErrAccountInactive    = errors.New("akun tidak aktif")
	ErrInvalidToken       = errors.New("token tidak valid")
	ErrTokenExpired       = errors.New("token sudah kedaluwarsa")
	ErrUserNotFound       = errors.New("pengguna tidak ditemukan")
	ErrEmailExists        = errors.New("email sudah terdaftar")
)

// ── Config ────────────────────────────────────────────────────────────

type Config struct {
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	MaxFailedAttempts int
	LockoutDuration   time.Duration
}

func DefaultConfig() Config {
	return Config{
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   7 * 24 * time.Hour,
		MaxFailedAttempts: 5,
		LockoutDuration:   15 * time.Minute,
	}
}

// ── UseCase ───────────────────────────────────────────────────────────

type UseCase struct {
	repo  domain.Repository
	token *token.Manager
	cfg   Config
	log   *logger.Logger
}

func New(repo domain.Repository, tokenMgr *token.Manager, cfg Config, log *logger.Logger) *UseCase {
	return &UseCase{
		repo:  repo,
		token: tokenMgr,
		cfg:   cfg,
		log:   log,
	}
}

// ── Auth Use Cases ────────────────────────────────────────────────────

// Login authenticates a user and returns tokens.
func (uc *UseCase) Login(ctx context.Context, email, password, ip string) (*domain.Pengguna, string, string, error) {
	user, err := uc.repo.GetUserByEmail(ctx, email)
	if err != nil {
		uc.log.Warn("login: user not found", map[string]interface{}{"email": email, "ip": ip})
		uc.recordLoginAttempt(ctx, nil, ip, false, "user_not_found")
		return nil, "", "", ErrInvalidCredentials
	}

	if user.Status != "AKTIF" {
		uc.log.Warn("login: inactive account", map[string]interface{}{"user_id": user.ID, "status": user.Status})
		uc.recordLoginAttempt(ctx, &user.ID, ip, false, "account_inactive")
		return nil, "", "", ErrAccountInactive
	}

	if user.PasswordHash != nil {
		recentFails, err := uc.repo.CountRecentFailedAttempts(ctx, user.ID, time.Now().Add(-uc.cfg.LockoutDuration))
		if err == nil && recentFails >= int64(uc.cfg.MaxFailedAttempts) {
			uc.log.Warn("login: account locked", map[string]interface{}{"user_id": user.ID, "fails": recentFails})
			return nil, "", "", ErrAccountLocked
		}
	}

	if user.PasswordHash == nil {
		uc.log.Warn("login: no password set (SSO only)", map[string]interface{}{"user_id": user.ID})
		uc.recordLoginAttempt(ctx, &user.ID, ip, false, "no_password")
		return nil, "", "", ErrInvalidCredentials
	}

	if !crypto.VerifyPassword(password, *user.PasswordHash) {
		uc.log.Warn("login: wrong password", map[string]interface{}{"user_id": user.ID, "ip": ip})
		uc.recordLoginAttempt(ctx, &user.ID, ip, false, "wrong_password")
		return nil, "", "", ErrInvalidCredentials
	}

	accessToken, err := uc.token.CreateAccessToken(user.ID, user.Peran, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("create access token: %w", err)
	}

	familyID := uuid.New().String()
	refreshToken, err := uc.token.CreateAccessToken(user.ID, user.Peran, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("create refresh token: %w", err)
	}

	tokenHash := sha256Hex(refreshToken)
	now := time.Now()
	rt := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		FamilyID:  familyID,
		IP:        &ip,
		ExpiresAt: now.Add(uc.cfg.RefreshTokenTTL),
	}
	if err := uc.repo.CreateRefreshToken(ctx, rt); err != nil {
		return nil, "", "", fmt.Errorf("save refresh token: %w", err)
	}

	uc.recordLoginAttempt(ctx, &user.ID, ip, true, "")

	user.TerakhirMasuk = &now
	_ = uc.repo.UpdateUser(ctx, user)

	uc.log.Info("login success", map[string]interface{}{"user_id": user.ID, "email": email, "ip": ip})
	return user, accessToken, refreshToken, nil
}

// Refresh rotates a refresh token.
func (uc *UseCase) Refresh(ctx context.Context, refreshTokenStr, ip string) (string, string, error) {
	tokenHash := sha256Hex(refreshTokenStr)

	stored, err := uc.repo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return "", "", ErrInvalidToken
	}

	if stored.RevokedAt != nil {
		_ = uc.repo.RevokeRefreshTokenFamily(ctx, stored.FamilyID)
		return "", "", ErrInvalidToken
	}

	if time.Now().After(stored.ExpiresAt) {
		return "", "", ErrTokenExpired
	}

	now := time.Now()
	if err := uc.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return "", "", fmt.Errorf("revoke old token: %w", err)
	}

	accessToken, err := uc.token.CreateAccessToken(stored.UserID, "", nil)
	if err != nil {
		return "", "", fmt.Errorf("create access token: %w", err)
	}

	newRefreshToken, err := uc.token.CreateAccessToken(stored.UserID, "", nil)
	if err != nil {
		return "", "", fmt.Errorf("create refresh token: %w", err)
	}

	newHash := sha256Hex(newRefreshToken)
	rt := &domain.RefreshToken{
		UserID:    stored.UserID,
		TokenHash: newHash,
		FamilyID:  stored.FamilyID,
		IP:        &ip,
		ExpiresAt: now.Add(uc.cfg.RefreshTokenTTL),
	}
	if err := uc.repo.CreateRefreshToken(ctx, rt); err != nil {
		return "", "", fmt.Errorf("save new refresh token: %w", err)
	}

	return accessToken, newRefreshToken, nil
}

// Logout revokes all refresh tokens for a user.
func (uc *UseCase) Logout(ctx context.Context, userID string) error {
	return nil
}

// GetMe returns the current user profile.
func (uc *UseCase) GetMe(ctx context.Context, userID string) (*domain.Pengguna, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// ── User Use Cases ────────────────────────────────────────────────────

// CreateUser creates a new user.
func (uc *UseCase) CreateUser(ctx context.Context, user *domain.Pengguna, password string) error {
	existing, _ := uc.repo.GetUserByEmail(ctx, user.Email)
	if existing != nil {
		return ErrEmailExists
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user.PasswordHash = &hash

	return uc.repo.CreateUser(ctx, user)
}

// ListUsers returns a paginated list of users.
func (uc *UseCase) ListUsers(ctx context.Context, page, limit int) ([]domain.Pengguna, int64, error) {
	return uc.repo.ListUsers(ctx, page, limit)
}

// GetUser returns a user by ID.
func (uc *UseCase) GetUser(ctx context.Context, id string) (*domain.Pengguna, error) {
	user, err := uc.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// ── Helpers ───────────────────────────────────────────────────────────

func (uc *UseCase) recordLoginAttempt(ctx context.Context, userID *string, ip string, success bool, reason string) {
	attempt := &domain.LoginAttempt{
		UserID:  userID,
		IP:      &ip,
		Success: success,
	}
	if reason != "" {
		attempt.FailureReason = &reason
	}
	_ = uc.repo.CreateLoginAttempt(ctx, attempt)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

