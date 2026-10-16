package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Mock Repository ───────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) ListFasilitas(ctx context.Context) ([]domain.FasilitasView, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.FasilitasView), args.Error(1)
}

func (m *mockRepository) ListStok(ctx context.Context, q domain.StokQuery) (domain.StokPage, error) {
	args := m.Called(ctx, q)
	return args.Get(0).(domain.StokPage), args.Error(1)
}

func (m *mockRepository) RingkasanStok(ctx context.Context, ambang float64) (domain.RingkasanStok, error) {
	args := m.Called(ctx, ambang)
	return args.Get(0).(domain.RingkasanStok), args.Error(1)
}

func (m *mockRepository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction) (domain.BulkResult, error) {
	args := m.Called(ctx, ids, action)
	return args.Get(0).(domain.BulkResult), args.Error(1)
}

func (m *mockRepository) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

// ── Test Helpers ──────────────────────────────────────────────────────

var fixedNow = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

func newTestUseCase(repo *mockRepository) *UseCase {
	return New(repo, logger.NewDefault()).WithClock(func() time.Time { return fixedNow })
}

// defaultQuery adalah kriteria setelah normalisasi bila pemanggil tidak mengisi apa pun.
var defaultQuery = domain.StokQuery{Sort: domain.SortSisaMasaSimpan, Page: 1, Limit: DefaultLimit}

func expectEmptyOverview(repo *mockRepository, q domain.StokQuery) {
	repo.On("ListFasilitas", mock.Anything).Return([]domain.FasilitasView{}, nil)
	repo.On("ListStok", mock.Anything, q).Return(domain.StokPage{}, nil)
	repo.On("RingkasanStok", mock.Anything, domain.RisikoTinggiAmbang).Return(domain.RingkasanStok{}, nil)
}

// ── GetAll Tests ──────────────────────────────────────────────────────

func TestGetAll_Success(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	aktual := 3.5
	fasilitas := []domain.FasilitasView{
		{ID: "CS-01", Nama: "Cold Storage A", KapasitasTon: 400, TerisiTon: 346.99999999999994, SuhuTargetC: 4, SuhuAktualC: &aktual, Status: "OPERASIONAL"},
		{ID: "CS-05", Nama: "Gudang Rencana", KapasitasTon: 120, Status: "RENCANA"},
	}
	stok := domain.StokPage{
		Items: []domain.StokView{
			{ID: "S1", ColdStorageID: "CS-01", VolumeTon: 34.900000000000006, KedaluwarsaPada: fixedNow.Add(5 * 24 * time.Hour)},
		},
		Total:          23,
		TotalVolumeTon: 812.0000000000001,
	}
	ringkasan := domain.RingkasanStok{TotalLot: 23, TotalVolumeTon: 812.00000001, RisikoTinggiLot: 4, RisikoTinggiTon: 172.70000000000002}

	repo.On("ListFasilitas", mock.Anything).Return(fasilitas, nil)
	repo.On("ListStok", mock.Anything, defaultQuery).Return(stok, nil)
	repo.On("RingkasanStok", mock.Anything, domain.RisikoTinggiAmbang).Return(ringkasan, nil)

	got, err := uc.GetAll(context.Background(), domain.StokQuery{})

	require.NoError(t, err)
	require.Len(t, got.Fasilitas, 2)
	assert.Equal(t, 347.0, got.Fasilitas[0].TerisiTon, "volume terisi dibulatkan satu desimal")
	assert.Equal(t, 86.8, got.Fasilitas[0].UtilisasiPersen)
	assert.Equal(t, 0.0, got.Fasilitas[1].UtilisasiPersen, "fasilitas rencana belum terisi")
	assert.Nil(t, got.Fasilitas[1].SuhuAktualC)

	require.Len(t, got.Stok, 1)
	assert.Equal(t, 34.9, got.Stok[0].VolumeTon)
	assert.Equal(t, 5, got.Stok[0].SisaMasaSimpanHari)

	assert.Equal(t, 1, got.Page)
	assert.Equal(t, DefaultLimit, got.Limit)
	assert.Equal(t, int64(23), got.Total)
	assert.Equal(t, 3, got.TotalPages)
	assert.Equal(t, 812.0, got.TotalVolumeTon)

	assert.Equal(t, int64(23), got.Ringkasan.TotalLot)
	assert.Equal(t, 812.0, got.Ringkasan.TotalVolumeTon)
	assert.Equal(t, 172.7, got.Ringkasan.RisikoTinggiTon)
	assert.Equal(t, int64(4), got.Ringkasan.RisikoTinggiLot)

	repo.AssertExpectations(t)
}

