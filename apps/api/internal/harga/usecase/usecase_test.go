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

func (m *mockRepository) Simpan(ctx context.Context, kirim domain.Pengiriman) (domain.HasilSimpan, error) {
	args := m.Called(ctx, kirim)
	hasil, _ := args.Get(0).(domain.HasilSimpan)
	if hasil.Entri == nil && hasil.Bentrok == nil && args.Error(1) == nil {
		hasil.Entri = kirim.Entri
	}
	return hasil, args.Error(1)
}

func (m *mockRepository) Transisi(ctx context.Context, ids []string, aksi domain.BulkAction, aktor, alasan string, waktu time.Time) ([]domain.HasilTransisi, error) {
	args := m.Called(ctx, ids, aksi, aktor, alasan, waktu)
	hasil, _ := args.Get(0).([]domain.HasilTransisi)
	return hasil, args.Error(1)
}

func (m *mockRepository) Riwayat(ctx context.Context, entriID string) ([]domain.RiwayatView, error) {
	args := m.Called(ctx, entriID)
	items, _ := args.Get(0).([]domain.RiwayatView)
	return items, args.Error(1)
}

var fixedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

type fakeRefresher struct{ triggers int }

func (f *fakeRefresher) Trigger() { f.triggers++ }

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	uc, recorder, _ := newTestUseCaseWithRefresher(repo)
	return uc, recorder
}

func newTestUseCaseWithRefresher(repo *mockRepository) (*UseCase, *audittest.Recorder, *fakeRefresher) {
	recorder, refresh := &audittest.Recorder{}, &fakeRefresher{}
	uc := New(repo, recorder, refresh, logger.NewDefault())
	uc.now = func() time.Time { return fixedNow }
	return uc, recorder, refresh
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
	repo.On("Simpan", mock.Anything, mock.AnythingOfType("domain.Pengiriman")).Return(domain.HasilSimpan{}, nil)

	hasil, err := uc.CreateHarga(t.Context(), CreateInput{
		KomoditasKode: "CBM-KRT", PasarID: "PSR-1171-01", Tipe: domain.TipeKonsumen, Harga: 52000,
	}, "mahasiswa_1")

	require.NoError(t, err)
	entri := hasil.Entri
	assert.False(t, hasil.Diulang)
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
	repo.AssertNotCalled(t, "Simpan", mock.Anything, mock.Anything)
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

func TestCreateHarga_PassesIdempotencyKeyAndPertahankan(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("SatuanKomoditas", mock.Anything, "CBM-KRT", "P").Return("kg", nil)
	repo.On("Simpan", mock.Anything, mock.MatchedBy(func(k domain.Pengiriman) bool {
		return k.Pertahankan && k.Entri.IdempotencyKey != nil && *k.Entri.IdempotencyKey == "kunci-1"
	})).Return(domain.HasilSimpan{}, nil)

	_, err := uc.CreateHarga(t.Context(), CreateInput{
		KomoditasKode: "CBM-KRT", PasarID: "P", Tipe: domain.TipeKonsumen, Harga: 1, IdempotencyKey: "kunci-1", Pertahankan: true,
	}, "u")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestCreateHarga_ReplayIsNotAuditedAgain(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	lama := &domain.EntriHarga{ID: "lama", Status: domain.StatusDiajukan}
	repo.On("SatuanKomoditas", mock.Anything, "CBM-KRT", "P").Return("kg", nil)
	repo.On("Simpan", mock.Anything, mock.Anything).Return(domain.HasilSimpan{Entri: lama, Diulang: true}, nil)

	hasil, err := uc.CreateHarga(t.Context(), CreateInput{KomoditasKode: "CBM-KRT", PasarID: "P", Tipe: domain.TipeKonsumen, Harga: 1, IdempotencyKey: "k"}, "u")

	require.NoError(t, err)
	assert.True(t, hasil.Diulang)
	assert.Equal(t, "lama", hasil.Entri.ID)
	assert.Empty(t, recorder.Entries())
}

func TestCreateHarga_ConflictReturnsServerVersion(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	bentrok := &domain.Bentrok{ID: "e1", Harga: 52000, DicatatPada: fixedNow, PetugasNama: "Cut Nanda"}
	repo.On("SatuanKomoditas", mock.Anything, "CBM-KRT", "P").Return("kg", nil)
	repo.On("Simpan", mock.Anything, mock.Anything).Return(domain.HasilSimpan{Bentrok: bentrok}, nil)

	_, err := uc.CreateHarga(t.Context(), CreateInput{KomoditasKode: "CBM-KRT", PasarID: "P", Tipe: domain.TipeKonsumen, Harga: 60000}, "u")

	var got *BentrokError
	require.ErrorAs(t, err, &got)
	assert.Equal(t, *bentrok, got.Server)
	assert.Empty(t, recorder.Entries(), "kiriman yang ditolak tidak mengubah data")
}

func TestCreateHarga_RejectsHalfCoordinates(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	lat := -5.5

	_, err := uc.CreateHarga(t.Context(), CreateInput{KomoditasKode: "CBM-KRT", PasarID: "P", Tipe: domain.TipeKonsumen, Harga: 1, Lat: &lat}, "u")

	require.ErrorIs(t, err, ErrValidasi)
	repo.AssertNotCalled(t, "Simpan", mock.Anything, mock.Anything)
}

func TestCreateHarga_RepoErrorSkipsAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("SatuanKomoditas", mock.Anything, "CBM-KRT", "P").Return("kg", nil)
	repo.On("Simpan", mock.Anything, mock.Anything).Return(domain.HasilSimpan{}, errors.New("db mati"))

	_, err := uc.CreateHarga(t.Context(), CreateInput{KomoditasKode: "CBM-KRT", PasarID: "P", Tipe: domain.TipeKonsumen, Harga: 1}, "u")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrValidasi)
	assert.Empty(t, recorder.Entries())
}

