package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) ListSaya(ctx context.Context, petugasID string) ([]domain.TugasView, error) {
	args := m.Called(ctx, petugasID)
	items, _ := args.Get(0).([]domain.TugasView)
	return items, args.Error(1)
}

func (m *mockRepository) GetByID(ctx context.Context, id, petugasID string, semua bool) (*domain.TugasView, error) {
	args := m.Called(ctx, id, petugasID, semua)
	item, _ := args.Get(0).(*domain.TugasView)
	return item, args.Error(1)
}

func (m *mockRepository) Aksesibel(ctx context.Context, ids []string, petugasID string, semua bool) ([]string, error) {
	args := m.Called(ctx, ids, petugasID, semua)
	found, _ := args.Get(0).([]string)
	return found, args.Error(1)
}

func (m *mockRepository) TandaiSelesai(ctx context.Context, ids []string, petugasID string, semua bool, waktu time.Time) ([]string, error) {
	args := m.Called(ctx, ids, petugasID, semua, waktu)
	changed, _ := args.Get(0).([]string)
	return changed, args.Error(1)
}

var fixedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	recorder := &audittest.Recorder{}
	uc := New(repo, recorder, logger.NewDefault())
	uc.now = func() time.Time { return fixedNow }
	return uc, recorder
}

func tugas(id, status string, tenggat time.Duration, selesai int) domain.TugasView {
	return domain.TugasView{ID: id, Status: status, Tenggat: fixedNow.Add(tenggat), Target: 4, JumlahSelesai: selesai}
}

func TestListSaya_ComputesDisplayedStatus(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("ListSaya", mock.Anything, "mahasiswa_1").Return([]domain.TugasView{
		tugas("belum", domain.StatusBelum, 2*time.Hour, 0),
		tugas("mulai", domain.StatusBelum, 2*time.Hour, 1),
		tugas("lewat", domain.StatusBerjalan, -time.Hour, 1),
		tugas("selesai", domain.StatusSelesai, -time.Hour, 4),
	}, nil)

	got, err := uc.ListSaya(t.Context(), "mahasiswa_1")

	require.NoError(t, err)
	status := map[string]string{}
	for _, item := range got {
		status[item.ID] = item.StatusTampil
	}
	assert.Equal(t, map[string]string{
		"belum": domain.StatusBelum, "mulai": domain.StatusBerjalan, "lewat": domain.StatusTerlambat, "selesai": domain.StatusSelesai,
	}, status)
}

func TestListSaya_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("ListSaya", mock.Anything, "u").Return(nil, errors.New("db mati"))

	_, err := uc.ListSaya(t.Context(), "u")

	require.Error(t, err)
}

func TestGetByID(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	item := tugas("t1", domain.StatusBelum, time.Hour, 0)
	repo.On("GetByID", mock.Anything, "t1", "u", false).Return(&item, nil)
	repo.On("GetByID", mock.Anything, "milik-lain", "u", false).Return(nil, domain.ErrNotFound)

	got, err := uc.GetByID(t.Context(), "t1", Pemanggil{ID: "u"})
	require.NoError(t, err)
	assert.Equal(t, "t1", got.ID)

	_, err = uc.GetByID(t.Context(), "milik-lain", Pemanggil{ID: "u"})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestBulkAction_TandaiSelesaiAudits(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("TandaiSelesai", mock.Anything, []string{"a", "b"}, "u", false, fixedNow).Return([]string{"a"}, nil)

	hasil, err := uc.BulkAction(t.Context(), []string{"a", "b", "a"}, "tandai_selesai", Pemanggil{ID: "u"})

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Berhasil: 1, Gagal: 1}, hasil)
	entry, ok := recorder.Last()
	require.True(t, ok, "aksi yang mengubah data tercatat di audit")
	assert.Equal(t, "TUGAS_TANDAI_SELESAI", entry.Aksi)
	assert.Equal(t, "u", entry.AktorID)
}

func TestBulkAction_UnduhIsReadOnly(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("Aksesibel", mock.Anything, []string{"a", "b"}, "u", false).Return([]string{"a"}, nil)

	hasil, err := uc.BulkAction(t.Context(), []string{"a", "b"}, "unduh_data_acuan", Pemanggil{ID: "u"})

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Berhasil: 1, Gagal: 1}, hasil)
	assert.Empty(t, recorder.Entries(), "unduh tidak mengubah data")
	repo.AssertNotCalled(t, "TandaiSelesai", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestBulkAction_Validation(t *testing.T) {
	uc, _ := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(t.Context(), []string{"a"}, "hapus", Pemanggil{ID: "u"})
	assert.ErrorIs(t, err, ErrInvalidAction)

	_, err = uc.BulkAction(t.Context(), []string{"", ""}, "tandai_selesai", Pemanggil{ID: "u"})
	assert.ErrorIs(t, err, ErrValidasi)
}

func TestBulkAction_RepoErrorSkipsAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("TandaiSelesai", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db mati"))

	_, err := uc.BulkAction(t.Context(), []string{"a"}, "tandai_selesai", Pemanggil{ID: "u"})

	require.Error(t, err)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_NothingChangedIsNotAudited(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("TandaiSelesai", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)

	hasil, err := uc.BulkAction(t.Context(), []string{"a"}, "tandai_selesai", Pemanggil{ID: "u"})

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Gagal: 1}, hasil)
	assert.Empty(t, recorder.Entries())
}
