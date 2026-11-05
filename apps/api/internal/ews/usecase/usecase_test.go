package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context, q domain.ListQuery) ([]domain.Alert, int64, error) {
	args := m.Called(ctx, q)
	return args.Get(0).([]domain.Alert), args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*domain.Alert, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Alert), args.Error(1)
}

func (m *mockRepository) ListAturan(ctx context.Context) ([]domain.Aturan, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Aturan), args.Error(1)
}

func (m *mockRepository) ListRiwayat(ctx context.Context, alertID string) ([]domain.Riwayat, error) {
	args := m.Called(ctx, alertID)
	return args.Get(0).([]domain.Riwayat), args.Error(1)
}

func (m *mockRepository) GetAturan(ctx context.Context, id string) (*domain.Aturan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Aturan), args.Error(1)
}

func (m *mockRepository) UpdateAturan(ctx context.Context, id string, upd domain.AturanUpdate, userID string) (*domain.Aturan, error) {
	args := m.Called(ctx, id, upd, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Aturan), args.Error(1)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction, userID string) ([]string, error) {
	args := m.Called(ctx, ids, action, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	recorder := &audittest.Recorder{}
	return New(repo, nil, recorder, logger.NewDefault()), recorder
}

func sampleAlert() domain.Alert {
	kode, nama := "BRS-MED", "Beras medium"
	return domain.Alert{
		EwsAlert: domain.EwsAlert{
			ID: "EWS-1000", Severity: "KRITIS", Jenis: "LONJAKAN_HARGA", Judul: "Lonjakan harga beras",
			KomoditasKode: &kode, KabupatenKode: "1171", Nilai: 21.4, SatuanNilai: "%", Status: domain.StatusBaru,
		},
		KomoditasNama: &nama, KabupatenNama: "KOTA BANDA ACEH",
	}
}

func TestList_AppliesDefaultsAndPassesFilters(t *testing.T) {
	cases := map[string]struct {
		in, want domain.ListQuery
	}{
		"kosong memakai bawaan":        {domain.ListQuery{}, domain.ListQuery{Page: 1, Limit: DefaultLimit}},
		"limit di atas batas":          {domain.ListQuery{Page: 3, Limit: 5000}, domain.ListQuery{Page: 3, Limit: DefaultLimit}},
		"filter diteruskan apa adanya": {domain.ListQuery{Status: "BARU", Severity: "KRITIS", Page: 2, Limit: 10}, domain.ListQuery{Status: "BARU", Severity: "KRITIS", Page: 2, Limit: 10}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := new(mockRepository)
			uc, _ := newTestUseCase(repo)
			repo.On("List", mock.Anything, tc.want).Return([]domain.Alert{sampleAlert()}, int64(42), nil)

			items, total, err := uc.List(context.Background(), tc.in)

			require.NoError(t, err)
			assert.Len(t, items, 1)
			assert.Equal(t, int64(42), total)
			repo.AssertExpectations(t)
		})
	}
}

func TestList_RepositoryError(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("List", mock.Anything, mock.Anything).Return([]domain.Alert{}, int64(0), assert.AnError)

	_, _, err := uc.List(context.Background(), domain.ListQuery{})

	assert.ErrorIs(t, err, assert.AnError)
}

func TestGetByID(t *testing.T) {
	t.Run("ditemukan", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		alert := sampleAlert()
		repo.On("GetByID", mock.Anything, "EWS-1000").Return(&alert, nil)

		got, err := uc.GetByID(context.Background(), "EWS-1000")

		require.NoError(t, err)
		assert.Equal(t, "EWS-1000", got.ID)
	})
	t.Run("tidak ada menjadi ErrNotFound", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "X").Return(nil, gorm.ErrRecordNotFound)

		_, err := uc.GetByID(context.Background(), "X")

		assert.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("galat lain diteruskan", func(t *testing.T) {
		repo := new(mockRepository)
		uc, _ := newTestUseCase(repo)
		repo.On("GetByID", mock.Anything, "X").Return(nil, assert.AnError)

		_, err := uc.GetByID(context.Background(), "X")

		assert.ErrorIs(t, err, assert.AnError)
		assert.NotErrorIs(t, err, ErrNotFound)
	})
}

func TestListAturan(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("ListAturan", mock.Anything).Return([]domain.Aturan{{AturanEws: domain.AturanEws{ID: "ATR-LH-BRS-MED"}}}, nil)

	items, err := uc.ListAturan(context.Background())

	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestBulkAction_ReportsChangedAndFailedAndWritesAudit(t *testing.T) {
	for _, action := range []string{"akui", "eskalasi", "selesai"} {
		t.Run(action, func(t *testing.T) {
			repo := new(mockRepository)
			uc, recorder := newTestUseCase(repo)
			// id ganda dibuang sebelum ke repository; "EWS-3" tidak berubah.
			repo.On("BulkAction", mock.Anything, []string{"EWS-1", "EWS-2", "EWS-3"}, domain.BulkAction(action), "user-001").
				Return([]string{"EWS-1", "EWS-2"}, nil)

			res, err := uc.BulkAction(context.Background(), []string{"EWS-1", "EWS-2", "EWS-1", "EWS-3"}, action, "user-001")

			require.NoError(t, err)
			assert.Equal(t, BulkResult{Berhasil: 2, Gagal: 1}, res)
			entry, ok := recorder.Last()
			require.True(t, ok, "aksi yang mengubah data harus dicatat di audit")
			assert.Equal(t, "ews_alert", entry.Entitas)
			assert.Equal(t, "user-001", entry.AktorID)
			assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
			assert.Equal(t, []string{"EWS-1", "EWS-2"}, entry.Metadata["ids"])
		})
	}
}

func TestBulkAction_NothingChangedWritesNoAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("BulkAction", mock.Anything, []string{"EWS-9"}, domain.BulkAkui, "u").Return([]string{}, nil)

	res, err := uc.BulkAction(context.Background(), []string{"EWS-9"}, "akui", "u")

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Gagal: 1}, res)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_InvalidAction(t *testing.T) {
	uc, recorder := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(context.Background(), []string{"EWS-1"}, "hapus", "u")

	assert.ErrorIs(t, err, ErrInvalidAction)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_RepositoryError(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("BulkAction", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	_, err := uc.BulkAction(context.Background(), []string{"EWS-1"}, "akui", "u")

	assert.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_AuditFailureDoesNotFailTheAction(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	recorder.Err = errors.New("audit down")
	repo.On("BulkAction", mock.Anything, []string{"EWS-1"}, domain.BulkAkui, "u").Return([]string{"EWS-1"}, nil)

	res, err := uc.BulkAction(context.Background(), []string{"EWS-1"}, "akui", "u")

	require.NoError(t, err)
	assert.Equal(t, 1, res.Berhasil)
}