func lolos(id string) domain.HasilTransisi { return domain.HasilTransisi{ID: id} }

func lewat(id, sebab string) domain.HasilTransisi { return domain.HasilTransisi{ID: id, Sebab: sebab} }

func TestBulkAction_ReportsChangedAndSkippedWithReasons(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	// id ganda dan kosong disaring: tiga id unik, dua berubah.
	repo.On("Transisi", mock.Anything, []string{"a", "b", "c"}, domain.BulkValidasi, "koordinator", "", fixedNow).
		Return([]domain.HasilTransisi{lolos("a"), lewat("b", domain.SebabStatus), lolos("c")}, nil)

	hasil, err := uc.BulkAction(t.Context(), []string{"a", "b", "a", "", "c"}, "validasi", "koordinator", "")

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Berhasil: 2, Gagal: 1, Rincian: []RincianBulk{{ID: "b", Sebab: domain.SebabStatus}}}, hasil)

	entry, ok := recorder.Last()
	require.True(t, ok)
	assert.Equal(t, "HARGA_VALIDASI", entry.Aksi)
	assert.Equal(t, "koordinator", entry.AktorID)
	assert.Equal(t, map[string]any{"ids": []string{"a", "c"}, "gagal": 1}, entry.Metadata)
}

func TestBulkAction_TolakRequiresReasonAndAuditsIt(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)

	for _, alasan := range []string{"", "  ", "ab"} {
		_, err := uc.BulkAction(t.Context(), []string{"a"}, "tolak", "u", alasan)
		require.ErrorIs(t, err, ErrValidasi, "alasan %q", alasan)
	}
	repo.AssertNotCalled(t, "Transisi", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	repo.On("Transisi", mock.Anything, []string{"a"}, domain.BulkTolak, "u", "harga tidak wajar", fixedNow).Return([]domain.HasilTransisi{lolos("a")}, nil)
	_, err := uc.BulkAction(t.Context(), []string{"a"}, "tolak", "u", "  harga tidak wajar ")
	require.NoError(t, err)
	entry, _ := recorder.Last()
	assert.Equal(t, "harga tidak wajar", entry.Metadata["alasan"])
}

