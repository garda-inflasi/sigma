package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/garda-inflasi/api/pkg/totp"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) GetUserByID(ctx context.Context, id string) (*domain.Pengguna, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Pengguna), args.Error(1)
}

func (m *mockRepository) GetUserByEmail(ctx context.Context, email string) (*domain.Pengguna, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Pengguna), args.Error(1)
}

func (m *mockRepository) CreateUser(ctx context.Context, user *domain.Pengguna) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockRepository) UpdateUser(ctx context.Context, user *domain.Pengguna) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *mockRepository) ListUsers(ctx context.Context, page, limit int) ([]domain.Pengguna, int64, error) {
	args := m.Called(ctx, page, limit)
	return args.Get(0).([]domain.Pengguna), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) CreateRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *mockRepository) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}

func (m *mockRepository) RevokeRefreshToken(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepository) RevokeRefreshTokenFamily(ctx context.Context, familyID string) error {
	args := m.Called(ctx, familyID)
	return args.Error(0)
}

func (m *mockRepository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockRepository) BlacklistJWT(ctx context.Context, jti string, ttl time.Duration) error {
	args := m.Called(ctx, jti, ttl)
	return args.Error(0)
}

func (m *mockRepository) IsJWTBlacklisted(ctx context.Context, jti string) (bool, error) {
	args := m.Called(ctx, jti)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepository) CreateLoginAttempt(ctx context.Context, attempt *domain.LoginAttempt) error {
	args := m.Called(ctx, attempt)
	return args.Error(0)
}

func (m *mockRepository) CountRecentFailedAttempts(ctx context.Context, userID string, since time.Time) (int64, error) {
	args := m.Called(ctx, userID, since)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockRepository) SaveResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	args := m.Called(ctx, userID, tokenHash, expiresAt)
	return args.Error(0)
}

func (m *mockRepository) GetResetToken(ctx context.Context, tokenHash string) (*domain.ResetToken, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ResetToken), args.Error(1)
}

func (m *mockRepository) MarkResetTokenUsed(ctx context.Context, id int64, usedAt time.Time) error {
	args := m.Called(ctx, id, usedAt)
	return args.Error(0)
}

func (m *mockRepository) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) *UseCase {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!",
		token.WithAccessTTL(15*time.Minute),
		token.WithRefreshTTL(7*24*time.Hour),
	)
	cfg := DefaultConfig()
	log := logger.NewDefault()
	emailSender := email.New(email.Config{}) // SMTP not configured — no-op sender
	return New(repo, tokenMgr, cfg, log, emailSender)
}

func activeUser() *domain.Pengguna {
	passwordHash, _ := crypto.HashPassword("Test1234!")
	return &domain.Pengguna{
		ID:           "user-123",
		Nama:         "Test User",
		Email:        "test@example.com",
		PasswordHash: &passwordHash,
		Peran:        "OPS_MAHASISWA",
		Lapis:        "OPS",
		Status:       "AKTIF",
	}
}

// ── Login Tests ───────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
	repo.On("CountRecentFailedAttempts", mock.Anything, "user-123", mock.Anything).Return(int64(0), nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.AnythingOfType("*domain.LoginAttempt")).Return(nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	result, accessToken, refreshToken, err := uc.Login(context.Background(), "test@example.com", "Test1234!", "10.0.0.1")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "user-123", result.ID)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	repo.AssertExpectations(t)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetUserByEmail", mock.Anything, "wrong@example.com").Return(nil, errors.New("not found"))
	repo.On("CreateLoginAttempt", mock.Anything, mock.AnythingOfType("*domain.LoginAttempt")).Return(nil)

	result, _, _, err := uc.Login(context.Background(), "wrong@example.com", "Test1234!", "10.0.0.1")

	assert.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

func TestLogin_InactiveAccount(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	user.Status = "NONAKTIF"

	repo.On("GetUserByEmail", mock.Anything, "inactive@example.com").Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.AnythingOfType("*domain.LoginAttempt")).Return(nil)

	result, _, _, err := uc.Login(context.Background(), "inactive@example.com", "Test1234!", "10.0.0.1")

	assert.ErrorIs(t, err, ErrAccountInactive)
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

