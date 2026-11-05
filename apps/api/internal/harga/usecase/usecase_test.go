package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context, filter domain.ListFilter) ([]domain.EntriView, int64, error) {
	args := m.Called(ctx, filter)
	items, _ := args.Get(0).([]domain.EntriView)
	return items, args.Get(1).(int64), args.Error(2)
}

func (m *mockRepository) GetByID(ctx context.Context, id string) (*domain.EntriView, error) {
	args := m.Called(ctx, id)
	entri, _ := args.Get(0).(*domain.EntriView)
	return entri, args.Error(1)
}

func (m *mockRepository) Ringkasan(ctx context.Context, sejak time.Time) (domain.Ringkasan, error) {
	args := m.Called(ctx, sejak)
	return args.Get(0).(domain.Ringkasan), args.Error(1)
}

func (m *mockRepository) Tren(ctx context.Context, komoditas, kabupaten string, sejak time.Time, interval string) ([]domain.TitikTren, error) {
	args := m.Called(ctx, komoditas, kabupaten, sejak, interval)
	tren, _ := args.Get(0).([]domain.TitikTren)
	return tren, args.Error(1)
}

func (m *mockRepository) SatuanKomoditas(ctx context.Context, komoditas, pasar string) (string, error) {
	args := m.Called(ctx, komoditas, pasar)
	return args.String(0), args.Error(1)
}

func (m *mockRepository) Create(ctx context.Context, entri *domain.EntriHarga) error {
	return m.Called(ctx, entri).Error(0)
}

func (m *mockRepository) Transisi(ctx context.Context, ids []string, aksi domain.BulkAction, aktor string, waktu time.Time) ([]string, error) {
	args := m.Called(ctx, ids, aksi, aktor, waktu)
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

func TestListHarga_NormalizesPaging(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("List", mock.Anything, domain.ListFilter{Status: "DIAJUKAN", Page: 1, Limit: DefaultLimit}).
		Return([]domain.EntriView{{ID: "h1"}}, int64(7), nil)

	items, total, err := uc.ListHarga(t.Context(), domain.ListFilter{Status: "DIAJUKAN"})

	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.EqualValues(t, 7, total)
	repo.AssertExpectations(t)
}

func TestListHarga_CapsLimit(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("List", mock.Anything, domain.ListFilter{Page: 2, Limit: MaxLimit}).Return([]domain.EntriView{}, int64(0), nil)

	_, _, err := uc.ListHarga(t.Context(), domain.ListFilter{Page: 2, Limit: 99999})

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestListHarga_RejectsUnknownStatus(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)

	_, _, err := uc.ListHarga(t.Context(), domain.ListFilter{Status: "BASI"})

	require.ErrorIs(t, err, ErrValidasi)
	repo.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
}

func TestListHarga_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("List", mock.Anything, mock.Anything).Return(nil, int64(0), errors.New("db mati"))

	_, _, err := uc.ListHarga(t.Context(), domain.ListFilter{})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrValidasi)
}

