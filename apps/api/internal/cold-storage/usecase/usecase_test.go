package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) GetAllFasilitas(ctx context.Context) ([]domain.FasilitasColdStorage, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.FasilitasColdStorage), args.Error(1)
}

func (m *mockRepository) GetAllStok(ctx context.Context) ([]domain.StokColdStorage, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.StokColdStorage), args.Error(1)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction) (int64, error) {
	args := m.Called(ctx, ids, action)
	return args.Get(0).(int64), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) *UseCase {
	return New(repo, logger.NewDefault())
}

// ── GetAll Tests ──────────────────────────────────────────────────────

func TestGetAll_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	fasilitas := []domain.FasilitasColdStorage{{
		ID: "CS1", Nama: "Cold Storage A", KabupatenKode: "1171", KapasitasTon: 100, Status: "AKTIF",
	}}
	stok := []domain.StokColdStorage{{
		ID: "S1", ColdStorageID: "CS1", KomoditasKode: "BERAS", VolumeTon: 10, MasukPada: time.Now(),
	}}

	repo.On("GetAllFasilitas", mock.Anything).Return(fasilitas, nil)
	repo.On("GetAllStok", mock.Anything).Return(stok, nil)

	f, s, err := uc.GetAll(context.Background())

	require.NoError(t, err)
	assert.Len(t, f, 1)
	assert.Len(t, s, 1)

	repo.AssertExpectations(t)
}

func TestGetAll_FasilitasError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetAllFasilitas", mock.Anything).Return([]domain.FasilitasColdStorage{}, assert.AnError)

	_, _, err := uc.GetAll(context.Background())

	assert.Error(t, err)

	repo.AssertExpectations(t)
}

func TestGetAll_StokError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetAllFasilitas", mock.Anything).Return([]domain.FasilitasColdStorage{}, nil)
	repo.On("GetAllStok", mock.Anything).Return([]domain.StokColdStorage{}, assert.AnError)

	_, _, err := uc.GetAll(context.Background())

	assert.Error(t, err)

	repo.AssertExpectations(t)
}

// ── BulkAction Tests ──────────────────────────────────────────────────

func TestBulkAction_RencanakanDistribusi(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{"S1", "S2"}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkRencanakanDistribusi).Return(int64(2), nil)

	count, err := uc.BulkAction(context.Background(), ids, "rencanakan_distribusi")

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_RekomendasiOlahan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{"S1"}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkRekomendasiOlahan).Return(int64(1), nil)

	count, err := uc.BulkAction(context.Background(), ids, "rekomendasi_olahan")

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_InvalidAction(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(context.Background(), []string{"S1"}, "hapus")
	assert.ErrorIs(t, err, ErrInvalidAction)
}

func TestBulkAction_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{"S1"}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkRekomendasiOlahan).Return(int64(0), assert.AnError)

	_, err := uc.BulkAction(context.Background(), ids, "rekomendasi_olahan")

	assert.Error(t, err)

	repo.AssertExpectations(t)
}