func TestGetAll_ZeroCapacityDoesNotDivideByZero(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("ListFasilitas", mock.Anything).Return([]domain.FasilitasView{{ID: "CS-X", KapasitasTon: 0, TerisiTon: 5}}, nil)
	repo.On("ListStok", mock.Anything, defaultQuery).Return(domain.StokPage{}, nil)
	repo.On("RingkasanStok", mock.Anything, domain.RisikoTinggiAmbang).Return(domain.RingkasanStok{}, nil)

	got, err := uc.GetAll(context.Background(), domain.StokQuery{})

	require.NoError(t, err)
	assert.Equal(t, 0.0, got.Fasilitas[0].UtilisasiPersen)
}

func TestGetAll_NormalizesQuery(t *testing.T) {
	cases := []struct {
		name string
		in   domain.StokQuery
		want domain.StokQuery
	}{
		{
			name: "kosong memakai bawaan",
			in:   domain.StokQuery{},
			want: domain.StokQuery{Sort: domain.SortSisaMasaSimpan, Page: 1, Limit: DefaultLimit},
		},
		{
			name: "halaman dan limit di bawah batas",
			in:   domain.StokQuery{Page: -3, Limit: -1},
			want: domain.StokQuery{Sort: domain.SortSisaMasaSimpan, Page: 1, Limit: DefaultLimit},
		},
		{
			name: "limit dibatasi maksimum",
			in:   domain.StokQuery{Page: 2, Limit: 5000},
			want: domain.StokQuery{Sort: domain.SortSisaMasaSimpan, Page: 2, Limit: MaxLimit},
		},
		{
			name: "teks dipangkas dan nilai sah dipertahankan",
			in:   domain.StokQuery{ColdStorageID: " CS-01 ", Search: "  cabai  ", Sort: domain.SortVolume, Desc: true, Page: 3, Limit: 25},
			want: domain.StokQuery{ColdStorageID: "CS-01", Search: "cabai", Sort: domain.SortVolume, Desc: true, Page: 3, Limit: 25},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockRepository)
			uc := newTestUseCase(repo)
			expectEmptyOverview(repo, tc.want)

			_, err := uc.GetAll(context.Background(), tc.in)

			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

func TestGetAll_FasilitasError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("ListFasilitas", mock.Anything).Return([]domain.FasilitasView{}, assert.AnError)

	_, err := uc.GetAll(context.Background(), domain.StokQuery{})

	assert.ErrorIs(t, err, assert.AnError)
	repo.AssertNotCalled(t, "ListStok", mock.Anything, mock.Anything)
}

func TestGetAll_StokError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("ListFasilitas", mock.Anything).Return([]domain.FasilitasView{}, nil)
	repo.On("ListStok", mock.Anything, defaultQuery).Return(domain.StokPage{}, assert.AnError)

	_, err := uc.GetAll(context.Background(), domain.StokQuery{})

	assert.ErrorIs(t, err, assert.AnError)
	repo.AssertExpectations(t)
}

