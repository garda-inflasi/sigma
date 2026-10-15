package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context, limit int, status string) ([]domain.EntriHarga, error) {
	args := m.Called(ctx, limit, status)
	return args.Get(0).([]domain.EntriHarga), args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*domain.EntriHarga, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EntriHarga), args.Error(1)
}

func (m *mockRepository) GetTren(ctx context.Context, komoditas, wilayah string) ([]domain.TrenHarga, error) {
	args := m.Called(ctx, komoditas, wilayah)
	return args.Get(0).([]domain.TrenHarga), args.Error(1)
}

func (m *mockRepository) Create(ctx context.Context, entri *domain.EntriHarga) error {
	args := m.Called(ctx, entri)
	return args.Error(0)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction, userID string) (int64, error) {
	args := m.Called(ctx, ids, action, userID)
	return args.Get(0).(int64), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) *UseCase {
	return New(repo, logger.NewDefault())
}

func sampleEntri() *domain.EntriHarga {
	return &domain.EntriHarga{
		ID:            "harga-001",
		KomoditasKode: "BERAS",
		PasarID:       "pasar-001",
		Harga:         12500,
		Satuan:        "kg",
		Status:        "DIAJUKAN",
		Periode:       "2024-01",
		Sumber:        "LAPANGAN",
	}
}

// ── ListHarga Tests ───────────────────────────────────────────────────

func TestListHarga_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	expected := []domain.EntriHarga{*sampleEntri()}
	repo.On("List", mock.Anything, 500, "").Return(expected, nil)

	result, err := uc.ListHarga(context.Background(), 500, "")

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "harga-001", result[0].ID)

	repo.AssertExpectations(t)
}

func TestListHarga_Empty(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("List", mock.Anything, 100, "DIAJUKAN").Return([]domain.EntriHarga{}, nil)

	result, err := uc.ListHarga(context.Background(), 100, "DIAJUKAN")

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

func TestListHarga_DefaultLimit(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("List", mock.Anything, 500, "").Return([]domain.EntriHarga{}, nil)

	result, err := uc.ListHarga(context.Background(), 0, "")

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

// ── GetTren Tests ─────────────────────────────────────────────────────

func TestGetTren_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	trenData := []domain.TrenHarga{
		{Periode: "2024-01", HargaRerata: 12000, HargaMin: 11500, HargaMax: 12500, JumlahEntri: 10},
	}
	repo.On("GetTren", mock.Anything, "BERAS", "1171").Return(trenData, nil)

	result, err := uc.GetTren(context.Background(), "BERAS", "1171")

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, float64(12000), result[0].HargaRerata)

	repo.AssertExpectations(t)
}

func TestGetTren_EmptyParams(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	_, err := uc.GetTren(context.Background(), "", "1171")
	assert.Error(t, err)

	_, err = uc.GetTren(context.Background(), "BERAS", "")
	assert.Error(t, err)
}

func TestGetTren_NoData(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetTren", mock.Anything, "GULA", "1171").Return([]domain.TrenHarga{}, nil)

	result, err := uc.GetTren(context.Background(), "GULA", "1171")

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

// ── CreateHarga Tests ─────────────────────────────────────────────────

func TestCreateHarga_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	entri := sampleEntri()
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.EntriHarga")).Return(nil)

	err := uc.CreateHarga(context.Background(), entri)

	require.NoError(t, err)
	assert.Equal(t, "DIAJUKAN", entri.Status)
	assert.NotEmpty(t, entri.ID)

	repo.AssertExpectations(t)
}

func TestCreateHarga_WithExistingID(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	entri := &domain.EntriHarga{
		ID:            "custom-id",
		KomoditasKode: "MINYAK",
		Harga:         20000,
		Satuan:        "liter",
		Periode:       "2024-02",
	}
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.EntriHarga")).Return(nil)

	err := uc.CreateHarga(context.Background(), entri)

	require.NoError(t, err)
	assert.Equal(t, "custom-id", entri.ID)
	assert.Equal(t, "DIAJUKAN", entri.Status)

	repo.AssertExpectations(t)
}

// ── GetHargaByID Tests ────────────────────────────────────────────────

func TestGetHargaByID_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetByID", mock.Anything, "harga-001").Return(sampleEntri(), nil)

	result, err := uc.GetHargaByID(context.Background(), "harga-001")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "harga-001", result.ID)

	repo.AssertExpectations(t)
}

func TestGetHargaByID_NotFound(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("GetByID", mock.Anything, "nonexistent").Return(nil, assert.AnError)

	result, err := uc.GetHargaByID(context.Background(), "nonexistent")

	assert.ErrorIs(t, err, ErrNotFound)
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

// ── BulkAction Tests ──────────────────────────────────────────────────

func TestBulkAction_Validasi(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{"harga-001", "harga-002"}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkValidasi, "user-001").Return(int64(2), nil)

	count, err := uc.BulkAction(context.Background(), ids, "validasi", "user-001")

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_Publikasi(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{"harga-001"}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkPublikasi, "user-001").Return(int64(1), nil)

	count, err := uc.BulkAction(context.Background(), ids, "publikasi", "user-001")

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_Tolak(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{"harga-003"}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkTolak, "user-001").Return(int64(1), nil)

	count, err := uc.BulkAction(context.Background(), ids, "tolak", "user-001")

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	repo.AssertExpectations(t)
}

func TestBulkAction_InvalidAction(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(context.Background(), []string{"harga-001"}, "hapus", "user-001")
	assert.ErrorIs(t, err, ErrInvalidAction)
}

func TestBulkAction_EmptyIDs(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkValidasi, "user-001").Return(int64(0), nil)

	count, err := uc.BulkAction(context.Background(), ids, "validasi", "user-001")

	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	repo.AssertExpectations(t)
}
