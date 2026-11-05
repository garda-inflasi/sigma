package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context) ([]domain.SumberData, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.SumberData), args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*domain.SumberData, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SumberData), args.Error(1)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction) ([]string, error) {
	args := m.Called(ctx, ids, action)
	return args.Get(0).([]string), args.Error(1)
}

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	recorder := &audittest.Recorder{}
	return New(repo, recorder, logger.NewDefault()), recorder
}

func TestList_Success(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("List", mock.Anything).Return([]domain.SumberData{{ID: "SRC-BPS", Nama: "BPS"}}, nil)

	result, err := uc.List(context.Background())

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "SRC-BPS", result[0].ID)
}

func TestList_Error(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("List", mock.Anything).Return([]domain.SumberData(nil), errors.New("db down"))

	_, err := uc.List(context.Background())

	assert.Error(t, err)
}

func TestGetByID_PassesNotFoundThrough(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("GetByID", mock.Anything, "SRC-X").Return(nil, domain.ErrNotFound)

	_, err := uc.GetByID(context.Background(), "SRC-X")

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetByID_DatabaseErrorIsNotReportedAsNotFound(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("GetByID", mock.Anything, "SRC-BPS").Return(nil, errors.New("db down"))

	_, err := uc.GetByID(context.Background(), "SRC-BPS")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestBulkAction_RejectsUnknownActions(t *testing.T) {
	// "jalankan" belum didukung: tidak ada penjadwal (05-B), jadi tidak boleh pura-pura berhasil.
	for _, action := range []string{"jalankan", "hapus", ""} {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)

		_, err := uc.BulkAction(context.Background(), []string{"SRC-BPS"}, action, "u1")

		assert.ErrorIs(t, err, ErrInvalidAction, action)
		repo.AssertNotCalled(t, "BulkAction")
		assert.Empty(t, recorder.Entries())
	}
}

func TestBulkAction_CountsFailuresAndWritesAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	// Id ganda dihitung sekali dan diurutkan; hanya satu dari dua yang berubah.
	repo.On("BulkAction", mock.Anything, []string{"SRC-BI", "SRC-BPS"}, domain.BulkJeda).Return([]string{"SRC-BPS"}, nil)

	res, err := uc.BulkAction(context.Background(), []string{"SRC-BPS", "SRC-BI", "SRC-BPS"}, "jeda", "u1")

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 1, Gagal: 1}, res)
	entry, ok := recorder.Last()
	require.True(t, ok, "aksi yang mengubah data harus diaudit")
	assert.Equal(t, "BULK_jeda", entry.Aksi)
	assert.Equal(t, "sumber_data", entry.Entitas)
	assert.Equal(t, "u1", entry.AktorID)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
	repo.AssertExpectations(t)
}

func TestBulkAction_RepositoryErrorWritesNoAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("BulkAction", mock.Anything, []string{"SRC-BPS"}, domain.BulkLanjutkan).Return([]string(nil), errors.New("db down"))

	_, err := uc.BulkAction(context.Background(), []string{"SRC-BPS"}, "lanjutkan", "u1")

	require.Error(t, err)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_AuditFailureDoesNotFailTheAction(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	recorder.Err = errors.New("audit down")
	repo.On("BulkAction", mock.Anything, []string{"SRC-BPS"}, domain.BulkJeda).Return([]string{"SRC-BPS"}, nil)

	res, err := uc.BulkAction(context.Background(), []string{"SRC-BPS"}, "jeda", "u1")

	require.NoError(t, err)
	assert.Equal(t, 1, res.Berhasil)
}