func TestGetAll_RingkasanError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("ListFasilitas", mock.Anything).Return([]domain.FasilitasView{}, nil)
	repo.On("ListStok", mock.Anything, defaultQuery).Return(domain.StokPage{}, nil)
	repo.On("RingkasanStok", mock.Anything, domain.RisikoTinggiAmbang).Return(domain.RingkasanStok{}, assert.AnError)

	_, err := uc.GetAll(context.Background(), domain.StokQuery{})

	assert.ErrorIs(t, err, assert.AnError)
	repo.AssertExpectations(t)
}

// ── Helper Tests ──────────────────────────────────────────────────────

func TestSisaHari(t *testing.T) {
	cases := []struct {
		name string
		exp  time.Time
		want int
	}{
		{"tepat lima hari", fixedNow.Add(5 * 24 * time.Hour), 5},
		{"lima hari kurang sedikit dibulatkan ke atas", fixedNow.Add(5*24*time.Hour - time.Minute), 5},
		{"lima hari lebih sedikit dibulatkan ke atas", fixedNow.Add(5*24*time.Hour + time.Minute), 6},
		{"hitungan jam terbaca satu hari", fixedNow.Add(3 * time.Hour), 1},
		{"tepat sekarang", fixedNow, 0},
		{"sudah lewat", fixedNow.Add(-48 * time.Hour), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sisaHari(fixedNow, tc.exp))
		})
	}
}

func TestTotalPages(t *testing.T) {
	assert.Equal(t, 0, totalPages(0, 10))
	assert.Equal(t, 1, totalPages(1, 10))
	assert.Equal(t, 1, totalPages(10, 10))
	assert.Equal(t, 2, totalPages(11, 10))
	assert.Equal(t, 3, totalPages(23, 10))
	assert.Equal(t, 0, totalPages(5, 0), "limit tak sah tidak menyebabkan pembagian nol")
}

func TestRound1(t *testing.T) {
	assert.Equal(t, 172.7, round1(172.70000000000002))
	assert.Equal(t, 86.8, round1(86.75))
	assert.Equal(t, 0.0, round1(0.04))
}

// ── BulkAction Tests ──────────────────────────────────────────────────

func TestBulkAction_RencanakanDistribusi(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{"S1", "S2"}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkRencanakanDistribusi).Return(domain.BulkResult{Berhasil: 2, VolumeTon: 65.89999999999999}, nil)
	repo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil)

	out, err := uc.BulkAction(context.Background(), BulkInput{Action: "rencanakan_distribusi", IDs: ids})

	require.NoError(t, err)
	assert.Equal(t, BulkOutcome{Berhasil: 2, Gagal: 0, VolumeTon: 65.9}, out)
	repo.AssertExpectations(t)
}

func TestBulkAction_RekomendasiOlahan(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	ids := []string{"S1"}
	repo.On("BulkAction", mock.Anything, ids, domain.BulkRekomendasiOlahan).Return(domain.BulkResult{Berhasil: 1, VolumeTon: 12.9}, nil)
	repo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil)

	out, err := uc.BulkAction(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: ids})

	require.NoError(t, err)
	assert.Equal(t, 1, out.Berhasil)
	repo.AssertExpectations(t)
}

func TestBulkAction_InvalidAction(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	_, err := uc.BulkAction(context.Background(), BulkInput{Action: "hapus", IDs: []string{"S1"}})

	assert.ErrorIs(t, err, ErrInvalidAction)
}

func TestBulkAction_NoIDs(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	for _, ids := range [][]string{nil, {}, {"", "  "}} {
		_, err := uc.BulkAction(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: ids})
		assert.ErrorIs(t, err, ErrNoIDs)
	}
}

func TestBulkAction_TooManyIDs(t *testing.T) {
	uc := newTestUseCase(new(mockRepository))

	ids := make([]string, MaxBulkIDs+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("S%d", i)
	}

	_, err := uc.BulkAction(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: ids})

	assert.ErrorIs(t, err, ErrTooManyIDs)
}

