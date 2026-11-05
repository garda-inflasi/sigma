package usecase

import (
	"context"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) ListKomoditas(ctx context.Context, q domain.ListQuery) ([]domain.Komoditas, int64, error) {
	args := m.Called(ctx, q)
	return args.Get(0).([]domain.Komoditas), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) ListWilayah(ctx context.Context, provinsiKode string, q domain.ListQuery) ([]domain.Kabupaten, int64, error) {
	args := m.Called(ctx, provinsiKode, q)
	return args.Get(0).([]domain.Kabupaten), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) ListPasar(ctx context.Context, kabupatenKode string, q domain.ListQuery) ([]domain.Pasar, int64, error) {
	args := m.Called(ctx, kabupatenKode, q)
	return args.Get(0).([]domain.Pasar), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) BulkKomoditas(ctx context.Context, kode []string, action domain.BulkAction) (int64, error) {
	args := m.Called(ctx, kode, action)
	return args.Get(0).(int64), args.Error(1)
}

// ── Test Helpers ──────────────────────────────────────────────────────

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	recorder := &audittest.Recorder{}
	return New(repo, recorder, logger.NewDefault()), recorder
}

// ── List Tests ────────────────────────────────────────────────────────

func TestListKomoditas_Success(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	expected := []domain.Komoditas{{Kode: "BRS-MED", Nama: "Beras medium", Kategori: "Pangan pokok", Satuan: "kg", Status: "AKTIF"}}
	query := domain.ListQuery{Search: "beras", Page: 2, Limit: 5}
	repo.On("ListKomoditas", mock.Anything, query).Return(expected, int64(11), nil)

	result, total, err := uc.ListKomoditas(context.Background(), query)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "BRS-MED", result[0].Kode)
	assert.EqualValues(t, 11, total)
	repo.AssertExpectations(t)
}

func TestListWilayah_Success(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	expected := []domain.Kabupaten{{Kode: "1171", Nama: "KOTA BANDA ACEH", ProvinsiKode: "11"}}
	repo.On("ListWilayah", mock.Anything, "11", domain.ListQuery{}).Return(expected, int64(1), nil)

	result, total, err := uc.ListWilayah(context.Background(), "11", domain.ListQuery{})

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "1171", result[0].Kode)
	assert.EqualValues(t, 1, total)
	repo.AssertExpectations(t)
}

func TestListWilayah_Empty(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("ListWilayah", mock.Anything, "", domain.ListQuery{}).Return([]domain.Kabupaten{}, int64(0), nil)

	result, _, err := uc.ListWilayah(context.Background(), "", domain.ListQuery{})

	require.NoError(t, err)
	assert.Empty(t, result)
	repo.AssertExpectations(t)
}

func TestListPasar_Success(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	expected := []domain.Pasar{{ID: "P1", Nama: "Pasar Induk", KabupatenKode: "1171", Aktif: true}}
	repo.On("ListPasar", mock.Anything, "1171", domain.ListQuery{}).Return(expected, int64(1), nil)

	result, _, err := uc.ListPasar(context.Background(), "1171", domain.ListQuery{})

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "P1", result[0].ID)
	repo.AssertExpectations(t)
}

// ── BulkKomoditas Tests ───────────────────────────────────────────────

func TestBulkKomoditas_Actions(t *testing.T) {
	cases := map[string]domain.BulkAction{
		"ajukan_perubahan": domain.BulkAjukanPerubahan,
		"setujui":          domain.BulkSetujui,
		"arsipkan":         domain.BulkArsipkan,
	}
	for action, domainAction := range cases {
		t.Run(action, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCase(repo)
			kode := []string{"BRS-MED", "CBM-KRT"}
			repo.On("BulkKomoditas", mock.Anything, kode, domainAction).Return(int64(2), nil)

			count, err := uc.BulkKomoditas(context.Background(), kode, action, "admin-1")

			require.NoError(t, err)
			assert.EqualValues(t, 2, count)
			entry, ok := recorder.Last()
			require.True(t, ok, "aksi yang mengubah data tercatat di audit")
			assert.Equal(t, "BULK_"+action, entry.Aksi)
			assert.Equal(t, "admin-1", entry.AktorID)
			assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
			assert.Equal(t, kode, entry.Metadata["kode"])
			repo.AssertExpectations(t)
		})
	}
}

func TestBulkKomoditas_InvalidAction(t *testing.T) {
	uc, recorder := newTestUseCase(new(mockRepository))

	_, err := uc.BulkKomoditas(context.Background(), []string{"BRS-MED"}, "hapus", "admin-1")

	assert.ErrorIs(t, err, ErrInvalidAction)
	assert.Empty(t, recorder.Entries())
}

func TestBulkKomoditas_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	kode := []string{"BRS-MED"}
	repo.On("BulkKomoditas", mock.Anything, kode, domain.BulkArsipkan).Return(int64(0), assert.AnError)

	_, err := uc.BulkKomoditas(context.Background(), kode, "arsipkan", "admin-1")

	assert.Error(t, err)
	assert.Empty(t, recorder.Entries(), "aksi yang gagal tidak dicatat sebagai berhasil")
	repo.AssertExpectations(t)
}

func TestBulkKomoditas_AuditFailureDoesNotFailAction(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	recorder.Err = assert.AnError
	kode := []string{"BRS-MED"}
	repo.On("BulkKomoditas", mock.Anything, kode, domain.BulkSetujui).Return(int64(1), nil)

	count, err := uc.BulkKomoditas(context.Background(), kode, "setujui", "admin-1")

	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}
