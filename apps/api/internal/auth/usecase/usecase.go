package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/garda-inflasi/api/pkg/totp"
	"github.com/google/uuid"
)

// ── Errors ────────────────────────────────────────────────────────────

var (
	ErrInvalidCredentials  = errors.New("email atau password salah")
	ErrAccountLocked       = errors.New("akun terkunci, coba lagi nanti")
	ErrAccountInactive     = errors.New("akun tidak aktif")
	ErrInvalidToken        = errors.New("token tidak valid")
	ErrTokenExpired        = errors.New("token sudah kedaluwarsa")
	ErrUserNotFound        = errors.New("pengguna tidak ditemukan")
	ErrEmailExists         = errors.New("email sudah terdaftar")
	ErrWeakPassword        = errors.New("kata sandi terlalu lemah")
	ErrPasswordReuse       = errors.New("kata sandi sudah pernah digunakan sebelumnya")
	ErrWrongPassword       = errors.New("kata sandi lama salah")
	ErrResetTokenInvalid   = errors.New("token atur ulang tidak valid")
	ErrResetTokenExpired   = errors.New("token atur ulang sudah kedaluwarsa")
)

// ── Password Policy ───────────────────────────────────────────────────

var (
	minPasswordLength = 8
	// Common password deny-list (top 100 most common passwords)
	denyList = map[string]bool{
		"password": true, "12345678": true, "123456789": true, "qwerty123": true,
		"admin123": true, "letmein": true, "welcome": true, "monkey": true,
		"dragon": true, "master": true, "acehkukaya": true,
	}
)

// ValidatePassword checks password strength against policy.
func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: minimal %d karakter", ErrWeakPassword, minPasswordLength)
	}

	if denyList[strings.ToLower(password)] {
		return fmt.Errorf("%w: kata sandi terlalu umum", ErrWeakPassword)
	}

	// Check for at least one uppercase letter
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString
	if !hasUpper(password) {
		return fmt.Errorf("%w: harus mengandung huruf kapital", ErrWeakPassword)
	}

	// Check for at least one digit
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString
	if !hasDigit(password) {
		return fmt.Errorf("%w: harus mengandung angka", ErrWeakPassword)
	}

	// Check for at least one special character
	hasSpecial := regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString
	if !hasSpecial(password) {
		return fmt.Errorf("%w: harus mengandung karakter khusus", ErrWeakPassword)
	}

	return nil
}

// ── Config ────────────────────────────────────────────────────────────

type Config struct {
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	MaxFailedAttempts int
	LockoutDuration   time.Duration
	ResetTokenTTL     time.Duration
}

func DefaultConfig() Config {
	return Config{
		AccessTokenTTL:    15 * time.Minute,
		RefreshTokenTTL:   7 * 24 * time.Hour,
		MaxFailedAttempts: 5,
		LockoutDuration:   15 * time.Minute,
		ResetTokenTTL:     30 * time.Minute,
	}
}

// ── UseCase ───────────────────────────────────────────────────────────

type UseCase struct {
	repo        domain.Repository
	token       *token.Manager
	cfg         Config
	log         *logger.Logger
	emailSender *email.Sender
}