func TestBulkAction_DeduplicatesAndCountsMissingAsFailed(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	// Hanya id unik, tak kosong, yang diteruskan ke repository.
	repo.On("BulkAction", mock.Anything, []string{"S1", "S2", "S3"}, domain.BulkRekomendasiOlahan).
		Return(domain.BulkResult{Berhasil: 2, VolumeTon: 40}, nil)
	repo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(nil)

	out, err := uc.BulkAction(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: []string{"S1", "S2", "S1", " ", "S3"}})

	require.NoError(t, err)
	assert.Equal(t, 2, out.Berhasil)
	assert.Equal(t, 1, out.Gagal, "S3 tidak ditemukan")
	repo.AssertExpectations(t)
}

func TestBulkAction_RecordsAudit(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("BulkAction", mock.Anything, []string{"S1", "S2"}, domain.BulkRencanakanDistribusi).
		Return(domain.BulkResult{Berhasil: 2, VolumeTon: 65.9}, nil)

	var logged *domain.AuditLog
	repo.On("CreateAuditLog", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { logged = args.Get(1).(*domain.AuditLog) }).
		Return(nil)

	_, err := uc.BulkAction(context.Background(), BulkInput{
		Action: "rencanakan_distribusi", IDs: []string{"S1", "S2"}, ActorID: "manajer_program", IP: "10.0.0.7",
	})

	require.NoError(t, err)
	require.NotNil(t, logged)
	assert.NotEmpty(t, logged.ID)
	assert.Equal(t, "BULK_rencanakan_distribusi", logged.Aksi)
	assert.Equal(t, "stok_cold_storage", logged.Entitas)
	assert.Equal(t, "2 lot", logged.EntitasID)
	assert.Equal(t, "BERHASIL", logged.Hasil)
	assert.Equal(t, fixedNow, logged.Waktu)
	require.NotNil(t, logged.AktorID)
	assert.Equal(t, "manajer_program", *logged.AktorID)
	require.NotNil(t, logged.IP)
	assert.Equal(t, "10.0.0.7", *logged.IP)
	require.NotNil(t, logged.Metadata)
	assert.JSONEq(t, `{"ids":["S1","S2"],"gagal":0,"volume_ton":65.9}`, *logged.Metadata)
}

func TestBulkAction_AuditWithoutActorAndAllMissing(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("BulkAction", mock.Anything, []string{"X"}, domain.BulkRekomendasiOlahan).Return(domain.BulkResult{}, nil)

	var logged *domain.AuditLog
	repo.On("CreateAuditLog", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { logged = args.Get(1).(*domain.AuditLog) }).
		Return(nil)

	out, err := uc.BulkAction(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: []string{"X"}})

	require.NoError(t, err)
	assert.Equal(t, BulkOutcome{Berhasil: 0, Gagal: 1}, out)
	require.NotNil(t, logged)
	assert.Equal(t, "GAGAL", logged.Hasil)
	assert.Nil(t, logged.AktorID)
	assert.Nil(t, logged.IP)
}

func TestBulkAction_AuditFailureDoesNotFailAction(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("BulkAction", mock.Anything, []string{"S1"}, domain.BulkRekomendasiOlahan).Return(domain.BulkResult{Berhasil: 1, VolumeTon: 5}, nil)
	repo.On("CreateAuditLog", mock.Anything, mock.Anything).Return(assert.AnError)

	out, err := uc.BulkAction(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: []string{"S1"}})

	require.NoError(t, err)
	assert.Equal(t, 1, out.Berhasil)
}

func TestBulkAction_RepoError(t *testing.T) {
	repo := new(mockRepository)
	uc := newTestUseCase(repo)

	repo.On("BulkAction", mock.Anything, []string{"S1"}, domain.BulkRekomendasiOlahan).Return(domain.BulkResult{}, assert.AnError)

	_, err := uc.BulkAction(context.Background(), BulkInput{Action: "rekomendasi_olahan", IDs: []string{"S1"}})

	assert.ErrorIs(t, err, assert.AnError)
	repo.AssertNotCalled(t, "CreateAuditLog", mock.Anything, mock.Anything)
}
