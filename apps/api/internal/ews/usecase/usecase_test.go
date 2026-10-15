package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context, limit int) ([]domain.EwsAlert, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]domain.EwsAlert), args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (*domain.EwsAlert, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EwsAlert), args.Error(1)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []int64, action domain.BulkAction, userID string) (int64, error) {
	args := m.Called(ctx, ids, action, userID)
	return args.Get(0).(int64), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) *UseCase {
	return New(repo, logger.NewDefault())
}

func sampleAlert() *domain.EwsAlert {
	return &domain.EwsAlert{
		ID:            1,
		KomoditasKode: "BERAS",
		WilayahKode:   "1171",
		Level:         domain.LevelKuning,
		Skor:          75.5,
		Deskripsi:     "Kenaikan harga beras signifikan",
		Status:        "BARU",
		Sumber:        "SISTEM",
		SkorDampak:    3,
	}
}

// ── List Tests ────────────────────────────────────────────────────────

func TestList_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	expected := []domain.EwsAlert{*sampleAlert()}
	repo.On("List", mock.Anything, 200).Return(expected, nil)

	result, err := uc.List(context.Background(), 200)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), result[0].ID)

	repo.AssertExpectations(t)
}

func TestList_Empty(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("List", mock.Anything, 200).Return([]domain.EwsAlert{}, nil)

	result, err := uc.List(context.Background(), 200)

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

func TestList_DefaultLimit(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("List", mock.Anything, 200).Return([]domain.EwsAlert{}, nil)

	result, err := uc.List(context.Background(), 0)

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

func TestList_ExceedsMaxLimit(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("List", mock.Anything, 200).Return([]domain.EwsAlert{}, nil)

	result, err := uc.List(context.Background(), 5000)

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

// ── GetByID Tests ─────────────────────────────────────────────────────

func TestGetByID_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetByID", mock.Anything, int64(1)).Return(sampleAlert(), nil)

	result, err := uc.GetByID(context.Background(), 1)

	require.NoError(t, err)
	assert.NotNil(t, result)
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

func TestBulkAction_Akui(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{1, 2, 3}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkAkui, "user-001").Return(int64(3), nil)

	count, err := uc.BulkAction(context.Background(), ids, "akui", "user-001")

	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_Eskalasi(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{1}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkEskalasi, "user-001").Return(int64(1), nil)

	count, err := uc.BulkAction(context.Background(), ids, "eskalasi", "user-001")

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_Selesai(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{2, 5}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkSelesai, "user-001").Return(int64(2), nil)

	count, err := uc.BulkAction(context.Background(), ids, "selesai", "user-001")

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_InvalidAction(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(context.Background(), []int64{1}, "hapus", "user-001")
	assert.ErrorIs(t, err, ErrInvalidAction)
}

func TestBulkAction_EmptyIDs(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkAkui, "user-001").Return(int64(0), nil)

	count, err := uc.BulkAction(context.Background(), ids, "akui", "user-001")

	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	repo.AssertExpectations(t)
}
