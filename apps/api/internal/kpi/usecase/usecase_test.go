package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/kpi/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hitungPanggilan mencatat satu panggilan Hitung.
type hitungPanggilan struct {
	Query string
	W     domain.Jendela
}

type simpanPanggilan struct {
	Kode    string
	Periode time.Time
	Nilai   float64
}

type fakeRepo struct {
	defs    []domain.Definisi
	targets []domain.Target
	nilai   []domain.Nilai
	riwayat []domain.Nilai
	// hasil Hitung per query; kunci tak ada = tidak ada dasar data.
	hasilHitung map[string]float64
	// hasil SimpanNilai per kode; bawaan Baru.
	hasilSimpan map[string]domain.HasilSimpan
	Err         error

	hitung []hitungPanggilan
	simpan []simpanPanggilan
}

func (f *fakeRepo) Definisi(context.Context) ([]domain.Definisi, error) { return f.defs, f.Err }
func (f *fakeRepo) DefinisiByKode(_ context.Context, kode string) (*domain.Definisi, error) {
	for i := range f.defs {
		if f.defs[i].Kode == kode {
			return &f.defs[i], nil
		}
	}
	return nil, domain.ErrNotFound
}
func (f *fakeRepo) TargetBerlaku(context.Context, time.Time, *string) ([]domain.Target, error) {
	return f.targets, f.Err
}
func (f *fakeRepo) NilaiBerlaku(context.Context, time.Time, time.Time, *string) ([]domain.Nilai, error) {
	return f.nilai, f.Err
}
func (f *fakeRepo) RiwayatKPI(context.Context, string, *string, int) ([]domain.Nilai, error) {
	return f.riwayat, f.Err
}
func (f *fakeRepo) Hitung(_ context.Context, q string, w domain.Jendela) (*float64, error) {
	f.hitung = append(f.hitung, hitungPanggilan{q, w})
	if f.Err != nil {
		return nil, f.Err
	}
	v, ok := f.hasilHitung[q]
	if !ok {
		return nil, nil
	}
	return &v, nil
}
func (f *fakeRepo) SimpanNilai(_ context.Context, kode string, periode time.Time, nilai float64, _ time.Time) (domain.HasilSimpan, error) {
	f.simpan = append(f.simpan, simpanPanggilan{kode, periode, nilai})
	return f.hasilSimpan[kode], nil
}

func newUC(repo *fakeRepo) (*UseCase, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	return New(repo, rec, logger.NewDefault()), rec
}

func q(s string) *string { return &s }

var (
	sekarang = time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC) // 10.00 WIB
	okt      = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	nov      = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
)

func TestAwalBulan(t *testing.T) {
	assert.Equal(t, nov, AwalBulan(time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC)))
}

func TestJendela_UsesWIBMonthBoundaries(t *testing.T) {
	w := jendela(okt, sekarang)

	assert.True(t, w.Mulai.Equal(time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)), "1 Oktober 00.00 WIB = 30 September 17.00 UTC")
	assert.True(t, w.Akhir.Equal(time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC)))
	assert.True(t, w.AsOf.Equal(w.Akhir), "bulan lampau dinilai pada akhir bulan")

	berjalan := jendela(nov, sekarang)
	assert.True(t, berjalan.AsOf.Equal(sekarang), "bulan berjalan dinilai pada sekarang, bukan akhir bulan")
}

func TestHitung_SkipsManualKPIsAndFutureMonths(t *testing.T) {
	repo := &fakeRepo{
		defs: []domain.Definisi{
			{Kode: "OTO", SumberQuery: q("utilisasi_cold_storage")},
			{Kode: "MANUAL"}, // tanpa penghitung: menunggu model
		},
		hasilHitung: map[string]float64{"utilisasi_cold_storage": 80},
	}
	uc, _ := newUC(repo)

	lap, err := uc.Hitung(t.Context(), okt, time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC), sekarang)

	require.NoError(t, err)
	assert.Equal(t, Laporan{Baru: 2}, lap, "Oktober dan November saja; Desember 2026 dan seterusnya belum terjadi")
	for _, s := range repo.simpan {
		assert.Equal(t, "OTO", s.Kode, "KPI tanpa penghitung tidak boleh diisi")
	}
}

func TestHitung_NoDataIsSkippedNotZero(t *testing.T) {
	repo := &fakeRepo{defs: []domain.Definisi{{Kode: "OTO", SumberQuery: q("ketepatan_data")}}}
	uc, rec := newUC(repo)

	lap, err := uc.Hitung(t.Context(), okt, okt, sekarang)

	require.NoError(t, err)
	assert.Equal(t, Laporan{TanpaDasar: 1}, lap)
	assert.Empty(t, repo.simpan, "tanpa dasar data, tidak ada nilai (apalagi 0) yang ditulis")
	assert.Empty(t, rec.Entries(), "tidak ada perubahan, tidak ada audit")
}