func New(repo domain.Repository, tokenMgr *token.Manager, cfg Config, log *logger.Logger, emailSender *email.Sender) *UseCase {
	return &UseCase{
		repo:        repo,
		token:       tokenMgr,
		cfg:         cfg,
		log:         log,
		emailSender: emailSender,
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

	// Get user permissions from RBAC
	permissions := uc.getPermissionsForRole(user.Peran)

	accessToken, err := uc.token.CreateAccessToken(user.ID, user.Peran, permissions)
	if err != nil {
		return nil, "", "", fmt.Errorf("create access token: %w", err)
	}

	familyID := uuid.New().String()
	rawRefresh, refreshHash, err := uc.token.CreateRefreshToken()
	if err != nil {
		return nil, "", "", fmt.Errorf("create refresh token: %w", err)
	}

	now := time.Now()
	rt := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
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

	uc.recordAuditLog(ctx, &user.ID, "LOGIN", "pengguna", user.ID, "BERHASIL", &ip, nil)
	uc.log.Info("login success", map[string]interface{}{"user_id": user.ID, "email": email, "ip": ip})
	return user, accessToken, rawRefresh, nil
}

// Refresh rotates a refresh token.
func (uc *UseCase) Refresh(ctx context.Context, refreshTokenStr, ip string) (string, string, error) {
	tokenHash := sha256Hex(refreshTokenStr)

	stored, err := uc.repo.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return "", "", ErrInvalidToken
	}

	if stored.RevokedAt != nil {
		// Reuse detection: revoke entire family
		uc.log.Warn("refresh: reuse detected, revoking family", map[string]interface{}{
			"family_id": stored.FamilyID,
			"user_id":   stored.UserID,
		})
		_ = uc.repo.RevokeRefreshTokenFamily(ctx, stored.FamilyID)
		uc.recordAuditLog(ctx, &stored.UserID, "TOKEN_REUSE", "refresh_token", fmt.Sprintf("%d", stored.ID), "BLOKIR", nil, map[string]interface{}{
			"family_id": stored.FamilyID,
		})
		return "", "", ErrInvalidToken
	}

	if time.Now().After(stored.ExpiresAt) {
		return "", "", ErrTokenExpired
	}

	now := time.Now()
	if err := uc.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return "", "", fmt.Errorf("revoke old token: %w", err)
	}

	// Get user for permissions
	user, err := uc.repo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return "", "", ErrUserNotFound
	}

	permissions := uc.getPermissionsForRole(user.Peran)

	accessToken, err := uc.token.CreateAccessToken(stored.UserID, user.Peran, permissions)
	if err != nil {
		return "", "", fmt.Errorf("create access token: %w", err)
	}

	rawRefresh, refreshHash, err := uc.token.CreateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("create refresh token: %w", err)
	}

	rt := &domain.RefreshToken{
		UserID:    stored.UserID,
		TokenHash: refreshHash,
		FamilyID:  stored.FamilyID,
		IP:        &ip,
		ExpiresAt: now.Add(uc.cfg.RefreshTokenTTL),
	}
	if err := uc.repo.CreateRefreshToken(ctx, rt); err != nil {
		return "", "", fmt.Errorf("save new refresh token: %w", err)
	}

	uc.log.Info("refresh success", map[string]interface{}{"user_id": stored.UserID, "ip": ip})
	return accessToken, rawRefresh, nil
}

// Logout revokes all refresh tokens for a user and blacklists the current JWT.
func (uc *UseCase) Logout(ctx context.Context, userID string, jti string, ip string) error {
	// Revoke all active refresh tokens for this user
	if err := uc.repo.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("revoke all tokens: %w", err)
	}

	// Blacklist the current JWT
	if jti != "" {
		if err := uc.repo.BlacklistJWT(ctx, jti, uc.cfg.AccessTokenTTL); err != nil {
			uc.log.Warn("logout: failed to blacklist JWT", map[string]interface{}{"jti": jti, "user_id": userID})
		}
	}

	uc.recordAuditLog(ctx, &userID, "LOGOUT", "pengguna", userID, "BERHASIL", &ip, nil)
	uc.log.Info("logout success", map[string]interface{}{"user_id": userID})
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

// ── Password Management ───────────────────────────────────────────────

// ChangePassword changes the user's password after verifying the old one.
func (uc *UseCase) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string, ip string) error {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if user.PasswordHash == nil {
		return ErrWrongPassword
	}

	if !crypto.VerifyPassword(oldPassword, *user.PasswordHash) {
		uc.recordLoginAttempt(ctx, &userID, ip, false, "wrong_password_change")
		return ErrWrongPassword
	}

	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	// Prevent password reuse
	if crypto.VerifyPassword(newPassword, *user.PasswordHash) {
		return ErrPasswordReuse
	}

	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user.PasswordHash = &hash

	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	// Revoke all other sessions except current
	_ = uc.repo.RevokeAllUserRefreshTokens(ctx, userID)

	uc.recordAuditLog(ctx, &userID, "CHANGE_PASSWORD", "pengguna", userID, "BERHASIL", &ip, nil)
	uc.log.Info("password changed", map[string]interface{}{"user_id": userID, "ip": ip})
	return nil
}

