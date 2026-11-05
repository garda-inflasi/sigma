package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	ingest "github.com/garda-inflasi/api/internal/ingestion/domain"
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

// fakeIngestion adalah Ingestion tiruan; mulaiErr dikembalikan Mulai bila diisi.
type fakeIngestion struct {
	konektor map[string]bool
	mulaiErr error
	run      *fakeRun
	dimulai  []string
	pipeline ingest.Pipeline
	sukses   map[string]time.Time
}

func (f *fakeIngestion) Pipeline(context.Context) (ingest.Pipeline, error) { return f.pipeline, nil }

func (f *fakeIngestion) TerakhirSukses(context.Context) (map[string]time.Time, error) {
	return f.sukses, nil
}

func (f *fakeIngestion) Punya(id string) bool { return f.konektor[id] }

func (f *fakeIngestion) Mulai(_ context.Context, id, pemicu string, aktor *string) (Berjalan, error) {
	if f.mulaiErr != nil {
		return nil, f.mulaiErr
	}
	f.dimulai = append(f.dimulai, id+"/"+pemicu+"/"+*aktor)
	return f.run, nil
}

func (f *fakeIngestion) Eksekusi(context.Context, string, int) ([]ingest.EksekusiSumber, error) {
	return []ingest.EksekusiSumber{{ID: 7}}, nil
}

func (f *fakeIngestion) SampelMentah(context.Context, string, int) ([]ingest.SampelMentah, error) {
	return []ingest.SampelMentah{{ID: 3}}, nil
}

type fakeRun struct {
	selesai chan struct{}
}

func (r *fakeRun) Eksekusi() ingest.EksekusiSumber {
	return ingest.EksekusiSumber{ID: 42, SumberID: "SRC-BI", Status: ingest.StatusBerjalan}
}

func (r *fakeRun) Jalankan(context.Context) (*ingest.EksekusiSumber, error) {
	close(r.selesai)
	return &ingest.EksekusiSumber{ID: 42, Status: ingest.StatusSukses}, nil
}

func newTestUseCase(repo *mockRepository) (*UseCase, *audittest.Recorder) {
	uc, recorder, _ := newTestUseCaseIngestion(repo)
	return uc, recorder
}

