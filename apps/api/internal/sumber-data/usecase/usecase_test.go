package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	ingest "github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// sid memberi UUID sumber dari kodenya, dengan rumus yang sama dengan seed dan migrasi 000115.
func sid(kode string) string { return idx.UUIDStabil("sumber_data", kode) }

// sumber membuat baris katalog tiruan dari kodenya.
func sumber(kode string) domain.SumberData { return domain.SumberData{ID: sid(kode), Kode: kode} }

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) List(ctx context.Context) ([]domain.SumberData, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.SumberData), args.Error(1)
}

func (m *mockRepository) Get(ctx context.Context, ref idx.RujukanUUID) (*domain.SumberData, error) {
	args := m.Called(ctx, ref)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SumberData), args.Error(1)
}

func (m *mockRepository) Cari(ctx context.Context, uuids, kodes []string) ([]domain.Ringkas, error) {
	args := m.Called(ctx, uuids, kodes)
	return args.Get(0).([]domain.Ringkas), args.Error(1)
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

func (f *fakeIngestion) Punya(kode string) bool { return f.konektor[kode] }

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
	return ingest.EksekusiSumber{ID: 42, SumberID: sid("SRC-BI"), Status: ingest.StatusBerjalan}
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
	repo.On("List", mock.Anything).Return([]domain.SumberData{{ID: sid("SRC-BPS"), Kode: "SRC-BPS", Nama: "BPS"}}, nil)

	result, err := uc.List(context.Background())

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, sid("SRC-BPS"), result[0].ID)
	assert.Equal(t, "SRC-BPS", result[0].Kode)
}

func TestList_Error(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("List", mock.Anything).Return([]domain.SumberData(nil), errors.New("db down"))

	_, err := uc.List(context.Background())

	assert.Error(t, err)
}

func TestGet_PassesNotFoundThrough(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	ref := idx.RujukanUUID{Kode: "SRC-X"}
	repo.On("Get", mock.Anything, ref).Return(nil, domain.ErrNotFound)

	_, err := uc.Get(context.Background(), ref)

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGet_DatabaseErrorIsNotReportedAsNotFound(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	ref := idx.RujukanUUID{ID: sid("SRC-BPS")}
	repo.On("Get", mock.Anything, ref).Return(nil, errors.New("db down"))

	_, err := uc.Get(context.Background(), ref)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestBulkAction_RejectsUnknownActions(t *testing.T) {
	// "jalankan" bukan aksi massal: menjalankan sumber punya endpoint sendiri (POST /:id/jalankan).
	for _, action := range []string{"jalankan", "hapus", ""} {
		repo := new(mockRepository)
		uc, recorder := newTestUseCase(repo)

		_, err := uc.BulkAction(context.Background(), []string{sid("SRC-BPS")}, action, uid("u1"))

		assert.ErrorIs(t, err, ErrInvalidAction, action)
		repo.AssertNotCalled(t, "BulkAction")
		repo.AssertNotCalled(t, "Cari")
		assert.Empty(t, recorder.Entries())
	}
}

func TestBulkAction_CountsFailuresAndWritesAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	// Entri ganda dihitung sekali; entri bermacam ejaan dipisah menjadi UUID dan kode.
	repo.On("Cari", mock.Anything, []string{sid("SRC-BPS")}, []string{"SRC-BI"}).Return([]domain.Ringkas{
		{ID: sid("SRC-BI"), Kode: "SRC-BI"}, {ID: sid("SRC-BPS"), Kode: "SRC-BPS"},
	}, nil)
	// Hanya satu dari dua yang berubah.
	repo.On("BulkAction", mock.Anything, []string{sid("SRC-BI"), sid("SRC-BPS")}, domain.BulkJeda).Return([]string{sid("SRC-BPS")}, nil)

	res, err := uc.BulkAction(context.Background(), []string{sid("SRC-BPS"), "SRC-BI", sid("SRC-BPS")}, "jeda", uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 1, Gagal: 1}, res)
	entry, ok := recorder.Last()
	require.True(t, ok, "aksi yang mengubah data harus diaudit")
	assert.Equal(t, "BULK_jeda", entry.Aksi)
	assert.Equal(t, "sumber_data", entry.Entitas)
	assert.Equal(t, uid("u1"), entry.AktorID)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
	repo.AssertExpectations(t)
}

func TestBulkAction_EntriTidakAdaDihitungGagalDanTidakMenyentuhBasisData(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("Cari", mock.Anything, []string(nil), []string{"SRC-ASING", "SRC-BI"}).Return([]domain.Ringkas{{ID: sid("SRC-BI"), Kode: "SRC-BI"}}, nil)
	repo.On("BulkAction", mock.Anything, []string{sid("SRC-BI")}, domain.BulkJeda).Return([]string{sid("SRC-BI")}, nil)

	res, err := uc.BulkAction(context.Background(), []string{"SRC-BI", "SRC-ASING"}, "jeda", uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 1, Gagal: 1}, res, "sumber yang tidak ada dihitung gagal")

	// Tidak ada satu pun yang ditemukan: aksi tidak dikirim ke basis data.
	repo2 := new(mockRepository)
	uc2, rec2 := newTestUseCase(repo2)
	repo2.On("Cari", mock.Anything, []string(nil), []string{"SRC-ASING"}).Return([]domain.Ringkas(nil), nil)
	res, err = uc2.BulkAction(context.Background(), []string{"SRC-ASING"}, "jeda", uid("u1"))
	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 0, Gagal: 1}, res)
	repo2.AssertNotCalled(t, "BulkAction")
	assert.Len(t, rec2.Entries(), 1, "permintaan tetap diaudit")
}