func TestHitung_CountsOutcomesAndAuditsOnlyChanges(t *testing.T) {
	defs := []domain.Definisi{
		{Kode: "A", SumberQuery: q("a")}, {Kode: "B", SumberQuery: q("b")}, {Kode: "C", SumberQuery: q("c")},
	}
	repo := &fakeRepo{
		defs:        defs,
		hasilHitung: map[string]float64{"a": 1, "b": 2, "c": 3},
		hasilSimpan: map[string]domain.HasilSimpan{"A": domain.Baru, "B": domain.Direvisi, "C": domain.Tetap},
	}
	uc, rec := newUC(repo)

	lap, err := uc.Hitung(t.Context(), okt, okt, sekarang)

	require.NoError(t, err)
	assert.Equal(t, Laporan{Baru: 1, Direvisi: 1, Tetap: 1}, lap)
	entry, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "HITUNG_KPI", entry.Aksi)
	assert.Equal(t, audit.HasilBerhasil, entry.Hasil)
	assert.Equal(t, "2026-10..2026-10", entry.EntitasID)

	repo.hasilSimpan = map[string]domain.HasilSimpan{"A": domain.Tetap, "B": domain.Tetap, "C": domain.Tetap}
	_, err = uc.Hitung(t.Context(), okt, okt, sekarang)
	require.NoError(t, err)
	assert.Len(t, rec.Entries(), 1, "putaran tanpa perubahan tidak menambah audit")
}

func TestHitung_PassesWindowToCalculator(t *testing.T) {
	repo := &fakeRepo{defs: []domain.Definisi{{Kode: "A", SumberQuery: q("a")}}}
	uc, _ := newUC(repo)

	_, err := uc.Hitung(t.Context(), time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), okt, sekarang)

	require.NoError(t, err)
	require.Len(t, repo.hitung, 1, "tanggal dinormalkan ke bulan")
	assert.True(t, repo.hitung[0].W.Mulai.Equal(time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)))
}

func TestHitung_ErrorsPropagate(t *testing.T) {
	repo := &fakeRepo{defs: []domain.Definisi{{Kode: "A", SumberQuery: q("a")}}, Err: errors.New("db down")}
	uc, _ := newUC(repo)

	_, err := uc.Hitung(t.Context(), okt, okt, sekarang)

	assert.Error(t, err)
}

func nilaiPer(kode string, bulan int, v float64) domain.Nilai {
	return domain.Nilai{KpiKode: kode, Periode: time.Date(2026, time.Month(bulan), 1, 0, 0, 0, 0, time.UTC), Nilai: v, Versi: 1}
}

func TestDaftar_AssemblesStatusTrendAndWaiting(t *testing.T) {
	repo := &fakeRepo{
		defs: []domain.Definisi{
			{Kode: "UTIL", ArahBaik: domain.ArahNaik, SumberQuery: q("utilisasi_cold_storage")},
			{Kode: "MAPE", ArahBaik: domain.ArahTurun},
			{Kode: "TANPA", ArahBaik: domain.ArahNaik, SumberQuery: q("ketepatan_data")},
		},
		targets: []domain.Target{
			{KpiKode: "UTIL", Target: 75, AmbangWaspada: new(67.5)},
			{KpiKode: "MAPE", Target: 5.4},
		},
		nilai: []domain.Nilai{
			nilaiPer("UTIL", 3, 50), nilaiPer("UTIL", 4, 52), nilaiPer("UTIL", 5, 55), nilaiPer("UTIL", 6, 60),
			nilaiPer("UTIL", 7, 65), nilaiPer("UTIL", 8, 70), nilaiPer("UTIL", 9, 72), nilaiPer("UTIL", 10, 82),
			nilaiPer("TANPA", 10, 90),
		},
	}
	uc, _ := newUC(repo)

	res, err := uc.Daftar(t.Context(), okt, nil)

	require.NoError(t, err)
	require.Len(t, res, 3)
	util, mape, tanpa := res[0], res[1], res[2]

	assert.Equal(t, domain.StatusTercapai, util.Status)
	assert.True(t, util.Otomatis)
	require.NotNil(t, util.Nilai)
	assert.InDelta(t, 82, util.Nilai.Nilai, 1e-9, "nilai terbaru")
	require.Len(t, util.Tren, titikTren)
	assert.InDelta(t, 55, util.Tren[0].Nilai, 1e-9, "tren = 6 periode terakhir (Mei–Okt), urut naik")
	assert.InDelta(t, 82, util.Tren[5].Nilai, 1e-9)

	assert.Equal(t, domain.StatusMenungguData, mape.Status)
	assert.False(t, mape.Otomatis)
	assert.Nil(t, mape.Nilai)
	assert.Empty(t, mape.Tren)

	assert.Equal(t, domain.StatusTanpaTarget, tanpa.Status)
}

func TestRiwayat_ReturnsDefinitionTargetAndHistory(t *testing.T) {
	repo := &fakeRepo{
		defs:    []domain.Definisi{{Kode: "UTIL", ArahBaik: domain.ArahNaik}},
		targets: []domain.Target{{KpiKode: "UTIL", Target: 75}, {KpiKode: "LAIN", Target: 1}},
		riwayat: []domain.Nilai{nilaiPer("UTIL", 9, 72), nilaiPer("UTIL", 10, 82)},
	}
	uc, _ := newUC(repo)

	ind, riwayat, err := uc.Riwayat(t.Context(), "UTIL", okt, nil)

	require.NoError(t, err)
	assert.Len(t, riwayat, 2)
	require.NotNil(t, ind.Target)
	assert.InDelta(t, 75, ind.Target.Target, 1e-9, "target milik KPI lain tidak ikut")
	assert.Equal(t, domain.StatusTercapai, ind.Status)
}

func TestRiwayat_UnknownKPI(t *testing.T) {
	uc, _ := newUC(&fakeRepo{})

	_, _, err := uc.Riwayat(t.Context(), "TAK-ADA", okt, nil)

	assert.ErrorIs(t, err, domain.ErrNotFound)
}