func TestBulkAction_ReasonIsOptionalForOtherActions(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("Transisi", mock.Anything, []string{"a"}, domain.BulkKembalikan, "u", "", fixedNow).Return([]domain.HasilTransisi{lolos("a")}, nil)

	_, err := uc.BulkAction(t.Context(), []string{"a"}, "kembalikan", "u", "")

	require.NoError(t, err)
}

func TestBulkAction_InvalidAction(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)

	_, err := uc.BulkAction(t.Context(), []string{"a"}, "hapus", "u", "")

	assert.ErrorIs(t, err, ErrInvalidAction)
	repo.AssertNotCalled(t, "Transisi", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestBulkAction_RejectsEmptyAndOversizedIDs(t *testing.T) {
	uc, _ := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(t.Context(), []string{"", ""}, "validasi", "u", "")
	assert.ErrorIs(t, err, ErrValidasi)

	banyak := make([]string, MaxBulkIDs+1)
	for i := range banyak {
		banyak[i] = string(rune('A'+i%26)) + time.Duration(i).String()
	}
	_, err = uc.BulkAction(t.Context(), banyak, "validasi", "u", "")
	assert.ErrorIs(t, err, ErrValidasi)
}

func TestBulkAction_RepoErrorSkipsAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("Transisi", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db mati"))

	_, err := uc.BulkAction(t.Context(), []string{"a"}, "validasi", "u", "")

	require.Error(t, err)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_AuditFailureDoesNotUndoAction(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	recorder.Err = errors.New("audit mati")
	repo.On("Transisi", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]domain.HasilTransisi{lolos("a")}, nil)

	hasil, err := uc.BulkAction(t.Context(), []string{"a"}, "publikasi", "u", "")

	require.NoError(t, err)
	assert.Equal(t, 1, hasil.Berhasil)
}

func TestBulkAction_NothingChangedIsNotAuditedOrRefreshed(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder, refresh := newTestUseCaseWithRefresher(repo)
	repo.On("Transisi", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]domain.HasilTransisi{lewat("a", domain.SebabTidakDitemukan)}, nil)

	hasil, err := uc.BulkAction(t.Context(), []string{"a"}, "validasi", "u", "")

	require.NoError(t, err)
	assert.Equal(t, HasilBulk{Gagal: 1, Rincian: []RincianBulk{{ID: "a", Sebab: domain.SebabTidakDitemukan}}}, hasil)
	assert.Empty(t, recorder.Entries())
	assert.Zero(t, refresh.triggers, "tanpa perubahan tidak perlu menyegarkan agregat")
}

func TestBulkAction_RefreshesDashboardWhenRowsChanged(t *testing.T) {
	repo := new(mockRepository)
	uc, _, refresh := newTestUseCaseWithRefresher(repo)
	repo.On("Transisi", mock.Anything, []string{"b"}, domain.BulkValidasi, "u", "", fixedNow).Return([]domain.HasilTransisi{lolos("b")}, nil)

	_, err := uc.BulkAction(t.Context(), []string{"b"}, "validasi", "u", "")

	require.NoError(t, err)
	assert.Equal(t, 1, refresh.triggers)
}

func TestRiwayat_OnlyOwnEntryWithoutViewPermission(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	pemilik := "mahasiswa_1"
	repo.On("GetByID", mock.Anything, "e1").Return(&domain.EntriView{ID: "e1", PetugasID: &pemilik}, nil)
	repo.On("Riwayat", mock.Anything, "e1").Return([]domain.RiwayatView{{Ke: domain.StatusDiajukan}}, nil)

	items, err := uc.Riwayat(t.Context(), "e1", "mahasiswa_1", false)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	_, err = uc.Riwayat(t.Context(), "e1", "mahasiswa_2", false)
	assert.ErrorIs(t, err, ErrNotFound, "entri petugas lain tampil sebagai tidak ada")

	items, err = uc.Riwayat(t.Context(), "e1", "koordinator", true)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}