func TestBulkAction_UUIDDanKodeSumberYangSamaDihitungSatu(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	repo.On("Cari", mock.Anything, []string{sid("SRC-BI")}, []string{"SRC-BI"}).Return([]domain.Ringkas{{ID: sid("SRC-BI"), Kode: "SRC-BI"}}, nil)
	repo.On("BulkAction", mock.Anything, []string{sid("SRC-BI")}, domain.BulkJeda).Return([]string{sid("SRC-BI")}, nil)

	res, err := uc.BulkAction(context.Background(), []string{sid("SRC-BI"), "SRC-BI"}, "jeda", uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 1, Gagal: 0}, res)
}

func TestBulkAction_UUIDHurufBesarDinormalkan(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	besar := strings.ToUpper(sid("SRC-BI"))
	repo.On("Cari", mock.Anything, []string{sid("SRC-BI")}, []string(nil)).Return([]domain.Ringkas{{ID: sid("SRC-BI"), Kode: "SRC-BI"}}, nil)
	repo.On("BulkAction", mock.Anything, []string{sid("SRC-BI")}, domain.BulkLanjutkan).Return([]string{sid("SRC-BI")}, nil)

	res, err := uc.BulkAction(context.Background(), []string{besar}, "lanjutkan", uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, BulkResult{Berhasil: 1, Gagal: 0}, res)
}

func TestBulkAction_RepositoryErrorWritesNoAudit(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	repo.On("Cari", mock.Anything, []string(nil), []string{"SRC-BPS"}).Return([]domain.Ringkas{{ID: sid("SRC-BPS"), Kode: "SRC-BPS"}}, nil)
	repo.On("BulkAction", mock.Anything, []string{sid("SRC-BPS")}, domain.BulkLanjutkan).Return([]string(nil), errors.New("db down"))

	_, err := uc.BulkAction(context.Background(), []string{"SRC-BPS"}, "lanjutkan", uid("u1"))

	require.Error(t, err)
	assert.Empty(t, recorder.Entries())
}

func TestBulkAction_AuditFailureDoesNotFailTheAction(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder := newTestUseCase(repo)
	recorder.Err = errors.New("audit down")
	repo.On("Cari", mock.Anything, []string(nil), []string{"SRC-BPS"}).Return([]domain.Ringkas{{ID: sid("SRC-BPS"), Kode: "SRC-BPS"}}, nil)
	repo.On("BulkAction", mock.Anything, []string{sid("SRC-BPS")}, domain.BulkJeda).Return([]string{sid("SRC-BPS")}, nil)

	res, err := uc.BulkAction(context.Background(), []string{"SRC-BPS"}, "jeda", uid("u1"))

	require.NoError(t, err)
	assert.Equal(t, 1, res.Berhasil)
}

func TestBisaDijalankan_MengikutiKonektorMenurutKode(t *testing.T) {
	uc, _ := newTestUseCase(new(mockRepository))

	assert.True(t, uc.BisaDijalankan("SRC-BI"))
	assert.False(t, uc.BisaDijalankan("SRC-DJPB"))
	assert.False(t, uc.BisaDijalankan(sid("SRC-BI")), "konektor dikenali lewat kode, bukan UUID")
}

func TestJalankan_MencatatAuditDanMenjalankanDiLatarBelakang(t *testing.T) {
	// Rute menerima kode sebagai alias; engine dan audit memakai UUID sumber.
	repo := new(mockRepository)
	uc, recorder, ing := newTestUseCaseIngestion(repo)
	ref := idx.RujukanUUID{Kode: "SRC-BI"}
	repo.On("Get", mock.Anything, ref).Return(&domain.SumberData{ID: sid("SRC-BI"), Kode: "SRC-BI"}, nil)

	e, err := uc.Jalankan(context.Background(), ref, uid("u1"))

	require.NoError(t, err)
	assert.EqualValues(t, 42, e.ID)
	assert.Equal(t, ingest.StatusBerjalan, e.Status, "respons tidak menunggu run selesai")
	assert.Equal(t, []string{sid("SRC-BI") + "/MANUAL/" + uid("u1")}, ing.dimulai)
	select {
	case <-ing.run.selesai:
	case <-time.After(2 * time.Second):
		t.Fatal("run tidak dijalankan di latar belakang")
	}
	entry, ok := recorder.Last()
	require.True(t, ok, "aksi yang mengubah data harus diaudit")
	assert.Equal(t, "JALANKAN_SUMBER", entry.Aksi)
	assert.Equal(t, sid("SRC-BI"), entry.EntitasID, "audit mencatat UUID sumber")
	assert.Equal(t, "SRC-BI", entry.Metadata["kode"])
	assert.Equal(t, uid("u1"), entry.AktorID)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
}