func newTestUseCaseIngestion(repo *mockRepository) (*UseCase, *audittest.Recorder, *fakeIngestion) {
	recorder := &audittest.Recorder{}
	ing := &fakeIngestion{konektor: map[string]bool{"SRC-BI": true}, run: &fakeRun{selesai: make(chan struct{})}}
	return New(repo, ing, recorder, logger.NewDefault()), recorder, ing
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
	// "jalankan" bukan aksi massal: menjalankan sumber punya endpoint sendiri (POST /:id/jalankan).
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

func TestBisaDijalankan_MengikutiKonektor(t *testing.T) {
	uc, _ := newTestUseCase(new(mockRepository))

	assert.True(t, uc.BisaDijalankan("SRC-BI"))
	assert.False(t, uc.BisaDijalankan("SRC-DJPB"))
}

func TestJalankan_MencatatAuditDanMenjalankanDiLatarBelakang(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder, ing := newTestUseCaseIngestion(repo)
	repo.On("GetByID", mock.Anything, "SRC-BI").Return(&domain.SumberData{ID: "SRC-BI"}, nil)

	e, err := uc.Jalankan(context.Background(), "SRC-BI", "u1")

	require.NoError(t, err)
	assert.EqualValues(t, 42, e.ID)
	assert.Equal(t, ingest.StatusBerjalan, e.Status, "respons tidak menunggu run selesai")
	assert.Equal(t, []string{"SRC-BI/MANUAL/u1"}, ing.dimulai)
	select {
	case <-ing.run.selesai:
	case <-time.After(2 * time.Second):
		t.Fatal("run tidak dijalankan di latar belakang")
	}
	entry, ok := recorder.Last()
	require.True(t, ok, "aksi yang mengubah data harus diaudit")
	assert.Equal(t, "JALANKAN_SUMBER", entry.Aksi)
	assert.Equal(t, "SRC-BI", entry.EntitasID)
	assert.Equal(t, "u1", entry.AktorID)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
}

func TestJalankan_SumberTakAdaTidakMemulaiApaPun(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder, ing := newTestUseCaseIngestion(repo)
	repo.On("GetByID", mock.Anything, "SRC-X").Return(nil, domain.ErrNotFound)

	_, err := uc.Jalankan(context.Background(), "SRC-X", "u1")

	assert.ErrorIs(t, err, ErrNotFound)
	assert.Empty(t, ing.dimulai)
	assert.Empty(t, recorder.Entries())
}

func TestJalankan_PenolakanDicatatSebagaiDitolak(t *testing.T) {
	for _, galat := range []error{ErrTanpaKonektor, ErrDijeda, ErrSudahBerjalan} {
		repo := new(mockRepository)
		uc, recorder, ing := newTestUseCaseIngestion(repo)
		ing.mulaiErr = galat
		repo.On("GetByID", mock.Anything, "SRC-BI").Return(&domain.SumberData{ID: "SRC-BI"}, nil)

		_, err := uc.Jalankan(context.Background(), "SRC-BI", "u1")

		assert.ErrorIs(t, err, galat)
		entry, ok := recorder.Last()
		require.True(t, ok)
		assert.Equal(t, audit.HasilDitolak, entry.Hasil)
	}
}

func TestEksekusiDanSampel_MemeriksaSumberAda(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("GetByID", mock.Anything, "SRC-BI").Return(&domain.SumberData{ID: "SRC-BI"}, nil)
	repo.On("GetByID", mock.Anything, "SRC-X").Return(nil, domain.ErrNotFound)

	riwayat, err := uc.Eksekusi(context.Background(), "SRC-BI", 10)
	require.NoError(t, err)
	assert.Len(t, riwayat, 1)
	sampel, err := uc.Sampel(context.Background(), "SRC-BI", 10)
	require.NoError(t, err)
	assert.Len(t, sampel, 1)

	_, err = uc.Eksekusi(context.Background(), "SRC-X", 10)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = uc.Sampel(context.Background(), "SRC-X", 10)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPipeline_KetepatanWaktuDariSelangJadwal(t *testing.T) {
	sekarang := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	repo := new(mockRepository)
	uc, _, ing := newTestUseCaseIngestion(repo)
	ing.konektor = map[string]bool{"SRC-BI": true, "SRC-BMKG": true, "SRC-BPS": true, "SRC-CS": true}
	ing.pipeline = ingest.Pipeline{Bronze: 3, Silver: 20, Gold: 5, Demo: true, Kelengkapan: new(97.5)}
	ing.sukses = map[string]time.Time{
		"SRC-BI":   sekarang.Add(-20 * time.Hour), // harian: batas 48 jam, tepat
		"SRC-BMKG": sekarang.Add(-10 * time.Hour), // tiap 3 jam: batas 6 jam, terlambat
		// SRC-BPS belum pernah sukses: terlambat.
		// SRC-CS dijeda: tidak dinilai.
	}
	repo.On("List", mock.Anything).Return([]domain.SumberData{
		{ID: "SRC-BI", JadwalCron: "0 10 * * *", Status: domain.StatusSehat},
		{ID: "SRC-BMKG", JadwalCron: "0 */3 * * *", Status: domain.StatusSehat},
		{ID: "SRC-BPS", JadwalCron: "0 6 * * *", Status: domain.StatusBelumAktif},
		{ID: "SRC-CS", JadwalCron: "*/5 * * * *", Status: domain.StatusDijeda},
		{ID: "SRC-DJPB", JadwalCron: "0 7 * * 1", Status: domain.StatusSehat}, // tanpa konektor
	}, nil)

	p, err := uc.Pipeline(context.Background(), sekarang)

	require.NoError(t, err)
	assert.EqualValues(t, 20, p.Silver)
	assert.True(t, p.Demo)
	assert.Equal(t, 97.5, *p.Kelengkapan)
	require.NotNil(t, p.KetepatanWaktu)
	assert.InDelta(t, 33.3, *p.KetepatanWaktu, 0.001, "1 dari 3 sumber yang dinilai")
}

func TestPipeline_TanpaSumberYangDinilaiKetepatanNil(t *testing.T) {
	repo := new(mockRepository)
	uc, _, ing := newTestUseCaseIngestion(repo)
	ing.konektor = map[string]bool{}
	repo.On("List", mock.Anything).Return([]domain.SumberData{{ID: "SRC-DJPB", JadwalCron: "0 7 * * 1"}}, nil)

	p, err := uc.Pipeline(context.Background(), time.Now())

	require.NoError(t, err)
	assert.Nil(t, p.KetepatanWaktu)
}
