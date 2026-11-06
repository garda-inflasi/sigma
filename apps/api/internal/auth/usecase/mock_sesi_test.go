package usecase

import (
	"context"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// Metode domain.SesiRepository pada mockRepository (didefinisikan di sini agar usecase_test.go tidak berubah).

func (m *mockRepository) RotateRefreshToken(ctx context.Context, oldID int64, next *domain.RefreshToken, now time.Time) (bool, error) {
	args := m.Called(ctx, oldID, next, now)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepository) IsSessionActive(ctx context.Context, familyID string) (bool, error) {
	args := m.Called(ctx, familyID)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepository) ListActiveRefreshTokens(ctx context.Context, userID string, now time.Time) ([]domain.RefreshToken, error) {
	args := m.Called(ctx, userID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.RefreshToken), args.Error(1)
}

func (m *mockRepository) RevokeSessions(ctx context.Context, userID, keepFamilyID string, now time.Time) ([]string, error) {
	args := m.Called(ctx, userID, keepFamilyID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockRepository) RevokeSession(ctx context.Context, userID, familyID string, now time.Time) (bool, error) {
	args := m.Called(ctx, userID, familyID, now)
	return args.Bool(0), args.Error(1)
}

func (m *mockRepository) TouchLastLogin(ctx context.Context, userID string, at time.Time) error {
	return m.Called(ctx, userID, at).Error(0)
}

func (m *mockRepository) RetireFamilyTokensExcept(ctx context.Context, familyID, keepTokenHash string, now time.Time) error {
	return m.Called(ctx, familyID, keepTokenHash, now).Error(0)
}