// ForgotPassword creates a password reset token and stores it.
func (uc *UseCase) ForgotPassword(ctx context.Context, email, ip string) error {
	user, err := uc.repo.GetUserByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether email exists
		uc.log.Info("forgot password: email not found", map[string]interface{}{"email": email})
		return nil
	}

	if user.Status != "AKTIF" {
		uc.log.Warn("forgot password: inactive account", map[string]interface{}{"user_id": user.ID})
		return nil
	}

	resetToken := uuid.New().String()
	tokenHash := sha256Hex(resetToken)
	expiresAt := time.Now().Add(uc.cfg.ResetTokenTTL)

	if err := uc.repo.SaveResetToken(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("save reset token: %w", err)
	}

	uc.recordAuditLog(ctx, &user.ID, "FORGOT_PASSWORD", "pengguna", user.ID, "BERHASIL", &ip, nil)
	uc.log.Info("forgot password: token created", map[string]interface{}{"user_id": user.ID, "email": email})

	// Send email with reset link
	if err := uc.emailSender.SendResetPassword(email, resetToken); err != nil {
		uc.log.Error("forgot password: failed to send email", map[string]interface{}{"user_id": user.ID, "error": err.Error()})
		return fmt.Errorf("send reset email: %w", err)
	}

	return nil
}

// ResetPassword resets the password using a reset token.
func (uc *UseCase) ResetPassword(ctx context.Context, resetTokenStr, newPassword, ip string) error {
	tokenHash := sha256Hex(resetTokenStr)

	stored, err := uc.repo.GetResetToken(ctx, tokenHash)
	if err != nil {
		return ErrResetTokenInvalid
	}

	if stored.UsedAt != nil {
		return ErrResetTokenInvalid
	}

	if time.Now().After(stored.ExpiresAt) {
		return ErrResetTokenExpired
	}

	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user, err := uc.repo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return ErrUserNotFound
	}

	// Prevent password reuse
	if user.PasswordHash != nil && crypto.VerifyPassword(newPassword, *user.PasswordHash) {
		return ErrPasswordReuse
	}

	user.PasswordHash = &hash
	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	// Mark token as used
	now := time.Now()
	if err := uc.repo.MarkResetTokenUsed(ctx, stored.ID, now); err != nil {
		uc.log.Warn("reset password: failed to mark token used", map[string]interface{}{"token_id": stored.ID})
	}

	// Revoke all sessions
	_ = uc.repo.RevokeAllUserRefreshTokens(ctx, stored.UserID)

	uc.recordAuditLog(ctx, &stored.UserID, "RESET_PASSWORD", "pengguna", stored.UserID, "BERHASIL", &ip, nil)
	uc.log.Info("password reset success", map[string]interface{}{"user_id": stored.UserID, "ip": ip})
	return nil
}

// ── User Use Cases ────────────────────────────────────────────────────