func TestJalankan_SumberTakAdaTidakMemulaiApaPun(t *testing.T) {
	repo := new(mockRepository)
	uc, recorder, ing := newTestUseCaseIngestion(repo)
	ref := idx.RujukanUUID{Kode: "SRC-X"}
	repo.On("Get", mock.Anything, ref).Return(nil, domain.ErrNotFound)

	_, err := uc.Jalankan(context.Background(), ref, uid("u1"))

	assert.ErrorIs(t, err, ErrNotFound)
	assert.Empty(t, ing.dimulai)
	assert.Empty(t, recorder.Entries())
}

func TestJalankan_PenolakanDicatatSebagaiDitolak(t *testing.T) {
	for _, galat := range []error{ErrTanpaKonektor, ErrDijeda, ErrSudahBerjalan} {
		repo := new(mockRepository)
		uc, recorder, ing := newTestUseCaseIngestion(repo)
		ing.mulaiErr = galat
		ref := idx.RujukanUUID{ID: sid("SRC-BI")}
		repo.On("Get", mock.Anything, ref).Return(&domain.SumberData{ID: sid("SRC-BI"), Kode: "SRC-BI"}, nil)

		_, err := uc.Jalankan(context.Background(), ref, uid("u1"))

		assert.ErrorIs(t, err, galat)
		entry, ok := recorder.Last()
		require.True(t, ok)
		assert.Equal(t, audit.HasilDitolak, entry.Hasil)
		assert.Equal(t, sid("SRC-BI"), entry.EntitasID)
	}
}

func TestEksekusiDanSampel_MemeriksaSumberAda(t *testing.T) {
	repo := new(mockRepository)
	uc, _ := newTestUseCase(repo)
	ada, tidakAda := idx.RujukanUUID{Kode: "SRC-BI"}, idx.RujukanUUID{Kode: "SRC-X"}
	repo.On("Get", mock.Anything, ada).Return(&domain.SumberData{ID: sid("SRC-BI"), Kode: "SRC-BI"}, nil)
	repo.On("Get", mock.Anything, tidakAda).Return(nil, domain.ErrNotFound)

	riwayat, err := uc.Eksekusi(context.Background(), ada, 10)
	require.NoError(t, err)
	assert.Len(t, riwayat, 1)
	sampel, err := uc.Sampel(context.Background(), ada, 10)
	require.NoError(t, err)
	assert.Len(t, sampel, 1)

	_, err = uc.Eksekusi(context.Background(), tidakAda, 10)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = uc.Sampel(context.Background(), tidakAda, 10)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPipeline_KetepatanWaktuDariSelangJadwal(t *testing.T) {
	sekarang := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	repo := new(mockRepository)
	uc, _, ing := newTestUseCaseIngestion(repo)
	// Konektor dikenali lewat kode; waktu sukses terakhir dikunci dengan UUID sumber.
	ing.konektor = map[string]bool{"SRC-BI": true, "SRC-BMKG": true, "SRC-BPS": true, "SRC-CS": true}
	ing.pipeline = ingest.Pipeline{Bronze: 3, Silver: 20, Gold: 5, Demo: true, Kelengkapan: new(97.5)}
	ing.sukses = map[string]time.Time{
		sid("SRC-BI"):   sekarang.Add(-20 * time.Hour), // harian: batas 48 jam, tepat
		sid("SRC-BMKG"): sekarang.Add(-10 * time.Hour), // tiap 3 jam: batas 6 jam, terlambat
		// SRC-BPS belum pernah sukses: terlambat.
		// SRC-CS dijeda: tidak dinilai.
	}
	repo.On("List", mock.Anything).Return([]domain.SumberData{
		{ID: sid("SRC-BI"), Kode: "SRC-BI", JadwalCron: "0 10 * * *", Status: domain.StatusSehat},
		{ID: sid("SRC-BMKG"), Kode: "SRC-BMKG", JadwalCron: "0 */3 * * *", Status: domain.StatusSehat},
		{ID: sid("SRC-BPS"), Kode: "SRC-BPS", JadwalCron: "0 6 * * *", Status: domain.StatusBelumAktif},
		{ID: sid("SRC-CS"), Kode: "SRC-CS", JadwalCron: "*/5 * * * *", Status: domain.StatusDijeda},
		{ID: sid("SRC-DJPB"), Kode: "SRC-DJPB", JadwalCron: "0 7 * * 1", Status: domain.StatusSehat}, // tanpa konektor
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
	repo.On("List", mock.Anything).Return([]domain.SumberData{{ID: sid("SRC-DJPB"), Kode: "SRC-DJPB", JadwalCron: "0 7 * * 1"}}, nil)

	p, err := uc.Pipeline(context.Background(), time.Now())

	require.NoError(t, err)
	assert.Nil(t, p.KetepatanWaktu)
}