func TestLogin_AccountLocked(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByEmail", mock.Anything, "locked@example.com").Return(user, nil)
	repo.On("CountRecentFailedAttempts", mock.Anything, "user-123", mock.Anything).Return(int64(10), nil)

	result, _, _, err := uc.Login(context.Background(), "locked@example.com", "Test1234!", "10.0.0.1")

	assert.ErrorIs(t, err, ErrAccountLocked)
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
	repo.On("CountRecentFailedAttempts", mock.Anything, "user-123", mock.Anything).Return(int64(0), nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.AnythingOfType("*domain.LoginAttempt")).Return(nil)

	result, _, _, err := uc.Login(context.Background(), "test@example.com", "WrongPass1!", "10.0.0.1")

	assert.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

func TestLogin_NoPassword(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	user.PasswordHash = nil // SSO-only user

	repo.On("GetUserByEmail", mock.Anything, "sso@example.com").Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.AnythingOfType("*domain.LoginAttempt")).Return(nil)

	result, _, _, err := uc.Login(context.Background(), "sso@example.com", "Test1234!", "10.0.0.1")

	assert.ErrorIs(t, err, ErrInvalidCredentials)
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

// ── Refresh Tests ─────────────────────────────────────────────────────

func TestRefresh_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	// We need to know the hash that will be passed to GetRefreshTokenByHash.
	// Since CreateRefreshToken generates random values, we'll set up the mock
	// to return a stored token for any hash, then verify the flow.
	storedToken := &domain.RefreshToken{
		ID:        1,
		UserID:    "user-123",
		TokenHash: "any-hash-will-match",
		FamilyID:  "family-uuid",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	repo.On("GetRefreshTokenByHash", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)
	repo.On("RevokeRefreshToken", mock.Anything, int64(1)).Return(nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("CreateRefreshToken", mock.Anything, mock.AnythingOfType("*domain.RefreshToken")).Return(nil)

	accessToken, newRefreshToken, err := uc.Refresh(context.Background(), "some-refresh-token", "10.0.0.1")

	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, newRefreshToken)

	repo.AssertExpectations(t)
}

func TestRefresh_InvalidToken(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	// The usecase hashes the input before looking up, so we need to match
	// the hash of our input, not the raw input itself.
	repo.On("GetRefreshTokenByHash", mock.Anything, mock.AnythingOfType("string")).Return(nil, errors.New("not found"))

	_, _, err := uc.Refresh(context.Background(), "nonexistent-token", "10.0.0.1")

	assert.ErrorIs(t, err, ErrInvalidToken)

	repo.AssertExpectations(t)
}

func TestRefresh_RevokedToken(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	now := time.Now()
	storedToken := &domain.RefreshToken{
		ID:        1,
		UserID:    "user-123",
		TokenHash: "revoked-hash",
		FamilyID:  "family-uuid",
		RevokedAt: &now,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	repo.On("GetRefreshTokenByHash", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)
	repo.On("RevokeRefreshTokenFamily", mock.Anything, "family-uuid").Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	_, _, err := uc.Refresh(context.Background(), "revoked-token", "10.0.0.1")

	assert.ErrorIs(t, err, ErrInvalidToken)

	repo.AssertExpectations(t)
}

func TestRefresh_ExpiredToken(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	storedToken := &domain.RefreshToken{
		ID:        1,
		UserID:    "user-123",
		TokenHash: "expired-hash",
		FamilyID:  "family-uuid",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	repo.On("GetRefreshTokenByHash", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)

	_, _, err := uc.Refresh(context.Background(), "expired-token", "10.0.0.1")
	assert.ErrorIs(t, err, ErrTokenExpired)

	repo.AssertExpectations(t)
}

// ── Logout Tests ──────────────────────────────────────────────────────

func TestLogout_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)
	repo.On("BlacklistJWT", mock.Anything, "jti-123", mock.Anything).Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := uc.Logout(context.Background(), "user-123", "jti-123", "10.0.0.1")

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestLogout_NoJTI(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := uc.Logout(context.Background(), "user-123", "", "10.0.0.1")

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

// ── GetMe Tests ───────────────────────────────────────────────────────

func TestGetMe_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	result, err := uc.GetMe(context.Background(), "user-123")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "user-123", result.ID)
	assert.Equal(t, "test@example.com", result.Email)

	repo.AssertExpectations(t)
}

func TestGetMe_NotFound(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetUserByID", mock.Anything, "nonexistent").Return(nil, errors.New("not found"))

	result, err := uc.GetMe(context.Background(), "nonexistent")

	assert.ErrorIs(t, err, ErrUserNotFound)
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

// ── ChangePassword Tests ──────────────────────────────────────────────

func TestChangePassword_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := uc.ChangePassword(context.Background(), "user-123", "Test1234!", "NewTest1@", "10.0.0.1")

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestChangePassword_WrongOldPassword(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("CreateLoginAttempt", mock.Anything, mock.AnythingOfType("*domain.LoginAttempt")).Return(nil)

	err := uc.ChangePassword(context.Background(), "user-123", "WrongOld1!", "NewTest1@", "10.0.0.1")

	assert.ErrorIs(t, err, ErrWrongPassword)

	repo.AssertExpectations(t)
}

func TestChangePassword_WeakPassword(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	err := uc.ChangePassword(context.Background(), "user-123", "Test1234!", "weak", "10.0.0.1")

	assert.ErrorIs(t, err, ErrWeakPassword)

	repo.AssertExpectations(t)
}

func TestChangePassword_PasswordReuse(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	err := uc.ChangePassword(context.Background(), "user-123", "Test1234!", "Test1234!", "10.0.0.1")

	assert.ErrorIs(t, err, ErrPasswordReuse)

	repo.AssertExpectations(t)
}

// ── ForgotPassword Tests ──────────────────────────────────────────────

func TestForgotPassword_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByEmail", mock.Anything, "test@example.com").Return(user, nil)
	repo.On("SaveResetToken", mock.Anything, "user-123", mock.AnythingOfType("string"), mock.Anything).Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := uc.ForgotPassword(context.Background(), "test@example.com", "10.0.0.1")

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestForgotPassword_EmailNotFound(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetUserByEmail", mock.Anything, "unknown@example.com").Return(nil, errors.New("not found"))

	// Should not return error to prevent email enumeration
	err := uc.ForgotPassword(context.Background(), "unknown@example.com", "10.0.0.1")

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestForgotPassword_InactiveAccount(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	user.Status = "NONAKTIF"

	repo.On("GetUserByEmail", mock.Anything, "inactive@example.com").Return(user, nil)

	// Should not return error to prevent email enumeration
	err := uc.ForgotPassword(context.Background(), "inactive@example.com", "10.0.0.1")

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

// ── ResetPassword Tests ───────────────────────────────────────────────

func TestResetPassword_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	storedToken := &domain.ResetToken{
		ID:        1,
		UserID:    "user-123",
		TokenHash: "valid-token-hash",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	repo.On("GetResetToken", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)
	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("MarkResetTokenUsed", mock.Anything, int64(1), mock.Anything).Return(nil)
	repo.On("RevokeAllUserRefreshTokens", mock.Anything, "user-123").Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := uc.ResetPassword(context.Background(), "valid-token-hash", "NewPass1@", "10.0.0.1")

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestResetPassword_InvalidToken(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetResetToken", mock.Anything, mock.AnythingOfType("string")).Return(nil, errors.New("not found"))

	err := uc.ResetPassword(context.Background(), "invalid-hash", "NewPass1@", "10.0.0.1")

	assert.ErrorIs(t, err, ErrResetTokenInvalid)

	repo.AssertExpectations(t)
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	storedToken := &domain.ResetToken{
		ID:        1,
		UserID:    "user-123",
		TokenHash: "expired-hash",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	repo.On("GetResetToken", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)

	err := uc.ResetPassword(context.Background(), "expired-token", "NewPass1@", "10.0.0.1")

	assert.ErrorIs(t, err, ErrResetTokenExpired)

	repo.AssertExpectations(t)
}

func TestResetPassword_UsedToken(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	now := time.Now()

	storedToken := &domain.ResetToken{
		ID:        1,
		UserID:    "user-123",
		TokenHash: "used-hash",
		ExpiresAt: time.Now().Add(30 * time.Minute),
		UsedAt:    &now,
	}

	repo.On("GetResetToken", mock.Anything, mock.AnythingOfType("string")).Return(storedToken, nil)

	err := uc.ResetPassword(context.Background(), "used-token", "NewPass1@", "10.0.0.1")

	assert.ErrorIs(t, err, ErrResetTokenInvalid)

	repo.AssertExpectations(t)
}

// ── Password Validation Tests ─────────────────────────────────────────

func TestValidatePassword_Valid(t *testing.T) {
	assert.NoError(t, ValidatePassword("Test1234!"))
	assert.NoError(t, ValidatePassword("Complex1@#"))
	assert.NoError(t, ValidatePassword("Str0ng!Pass"))
}

func TestValidatePassword_TooShort(t *testing.T) {
	err := ValidatePassword("Ab1!")
	assert.ErrorIs(t, err, ErrWeakPassword)
}

func TestValidatePassword_NoUppercase(t *testing.T) {
	err := ValidatePassword("test1234!")
	assert.ErrorIs(t, err, ErrWeakPassword)
}

func TestValidatePassword_NoDigit(t *testing.T) {
	err := ValidatePassword("Testabcd!")
	assert.ErrorIs(t, err, ErrWeakPassword)
}

func TestValidatePassword_NoSpecialChar(t *testing.T) {
	err := ValidatePassword("Test1234")
	assert.ErrorIs(t, err, ErrWeakPassword)
}

func TestValidatePassword_DeniedList(t *testing.T) {
	// These passwords are not in the deny list (deny list entries are lowercase without special chars)
	err := ValidatePassword("Password1!")
	assert.NoError(t, err)

	err = ValidatePassword("Admin123!")
	assert.NoError(t, err)
}

// ── CreateUser Tests ──────────────────────────────────────────────────

func TestCreateUser_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	user := &domain.Pengguna{
		ID:    "new-user",
		Nama:  "New User",
		Email: "new@example.com",
		Peran: "OPS_MAHASISWA",
		Lapis: "OPS",
	}

	repo.On("GetUserByEmail", mock.Anything, "new@example.com").Return(nil, errors.New("not found"))
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := uc.CreateUser(context.Background(), user, "NewUser1!")

	require.NoError(t, err)
	assert.NotNil(t, user.PasswordHash)
	assert.True(t, len(*user.PasswordHash) > 0)

	repo.AssertExpectations(t)
}

func TestCreateUser_EmailExists(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	user := &domain.Pengguna{
		ID:    "existing-user",
		Email: "existing@example.com",
	}

	repo.On("GetUserByEmail", mock.Anything, "existing@example.com").Return(activeUser(), nil)

	err := uc.CreateUser(context.Background(), user, "NewUser1!")

	assert.ErrorIs(t, err, ErrEmailExists)

	repo.AssertExpectations(t)
}

func TestCreateUser_WeakPassword(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	user := &domain.Pengguna{
		ID:    "new-user",
		Email: "new@example.com",
	}

	err := uc.CreateUser(context.Background(), user, "weak")

	assert.ErrorIs(t, err, ErrWeakPassword)

	repo.AssertExpectations(t)
}

// ── RBAC Helper Tests ─────────────────────────────────────────────────

func TestGetPermissionsForRole(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	// Test a few roles
	perms := uc.getPermissionsForRole("ADMIN_SISTEM")
	assert.Contains(t, perms, "pengguna.kelola")
	assert.Contains(t, perms, "audit.lihat")
	assert.NotContains(t, perms, "harga.input")

	perms = uc.getPermissionsForRole("OPS_MAHASISWA")
	assert.Contains(t, perms, "harga.input")
	assert.NotContains(t, perms, "pengguna.kelola")

	perms = uc.getPermissionsForRole("STRATEGIS_FMIPA")
	assert.Contains(t, perms, "dashboard.eksekutif")
	assert.Contains(t, perms, "intervensi.setujui")
	assert.NotContains(t, perms, "harga.input")
}

func TestGetPermissionsForRole_UnknownRole(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	perms := uc.getPermissionsForRole("UNKNOWN_ROLE")
	assert.Empty(t, perms)
}

// ── MFA Tests ─────────────────────────────────────────────────────────

func TestSetupMFA_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	secret, uri, backupCodes, err := uc.SetupMFA(context.Background(), "user-123")

	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, uri, "otpauth://totp/")
	assert.Len(t, backupCodes, 8)

	repo.AssertExpectations(t)
}

func TestSetupMFA_AlreadyActive(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	user.MfaAktif = true

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	_, _, _, err := uc.SetupMFA(context.Background(), "user-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sudah aktif")

	repo.AssertExpectations(t)
}

func TestSetupMFA_UserNotFound(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetUserByID", mock.Anything, "nonexistent").Return(nil, errors.New("not found"))

	_, _, _, err := uc.SetupMFA(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrUserNotFound)

	repo.AssertExpectations(t)
}

func TestVerifyMFA_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	// Setup MFA first
	secret := totp.GenerateSecret()
	user.MfaSecret = &secret

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	// Generate a valid TOTP code
	code, err := totp.GenerateTOTP(secret, time.Now())
	require.NoError(t, err)

	err = uc.VerifyMFA(context.Background(), "user-123", code)
	require.NoError(t, err)
	assert.True(t, user.MfaAktif)

	repo.AssertExpectations(t)
}

func TestVerifyMFA_InvalidCode(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	secret := totp.GenerateSecret()
	user.MfaSecret = &secret

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	err := uc.VerifyMFA(context.Background(), "user-123", "000000")
	assert.Error(t, err)

	repo.AssertExpectations(t)
}

func TestVerifyMFA_NoSecret(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	err := uc.VerifyMFA(context.Background(), "user-123", "123456")
	assert.Error(t, err)

	repo.AssertExpectations(t)
}

func TestValidateMFACode_TOTP(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	user.MfaAktif = true
	secret := totp.GenerateSecret()
	user.MfaSecret = &secret

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	code, err := totp.GenerateTOTP(secret, time.Now())
	require.NoError(t, err)

	valid, err := uc.ValidateMFACode(context.Background(), "user-123", code)
	require.NoError(t, err)
	assert.True(t, valid)

	repo.AssertExpectations(t)
}

func TestValidateMFACode_BackupCode(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	user.MfaAktif = true
	secret := totp.GenerateSecret()
	user.MfaSecret = &secret
	codes := []string{"CODE0001", "CODE0002"}
	codesJSON, _ := json.Marshal(codes)
	s := string(codesJSON)
	user.MfaBackupCodes = &s

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil).Once()

	valid, err := uc.ValidateMFACode(context.Background(), "user-123", "CODE0001")
	require.NoError(t, err)
	assert.True(t, valid)

	repo.AssertExpectations(t)
}

func TestValidateMFACode_MFANotActive(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	_, err := uc.ValidateMFACode(context.Background(), "user-123", "123456")
	assert.Error(t, err)

	repo.AssertExpectations(t)
}

func TestDisableMFA_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()
	user.MfaAktif = true

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*domain.Pengguna")).Return(nil)
	repo.On("CreateAuditLog", mock.Anything, mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	err := uc.DisableMFA(context.Background(), "user-123", "Test1234!")
	require.NoError(t, err)
	assert.False(t, user.MfaAktif)
	assert.Nil(t, user.MfaSecret)
	assert.Nil(t, user.MfaBackupCodes)

	repo.AssertExpectations(t)
}

func TestDisableMFA_WrongPassword(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)
	user := activeUser()

	repo.On("GetUserByID", mock.Anything, "user-123").Return(user, nil)

	err := uc.DisableMFA(context.Background(), "user-123", "WrongPass1!")
	assert.ErrorIs(t, err, ErrWrongPassword)

	repo.AssertExpectations(t)
}

func TestDisableMFA_UserNotFound(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetUserByID", mock.Anything, "nonexistent").Return(nil, errors.New("not found"))

	err := uc.DisableMFA(context.Background(), "nonexistent", "Test1234!")
	assert.ErrorIs(t, err, ErrUserNotFound)

	repo.AssertExpectations(t)
}