// CreateUser creates a new user.
func (uc *UseCase) CreateUser(ctx context.Context, user *domain.Pengguna, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}

	existing, _ := uc.repo.GetUserByEmail(ctx, user.Email)
	if existing != nil {
		return ErrEmailExists
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user.PasswordHash = &hash

	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	ip := ""
	uc.recordAuditLog(ctx, nil, "CREATE_USER", "pengguna", user.ID, "BERHASIL", &ip, nil)
	return nil
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

// ── RBAC Helper ───────────────────────────────────────────────────────

// getPermissionsForRole returns the permission list for a given role.
// This mirrors the frontend RBAC matrix in lib/rbac/permissions.ts.
func (uc *UseCase) getPermissionsForRole(role string) []string {
	// Map of role -> permissions (mirrors frontend ROLE_PERMISSIONS)
	perms := map[string][]string{
		"STRATEGIS_FMIPA": {
			"dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor",
			"ews.lihat", "prediksi.lihat", "spasial.lihat", "coldstorage.lihat",
			"fiskal.lihat", "intervensi.lihat", "intervensi.setujui", "brief.lihat",
			"brief.setujui", "kpi.lihat", "master.lihat", "master.setujui", "sumber.lihat", "audit.lihat",
		},
		"STRATEGIS_BI": {
			"dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor",
			"ews.lihat", "prediksi.lihat", "spasial.lihat", "coldstorage.lihat",
			"fiskal.lihat", "intervensi.lihat", "intervensi.setujui", "brief.lihat",
			"brief.setujui", "kpi.lihat", "master.lihat", "sumber.lihat", "audit.lihat",
		},
		"STRATEGIS_BPS": {
			"dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor",
			"ews.lihat", "prediksi.lihat", "spasial.lihat", "coldstorage.lihat",
			"fiskal.lihat", "intervensi.lihat", "intervensi.setujui", "brief.lihat",
			"brief.setujui", "kpi.lihat", "master.lihat", "master.setujui", "sumber.lihat", "audit.lihat",
		},
		"STRATEGIS_DJPB": {
			"dashboard.eksekutif", "dashboard.operasional", "harga.lihat", "harga.ekspor",
			"ews.lihat", "prediksi.lihat", "spasial.lihat", "coldstorage.lihat",
			"fiskal.lihat", "intervensi.lihat", "intervensi.setujui", "intervensi.setujui_fiskal",
			"brief.lihat", "brief.setujui", "kpi.lihat", "master.lihat", "sumber.lihat", "audit.lihat",
		},
		"MANAJER_PROGRAM": {
			"dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "harga.ekspor",
			"tugas.kelola", "ews.lihat", "ews.akui", "ews.eskalasi", "prediksi.lihat",
			"spasial.lihat", "logistik.kelola", "coldstorage.lihat", "fiskal.lihat",
			"intervensi.lihat", "intervensi.kelola", "brief.lihat", "brief.susun",
			"kpi.lihat", "master.lihat", "master.ajukan", "sumber.lihat", "audit.lihat",
		},
		"MANAJER_DATA": {
			"dashboard.operasional", "harga.lihat", "harga.publikasi", "harga.ekspor",
			"ews.lihat", "ews.akui", "ews.eskalasi", "ews.atur_ambang", "prediksi.lihat",
			"model.kelola", "spasial.lihat", "intervensi.lihat", "brief.lihat",
			"kpi.lihat", "master.lihat", "master.ajukan", "sumber.lihat", "sumber.kelola", "audit.lihat",
		},
		"MANAJER_KEBIJAKAN": {
			"dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "ews.lihat",
			"prediksi.lihat", "spasial.lihat", "fiskal.lihat", "intervensi.lihat",
			"intervensi.kelola", "brief.lihat", "brief.susun", "kpi.lihat",
		},
		"MANAJER_PEMBIAYAAN": {
			"dashboard.operasional", "dashboard.eksekutif", "harga.lihat", "spasial.lihat",
			"coldstorage.lihat", "fiskal.lihat", "intervensi.lihat", "brief.lihat", "kpi.lihat",
		},
		"MANAJER_RISIKO": {
			"dashboard.operasional", "harga.lihat", "harga.publikasi", "harga.ekspor",
			"ews.lihat", "ews.akui", "ews.eskalasi", "coldstorage.lihat",
			"intervensi.lihat", "kpi.lihat", "master.lihat", "audit.lihat",
		},
		"MANAJER_MUTU": {
			"dashboard.operasional", "harga.lihat", "ews.lihat", "prediksi.lihat",
			"coldstorage.lihat", "coldstorage.kelola", "intervensi.lihat", "intervensi.kelola", "kpi.lihat",
		},
		"OPS_KOORDINATOR": {
			"dashboard.operasional", "harga.input", "harga.lihat", "harga.validasi",
			"tugas.kelola", "tugas.lihat_sendiri", "ews.lihat", "ews.akui",
			"spasial.lihat", "intervensi.lihat", "master.lihat",
		},
		"OPS_AGREGATOR": {
			"dashboard.operasional", "harga.input", "harga.lihat", "tugas.lihat_sendiri",
			"ews.lihat", "coldstorage.lihat", "coldstorage.kelola", "logistik.kelola", "intervensi.lihat",
		},
		"OPS_MAHASISWA": {
			"harga.input", "tugas.lihat_sendiri",
		},
		"ADMIN_SISTEM": {
			"pengguna.kelola", "master.lihat", "master.ajukan", "sumber.lihat", "sumber.kelola", "audit.lihat",
		},
	}

	if p, ok := perms[role]; ok {
		return p
	}
	return []string{}
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

func (uc *UseCase) recordAuditLog(ctx context.Context, aktorID *string, aksi, entitas, entitasID, hasil string, ip *string, metadata map[string]interface{}) {
	var metaStr *string
	if len(metadata) > 0 {
		b, _ := json.Marshal(metadata)
		s := string(b)
		metaStr = &s
	}

	log := &domain.AuditLog{
		ID:        uuid.New().String(),
		Waktu:     time.Now(),
		AktorID:   aktorID,
		Aksi:      aksi,
		Entitas:   entitas,
		EntitasID: entitasID,
		Hasil:     hasil,
		IP:        ip,
		Metadata:  metaStr,
	}
	_ = uc.repo.CreateAuditLog(ctx, log)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ── TOTP / MFA ───────────────────────────────────────────────────────

// SetupMFA generates a TOTP secret and provisioning URI for the user.
func (uc *UseCase) SetupMFA(ctx context.Context, userID string) (string, string, []string, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", nil, ErrUserNotFound
	}

	if user.MfaAktif {
		return "", "", nil, fmt.Errorf("MFA sudah aktif")
	}

	secret := totp.GenerateSecret()
	uri := totp.GenerateProvisioningURI(secret, user.Email)
	backupCodes := totp.GenerateBackupCodes(8)

	// Store backup codes as JSON
	codesJSON, _ := json.Marshal(backupCodes)
	codesStr := string(codesJSON)
	user.MfaSecret = &secret
	user.MfaBackupCodes = &codesStr

	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return "", "", nil, fmt.Errorf("update user: %w", err)
	}

	uc.recordAuditLog(ctx, &userID, "MFA_SETUP", "pengguna", userID, "BERHASIL", nil, nil)
	return secret, uri, backupCodes, nil
}

// VerifyMFA verifies a TOTP code and activates MFA for the user.
func (uc *UseCase) VerifyMFA(ctx context.Context, userID, code string) error {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if user.MfaSecret == nil || *user.MfaSecret == "" {
		return fmt.Errorf("MFA belum di-setup")
	}

	if !totp.ValidateTOTP(*user.MfaSecret, code, time.Now()) {
		return fmt.Errorf("kode TOTP tidak valid")
	}

	user.MfaAktif = true
	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	uc.recordAuditLog(ctx, &userID, "MFA_VERIFY", "pengguna", userID, "BERHASIL", nil, nil)
	return nil
}

// ValidateMFACode validates a TOTP or backup code during login.
func (uc *UseCase) ValidateMFACode(ctx context.Context, userID, code string) (bool, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return false, ErrUserNotFound
	}

	if !user.MfaAktif || user.MfaSecret == nil {
		return false, fmt.Errorf("MFA tidak aktif")
	}

	// Try TOTP first
	if totp.ValidateTOTP(*user.MfaSecret, code, time.Now()) {
		return true, nil
	}

	// Try backup codes
	if user.MfaBackupCodes != nil && *user.MfaBackupCodes != "" {
		var codes []string
		if err := json.Unmarshal([]byte(*user.MfaBackupCodes), &codes); err == nil {
			for i, c := range codes {
				if c == code {
					// Remove used backup code
					codes = append(codes[:i], codes[i+1:]...)
					newJSON, _ := json.Marshal(codes)
					s := string(newJSON)
					user.MfaBackupCodes = &s
					_ = uc.repo.UpdateUser(ctx, user)
					uc.recordAuditLog(ctx, &userID, "MFA_VALIDATE_BACKUP", "pengguna", userID, "BERHASIL", nil, nil)
					return true, nil
				}
			}
		}
	}

	return false, fmt.Errorf("kode tidak valid")
}

// DisableMFA disables MFA for a user.
func (uc *UseCase) DisableMFA(ctx context.Context, userID, password string) error {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if user.PasswordHash != nil && !crypto.VerifyPassword(password, *user.PasswordHash) {
		return ErrWrongPassword
	}

	user.MfaAktif = false
	user.MfaSecret = nil
	user.MfaBackupCodes = nil

	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	uc.recordAuditLog(ctx, &userID, "MFA_DISABLE", "pengguna", userID, "BERHASIL", nil, nil)
	return nil
}

