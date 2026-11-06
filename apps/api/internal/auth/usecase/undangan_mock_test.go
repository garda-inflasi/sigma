package usecase

import (
	"context"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

// Metode domain.UndanganRepository dan domain.EksporRepository pada mockRepository (didefinisikan di sini agar
// usecase_test.go tidak berubah). Tes undangan memakai memUndangan (undangan_mem_test.go) yang berjalan di atas
// penyimpanan dalam memori; stub ini hanya membuat mockRepository memenuhi domain.Repository.

func (m *mockRepository) BuatUndangan(ctx context.Context, baru []domain.UndanganBaru) error {
	return m.Called(ctx, baru).Error(0)
}

func (m *mockRepository) GantiUndangan(ctx context.Context, userID string, u *domain.Undangan, now time.Time) (*domain.Pengguna, error) {
	args := m.Called(ctx, userID, u, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Pengguna), args.Error(1)
}

func (m *mockRepository) GetUndangan(ctx context.Context, tokenHash string) (*domain.Undangan, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Undangan), args.Error(1)
}

func (m *mockRepository) Aktifkan(ctx context.Context, tokenHash, passwordHash string, now time.Time) (*domain.Pengguna, error) {
	args := m.Called(ctx, tokenHash, passwordHash, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Pengguna), args.Error(1)
}

func (m *mockRepository) EmailTerdaftar(ctx context.Context, emailKecil []string) (map[string]bool, error) {
	args := m.Called(ctx, emailKecil)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *mockRepository) KabupatenAda(ctx context.Context, kode []string) (map[string]bool, error) {
	args := m.Called(ctx, kode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *mockRepository) ListUsersUntukEkspor(ctx context.Context, filter domain.FilterPengguna, _ time.Time, batas int) ([]domain.Pengguna, bool, error) {
	args := m.Called(ctx, filter, batas)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).([]domain.Pengguna), args.Bool(1), args.Error(2)
}
