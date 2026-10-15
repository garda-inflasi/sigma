package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) ListSaya(ctx context.Context, pic string) ([]domain.TugasLapangan, error) {
	args := m.Called(ctx, pic)
	return args.Get(0).([]domain.TugasLapangan), args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (*domain.TugasLapangan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TugasLapangan), args.Error(1)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []int64, action domain.BulkAction) (int64, error) {
	args := m.Called(ctx, ids, action)
	return args.Get(0).(int64), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) *UseCase {
	return New(repo)
}

func sampleTugas() *domain.TugasLapangan {
	return &domain.TugasLapangan{
		ID:         1,
		Judul:      "Survei Harga Pasar",
		Lokasi:     "Pasar Induk",
		Tipe:       "SURVEI",
		Status:     "BARU",
		PIC:        "user-001",
		DibuatPada: time.Now(),
	}
}

// ── ListSaya Tests ────────────────────────────────────────────────────

func TestListSaya_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("ListSaya", mock.Anything, "user-001").Return([]domain.TugasLapangan{*sampleTugas()}, nil)

	result, err := uc.ListSaya(context.Background(), "user-001")

	require.NoError(t, err)
	assert.Len(t, result, 1)

	repo.AssertExpectations(t)
}

func TestListSaya_Empty(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("ListSaya", mock.Anything, "").Return([]domain.TugasLapangan{}, nil)

	result, err := uc.ListSaya(context.Background(), "")

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

// ── GetByID Tests ─────────────────────────────────────────────────────

func TestGetByID_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetByID", mock.Anything, int64(1)).Return(sampleTugas(), nil)

	result, err := uc.GetByID(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)

	repo.AssertExpectations(t)
}

func TestGetByID_NotFound(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetByID", mock.Anything, int64(999)).Return(nil, assert.AnError)

	result, err := uc.GetByID(context.Background(), 999)

	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

// ── BulkAction Tests ──────────────────────────────────────────────────

func TestBulkAction_UnduhDataAcuan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{1, 2}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkUnduhDataAcuan).Return(int64(2), nil)

	count, err := uc.BulkAction(context.Background(), ids, "unduh_data_acuan")

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_TandaiSelesai(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{1}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkTandaiSelesai).Return(int64(1), nil)

	count, err := uc.BulkAction(context.Background(), ids, "tandai_selesai")

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_InvalidAction(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(context.Background(), []int64{1}, "hapus")
	assert.ErrorIs(t, err, ErrInvalidAction)
}

func TestBulkAction_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{1}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkTandaiSelesai).Return(int64(0), assert.AnError)

	_, err := uc.BulkAction(context.Background(), ids, "tandai_selesai")

	assert.Error(t, err)

	repo.AssertExpectations(t)
}
