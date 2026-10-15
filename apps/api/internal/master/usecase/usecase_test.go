package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) ListKomoditas(ctx context.Context) ([]domain.Komoditas, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Komoditas), args.Error(1)
}

func (m *mockRepository) ListWilayah(ctx context.Context, provinsiKode string) ([]domain.Kabupaten, error) {
	args := m.Called(ctx, provinsiKode)
	return args.Get(0).([]domain.Kabupaten), args.Error(1)
}

func (m *mockRepository) ListPasar(ctx context.Context, kabupatenKode string) ([]domain.Pasar, error) {
	args := m.Called(ctx, kabupatenKode)
	return args.Get(0).([]domain.Pasar), args.Error(1)
}

func (m *mockRepository) BulkKomoditas(ctx context.Context, kode []string, action domain.BulkAction) (int64, error) {
	args := m.Called(ctx, kode, action)
	return args.Get(0).(int64), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) *UseCase {
	return New(repo)
}

// ── ListKomoditas Tests ───────────────────────────────────────────────

func TestListKomoditas_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	expected := []domain.Komoditas{{Kode: "BERAS", Nama: "Beras", Kategori: "PANGAN", Satuan: "kg", Status: "AKTIF"}}
	repo.On("ListKomoditas", mock.Anything).Return(expected, nil)

	result, err := uc.ListKomoditas(context.Background())

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "BERAS", result[0].Kode)

	repo.AssertExpectations(t)
}

// ── ListWilayah Tests ─────────────────────────────────────────────────

func TestListWilayah_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	expected := []domain.Kabupaten{{Kode: "1171", Nama: "KOTA BANDA ACEH", ProvinsiKode: "11"}}
	repo.On("ListWilayah", mock.Anything, "11").Return(expected, nil)

	result, err := uc.ListWilayah(context.Background(), "11")

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "1171", result[0].Kode)

	repo.AssertExpectations(t)
}

func TestListWilayah_Empty(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("ListWilayah", mock.Anything, "").Return([]domain.Kabupaten{}, nil)

	result, err := uc.ListWilayah(context.Background(), "")

	require.NoError(t, err)
	assert.Empty(t, result)

	repo.AssertExpectations(t)
}

// ── ListPasar Tests ───────────────────────────────────────────────────

func TestListPasar_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	expected := []domain.Pasar{{ID: "P1", Nama: "Pasar Induk", KabupatenKode: "1171", Aktif: true}}
	repo.On("ListPasar", mock.Anything, "1171").Return(expected, nil)

	result, err := uc.ListPasar(context.Background(), "1171")

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "P1", result[0].ID)

	repo.AssertExpectations(t)
}

// ── BulkKomoditas Tests ───────────────────────────────────────────────

func TestBulkKomoditas_AjukanPerubahan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	kode := []string{"BERAS", "CABAI"}
	repo.On("BulkKomoditas", mock.Anything, kode, domain.BulkAjukanPerubahan).Return(int64(2), nil)

	count, err := uc.BulkKomoditas(context.Background(), kode, "ajukan_perubahan")

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	repo.AssertExpectations(t)
}

func TestBulkKomoditas_Setujui(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	kode := []string{"BERAS"}
	repo.On("BulkKomoditas", mock.Anything, kode, domain.BulkSetujui).Return(int64(1), nil)

	count, err := uc.BulkKomoditas(context.Background(), kode, "setujui")

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	repo.AssertExpectations(t)
}

func TestBulkKomoditas_Arsipkan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	kode := []string{"BERAS"}
	repo.On("BulkKomoditas", mock.Anything, kode, domain.BulkArsipkan).Return(int64(1), nil)

	count, err := uc.BulkKomoditas(context.Background(), kode, "arsipkan")

	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	repo.AssertExpectations(t)
}

func TestBulkKomoditas_InvalidAction(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	_, err := uc.BulkKomoditas(context.Background(), []string{"BERAS"}, "hapus")
	assert.ErrorIs(t, err, ErrInvalidAction)
}

func TestBulkKomoditas_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	kode := []string{"BERAS"}
	repo.On("BulkKomoditas", mock.Anything, kode, domain.BulkArsipkan).Return(int64(0), assert.AnError)

	_, err := uc.BulkKomoditas(context.Background(), kode, "arsipkan")

	assert.Error(t, err)

	repo.AssertExpectations(t)
}