func TestGetHargaByID(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("GetByID", mock.Anything, "ada").Return(&domain.EntriView{ID: "ada"}, nil)
	repo.On("GetByID", mock.Anything, "hilang").Return(nil, domain.ErrNotFound)

	got, err := uc.GetHargaByID(t.Context(), "ada")
	require.NoError(t, err)
	assert.Equal(t, "ada", got.ID)

	_, err = uc.GetHargaByID(t.Context(), "hilang")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestRingkasan_WindowDefaultsAndCaps(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("Ringkasan", mock.Anything, fixedNow.AddDate(0, 0, -DefaultRingkasanHari)).Return(domain.Ringkasan{Diajukan: 3}, nil)
	repo.On("Ringkasan", mock.Anything, fixedNow.AddDate(0, 0, -MaxHari)).Return(domain.Ringkasan{}, nil)

	r, err := uc.Ringkasan(t.Context(), 0)
	require.NoError(t, err)
	assert.EqualValues(t, 3, r.Diajukan)

	_, err = uc.Ringkasan(t.Context(), 10_000)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestGetTren(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	tren := []domain.TitikTren{{Tanggal: fixedNow}}
	repo.On("Tren", mock.Anything, "CBM-KRT", "1171", fixedNow.AddDate(0, 0, -DefaultTrenHari), domain.IntervalHarian).Return(tren, nil)

	got, err := uc.GetTren(t.Context(), "CBM-KRT", "1171", 0, "")

	require.NoError(t, err)
	assert.Equal(t, tren, got)
}

func TestGetTren_Validation(t *testing.T) {
	uc, _ := newTestUseCase(new(mockRepository))

	_, err := uc.GetTren(t.Context(), "", "1171", 0, "")
	assert.ErrorIs(t, err, ErrValidasi)
	_, err = uc.GetTren(t.Context(), "CBM-KRT", "", 0, "")
	assert.ErrorIs(t, err, ErrValidasi)
	_, err = uc.GetTren(t.Context(), "CBM-KRT", "1171", 0, "tahunan")
	assert.ErrorIs(t, err, ErrValidasi)
}

func TestCreateHarga_StoresPendingEntryAndAudits(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("SatuanKomoditas", mock.Anything, "CBM-KRT", "PSR-1171-01").Return("kg", nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.EntriHarga")).Return(nil)

	entri, err := uc.CreateHarga(t.Context(), CreateInput{
		KomoditasKode: "CBM-KRT", PasarID: "PSR-1171-01", Tipe: domain.TipeKonsumen, Harga: 52000,
	}, "mahasiswa_1")

	require.NoError(t, err)
	assert.NotEmpty(t, entri.ID)
	assert.Equal(t, domain.StatusDiajukan, entri.Status)
	assert.Equal(t, "kg", entri.Satuan)
	assert.Equal(t, "MEDIUM", entri.Kualitas)
	assert.Equal(t, fixedNow, entri.DicatatPada)
	require.NotNil(t, entri.PetugasID)
	assert.Equal(t, "mahasiswa_1", *entri.PetugasID)

	entry, ok := recorder.Last()
	require.True(t, ok, "aksi yang mengubah data tercatat di audit")
	assert.Equal(t, "HARGA_INPUT", entry.Aksi)
	assert.Equal(t, entri.ID, entry.EntitasID)
	assert.Equal(t, "mahasiswa_1", entry.AktorID)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
}

func TestCreateHarga_RejectsFutureTimestamp(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	masaDepan := fixedNow.Add(time.Hour)

	_, err := uc.CreateHarga(t.Context(), CreateInput{KomoditasKode: "CBM-KRT", PasarID: "P", Tipe: domain.TipeKonsumen, Harga: 1, DicatatPada: &masaDepan}, "u")

	require.ErrorIs(t, err, ErrValidasi)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	assert.Empty(t, recorder.Entries())
}

func TestCreateHarga_UnknownReference(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("SatuanKomoditas", mock.Anything, "XXX", "P").Return("", domain.ErrReferensiTidakAda)

	_, err := uc.CreateHarga(t.Context(), CreateInput{KomoditasKode: "XXX", PasarID: "P", Tipe: domain.TipeKonsumen, Harga: 1}, "u")

	require.ErrorIs(t, err, ErrValidasi)
	assert.Empty(t, recorder.Entries())
}

func TestCreateHarga_RepoErrorSkipsAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("SatuanKomoditas", mock.Anything, "CBM-KRT", "P").Return("kg", nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db mati"))

	_, err := uc.CreateHarga(t.Context(), CreateInput{KomoditasKode: "CBM-KRT", PasarID: "P", Tipe: domain.TipeKonsumen, Harga: 1}, "u")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrValidasi)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_ReportsChangedAndSkipped(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	// id ganda dan kosong disaring: tiga id unik, dua berubah.
	repo.On("Transisi", mock.Anything, []string{"a", "b", "c"}, domain.BulkValidasi, "koordinator", fixedNow).Return([]string{"a", "c"}, nil)

	hasil, err := uc.BulkAction(t.Context(), []string{"a", "b", "a", "", "c"}, "validasi", "koordinator")

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Berhasil: 2, Gagal: 1}, hasil)

	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "HARGA_VALIDASI", entry.Aksi)
	assert.Equal(t, "koordinator", entry.AktorID)
	assert.Equal(t, map[string]any{"ids": []string{"a", "c"}, "gagal": 1}, entry.Metadata)
}

func TestBulkAction_InvalidAction(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)

	_, err := uc.BulkAction(t.Context(), []string{"a"}, "hapus", "u")

	assert.ErrorIs(t, err, ErrInvalidAction)
	repo.AssertNotCalled(t, "Transisi", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestBulkAction_RejectsEmptyAndOversizedIDs(t *testing.T) {
	uc, _ := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(t.Context(), []string{"", ""}, "validasi", "u")
	assert.ErrorIs(t, err, ErrValidasi)

	banyak := make([]string, MaxBulkIDs+1)
	for i := range banyak {
		banyak[i] = string(rune('A'+i%26)) + time.Duration(i).String()
	}
	_, err = uc.BulkAction(t.Context(), banyak, "validasi", "u")
	assert.ErrorIs(t, err, ErrValidasi)
}

func TestBulkAction_RepoErrorSkipsAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("Transisi", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db mati"))

	_, err := uc.BulkAction(t.Context(), []string{"a"}, "tolak", "u")

	require.Error(t, err)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_AuditFailureDoesNotUndoAction(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	recorder.Err = errors.New("audit mati")
	repo.On("Transisi", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{"a"}, nil)

	hasil, err := uc.BulkAction(t.Context(), []string{"a"}, "publikasi", "u")

	require.NoError(t, err)
	assert.Equal(t, 1, hasil.Berhasil)
}
