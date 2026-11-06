package usecase

import (
	"context"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// Metode domain.ResetMFARepository dan domain.RiwayatPeranRepository pada mockRepository (didefinisikan di sini
// agar usecase_test.go tidak membengkak).

func (m *mockRepository) ResetMFA(ctx context.Context, userID string, now time.Time) (*domain.HasilResetMFA, error) {
	args := m.Called(ctx, userID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.HasilResetMFA), args.Error(1)
}

func (m *mockRepository) RiwayatPeran(ctx context.Context, userID string, limit int) ([]domain.PerubahanPeran, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.PerubahanPeran), args.Error(1)
}
