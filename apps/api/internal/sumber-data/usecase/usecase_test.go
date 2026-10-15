package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context) ([]domain.SumberData, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.SumberData), args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (*domain.SumberData, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SumberData), args.Error(1)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []int64, action domain.BulkAction) (int64, error) {
	args := m.Called(ctx, ids, action)
	return args.Get(0).(int64), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) *UseCase {
	return New(repo)
}

func sampleSumberData() *domain.SumberData {
	return &domain.SumberData{
		ID:              1,
		Nama:            "Pasar Induk",
		Tipe:            "API",
		URL:             "https://example.com/api",
		Status:          "AKTIF",
		FrekuensiUpdate: "HARIAN",
		DibuatPada:      time.Now(),
	}
}

// ── List Tests ────────────────────────────────────────────────────────

func TestList_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("List", mock.Anything).Return([]domain.SumberData{*sampleSumberData()}, nil)

	result, err := uc.List(context.Background())

	require.NoError(t, err)
	assert.Len(t, result, 1)

	repo.AssertExpectations(t)
}

func TestList_Empty(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("List", mock.Anything).Return([]domain.SumberData{}, nil)

	result, err := uc.List(context.Background())

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

// ── GetByID Tests ─────────────────────────────────────────────────────

func TestGetByID_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetByID", mock.Anything, int64(1)).Return(sampleSumberData(), nil)

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

func TestBulkAction_Jalankan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{1, 2, 3}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkJalankan).Return(int64(3), nil)

	count, err := uc.BulkAction(context.Background(), ids, "jalankan")

	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_Jeda(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{1}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkJeda).Return(int64(1), nil)

	count, err := uc.BulkAction(context.Background(), ids, "jeda")

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
	repo.On("BulkAction", mock.Anything, ids, domain.BulkLanjutkan).Return(int64(0), assert.AnError)

	_, err := uc.BulkAction(context.Background(), ids, "lanjutkan")

	assert.Error(t, err)

	repo.AssertExpectations(t)
}
