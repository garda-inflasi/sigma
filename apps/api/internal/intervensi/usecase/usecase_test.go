package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context) ([]domain.KartuIntervensi, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.KartuIntervensi), args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (*domain.KartuIntervensi, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.KartuIntervensi), args.Error(1)
}

func (m *mockRepository) Create(ctx context.Context, kartu *domain.KartuIntervensi) error {
	args := m.Called(ctx, kartu)
	return args.Error(0)
}

func (m *mockRepository) UpdateTahap(ctx context.Context, id int64, tahap domain.TahapEnum, urutan int) error {
	args := m.Called(ctx, id, tahap, urutan)
	return args.Error(0)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []int64, action domain.BulkAction) (int64, error) {
	args := m.Called(ctx, ids, action)
	return args.Get(0).(int64), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) *UseCase {
	return New(repo, logger.NewDefault())
}

func sampleKartu() *domain.KartuIntervensi {
	return &domain.KartuIntervensi{
		ID:            1,
		Judul:         "Operasi Pasar Beras",
		Deskripsi:     "Operasi pasar untuk menstabilkan harga beras",
		Tahap:         domain.TahapUsulan,
		Urutan:        0,
		Prioritas:     "TINGGI",
		PIC:           "user-001",
		KomoditasKode: "BERAS",
		WilayahKode:   "1171",
		DibuatOlehID:  "user-001",
	}
}

// ── List Tests ────────────────────────────────────────────────────────

func TestList_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	expected := []domain.KartuIntervensi{*sampleKartu()}
	repo.On("List", mock.Anything).Return(expected, nil)

	result, err := uc.List(context.Background())

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Operasi Pasar Beras", result[0].Judul)

	repo.AssertExpectations(t)
}

func TestList_Empty(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("List", mock.Anything).Return([]domain.KartuIntervensi{}, nil)

	result, err := uc.List(context.Background())

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

// ── Create Tests ──────────────────────────────────────────────────────

func TestCreate_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	kartu := sampleKartu()
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.KartuIntervensi")).Return(nil)

	err := uc.Create(context.Background(), kartu)

	require.NoError(t, err)
	assert.Equal(t, domain.TahapUsulan, kartu.Tahap)

	repo.AssertExpectations(t)
}

// ── UpdateTahap Tests ─────────────────────────────────────────────────

func TestUpdateTahap_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	kartu := sampleKartu()
	repo.On("GetByID", mock.Anything, int64(1)).Return(kartu, nil)
	repo.On("UpdateTahap", mock.Anything, int64(1), domain.TahapKaji, 1).Return(nil)

	err := uc.UpdateTahap(context.Background(), 1, "kaji", 1)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestUpdateTahap_NotFound(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetByID", mock.Anything, int64(999)).Return(nil, assert.AnError)

	err := uc.UpdateTahap(context.Background(), 999, "kaji", 1)
	assert.ErrorIs(t, err, ErrNotFound)

	repo.AssertExpectations(t)
}

func TestUpdateTahap_InvalidTahap(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	err := uc.UpdateTahap(context.Background(), 1, "invalid_tahap", 0)
	assert.ErrorIs(t, err, ErrInvalidTahap)
}

func TestUpdateTahap_AllValidTahaps(t *testing.T) {
	testCases := []string{"usulan", "kaji", "setuju", "laksana", "selesai", "batal"}

	for _, tc := range testCases {
		repo := new(mockRepository)
		uc := newTestUseCase(repo)

		kartu := sampleKartu()
		repo.On("GetByID", mock.Anything, int64(1)).Return(kartu, nil)
		repo.On("UpdateTahap", mock.Anything, int64(1), domain.TahapEnum(tc), 1).Return(nil)

		err := uc.UpdateTahap(context.Background(), 1, tc, 1)
		assert.NoError(t, err, "tahap %s should be valid", tc)

		repo.AssertExpectations(t)
	}
}

// ── BulkAction Tests ──────────────────────────────────────────────────

func TestBulkAction_Arsip(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{1, 2}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkArsip).Return(int64(2), nil)

	count, err := uc.BulkAction(context.Background(), ids, "arsip")

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_Hapus(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []int64{3}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkHapus).Return(int64(1), nil)

	count, err := uc.BulkAction(context.Background(), ids, "hapus")

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_InvalidAction(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(context.Background(), []int64{1}, "unknown")
	assert.ErrorIs(t, err, ErrInvalidAction)
}
