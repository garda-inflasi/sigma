package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/eksekutif/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo mengembalikan data tetap; Err (bila diisi) menggagalkan setiap metode.
type fakeRepo struct {
	inflasi    *domain.InflasiBulanan
	seri       []domain.InflasiBulanan
	target     *domain.TargetInflasi
	penyumbang []domain.Penyumbang
	neraca     []domain.NeracaItem
	fiskal     []domain.RuangFiskal
	ews        map[string]int
	intervensi []domain.HitungIntervensi
	Err        error

	fiskalDipanggil bool
}

func (f *fakeRepo) InflasiTerbaru(context.Context, string) (*domain.InflasiBulanan, error) {
	return f.inflasi, f.Err
}
func (f *fakeRepo) SeriInflasi(context.Context, string, time.Time, int) ([]domain.InflasiBulanan, error) {
	return f.seri, f.Err
}
func (f *fakeRepo) TargetBerlaku(context.Context, int) (*domain.TargetInflasi, error) {
	return f.target, f.Err
}
func (f *fakeRepo) Penyumbang(context.Context, string, time.Time, int) ([]domain.Penyumbang, error) {
	return f.penyumbang, f.Err
}
func (f *fakeRepo) NeracaTerbaru(context.Context, string) ([]domain.NeracaItem, error) {
	return f.neraca, f.Err
}
func (f *fakeRepo) RuangFiskalTerbaru(context.Context) ([]domain.RuangFiskal, error) {
	f.fiskalDipanggil = true
	return f.fiskal, f.Err
}
func (f *fakeRepo) HitungEWS(context.Context) (map[string]int, error) { return f.ews, f.Err }
func (f *fakeRepo) HitungIntervensi(context.Context) ([]domain.HitungIntervensi, error) {
	return f.intervensi, f.Err
}

var okt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func repoLengkap(sumber string) *fakeRepo {
	return &fakeRepo{
		inflasi: &domain.InflasiBulanan{Periode: okt, InflasiYoy: 2.84, InflasiMtm: 0.41, InflasiYtd: 1.92, IHK: 110, Sumber: sumber},
		seri: []domain.InflasiBulanan{
			{Periode: okt.AddDate(0, -1, 0), InflasiYoy: 2.63}, {Periode: okt, InflasiYoy: 2.84},
		},
		target: &domain.TargetInflasi{Tahun: 2026, BatasBawah: 1.5, BatasAtas: 3.5},
		penyumbang: []domain.Penyumbang{
			{Kode: "TMT", Nama: "Tomat", Andil: -0.02}, {Kode: "CBM-KRT", Nama: "Cabai merah", Andil: 0.18},
			{Kode: "BWM", Nama: "Bawang merah", Andil: 0.06},
		},
		neraca: []domain.NeracaItem{{KomoditasKode: "BRS-MED", Nama: "Beras", Periode: okt, ProduksiTon: 100, KonsumsiTon: 50, Sumber: sumber}},
		fiskal: []domain.RuangFiskal{
			{Instrumen: "BTT", Pagu: 120, Realisasi: 30, PerTanggal: okt, Sumber: sumber},
			{Instrumen: "Operasi pasar", Pagu: 80, Realisasi: 20, PerTanggal: okt, Sumber: sumber},
		},
		ews: map[string]int{"KRITIS": 2, "SIAGA": 3},
		intervensi: []domain.HitungIntervensi{
			{Siklus: "tekanan", Aktif: 4, Selesai: 1}, {Siklus: "surplus", Aktif: 4, Selesai: 0},
		},
	}
}

func TestRingkasan_AssemblesEveryBlock(t *testing.T) {
	res, err := New(repoLengkap(domain.SumberBPS)).Ringkasan(t.Context(), true)

	require.NoError(t, err)
	require.NotNil(t, res.Inflasi)
	assert.InDelta(t, 2.84, res.Inflasi.Yoy, 1e-9)
	assert.Equal(t, domain.Sasaran{Tahun: 2026, Bawah: 1.5, Atas: 3.5}, res.Inflasi.Sasaran)
	require.Len(t, res.Inflasi.Seri, 2)
	assert.Equal(t, okt, res.Inflasi.Seri[1].Periode, "seri berurutan naik")
	require.NotNil(t, res.Neraca)
	assert.Equal(t, okt, res.Neraca.Periode)
	require.NotNil(t, res.Fiskal)
	assert.False(t, res.Demo, "angka resmi tidak diberi label demo")
}

func TestRingkasan_PenyumbangSortedByAndilDescending(t *testing.T) {
	res, err := New(repoLengkap(domain.SumberBPS)).Ringkasan(t.Context(), true)

	require.NoError(t, err)
	var kode []string
	for _, p := range res.Inflasi.Penyumbang {
		kode = append(kode, p.Kode)
	}
	assert.Equal(t, []string{"CBM-KRT", "BWM", "TMT"}, kode, "yang menaikkan inflasi di atas, yang menurunkan di bawah")
}

func TestRingkasan_DemoFlagWhenAnyBlockIsDemo(t *testing.T) {
	for name, mutasi := range map[string]func(*fakeRepo){
		"inflasi": func(r *fakeRepo) { r.inflasi.Sumber = domain.SumberDemo },
		"neraca":  func(r *fakeRepo) { r.neraca[0].Sumber = domain.SumberDemo },
		"fiskal":  func(r *fakeRepo) { r.fiskal[1].Sumber = domain.SumberDemo },
	} {
		t.Run(name, func(t *testing.T) {
			repo := repoLengkap(domain.SumberBPS)
			mutasi(repo)

			res, err := New(repo).Ringkasan(t.Context(), true)

			require.NoError(t, err)
			assert.True(t, res.Demo)
		})
	}
}

func TestRingkasan_FiskalIsNotLoadedWithoutPermission(t *testing.T) {
	repo := repoLengkap(domain.SumberBPS)

	res, err := New(repo).Ringkasan(t.Context(), false)

	require.NoError(t, err)
	assert.Nil(t, res.Fiskal)
	assert.False(t, repo.fiskalDipanggil, "tanpa izin, data fiskal tidak boleh dibaca")
}

func TestRingkasan_FiskalTotalsAndScore(t *testing.T) {
	res, err := New(repoLengkap(domain.SumberBPS)).Ringkasan(t.Context(), true)

	require.NoError(t, err)
	assert.EqualValues(t, 200, res.Fiskal.Pagu)
	assert.EqualValues(t, 50, res.Fiskal.Realisasi)
	assert.Equal(t, 75, res.Fiskal.Skor)
	assert.Equal(t, domain.FiskalMemadai, res.Fiskal.Status)
}

func TestSkorFiskal(t *testing.T) {
	tests := []struct {
		name            string
		pagu, realisasi int64
		skor            int
		status          string
	}{
		{"sisa 71%", 186, 54, 71, domain.FiskalMemadai},
		{"batas memadai 60", 100, 40, 60, domain.FiskalMemadai},
		{"terbatas 59", 100, 41, 59, domain.FiskalTerbatas},
		{"batas terbatas 30", 100, 70, 30, domain.FiskalTerbatas},
		{"kritis 29", 100, 71, 29, domain.FiskalKritis},
		{"habis", 100, 100, 0, domain.FiskalKritis},
		{"realisasi melebihi pagu tidak negatif", 100, 130, 0, domain.FiskalKritis},
		{"tanpa pagu", 0, 0, 0, domain.FiskalKritis},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			skor, status := domain.SkorFiskal(tc.pagu, tc.realisasi)
			assert.Equal(t, tc.skor, skor)
			assert.Equal(t, tc.status, status)
		})
	}
}

func TestRingkasan_EmptyDatabaseYieldsEmptyDashboardNotAnError(t *testing.T) {
	res, err := New(&fakeRepo{}).Ringkasan(t.Context(), true)

	require.NoError(t, err)
	assert.Nil(t, res.Inflasi)
	assert.Nil(t, res.Neraca)
	assert.Nil(t, res.Fiskal)
	assert.False(t, res.Demo)
	assert.Equal(t, map[string]int{"INFO": 0, "WASPADA": 0, "SIAGA": 0, "KRITIS": 0}, res.EWS)
	assert.Equal(t, "normal", res.SiklusDominan)
	assert.Equal(t, map[string]domain.IntervensiRingkas{"normal": {}, "tekanan": {}, "surplus": {}}, res.Intervensi)
}

func TestRingkasan_InflasiHiddenWithoutApplicableTarget(t *testing.T) {
	repo := repoLengkap(domain.SumberBPS)
	repo.target = nil

	res, err := New(repo).Ringkasan(t.Context(), true)

	require.NoError(t, err)
	assert.Nil(t, res.Inflasi, "koridor sasaran tidak bisa digambar tanpa target")
}

func TestRingkasan_DominantCycleIsTheOneWithMostActiveCards(t *testing.T) {
	tests := []struct {
		name       string
		intervensi []domain.HitungIntervensi
		dominan    string
	}{
		{"terbanyak menang", []domain.HitungIntervensi{{Siklus: "normal", Aktif: 5}, {Siklus: "surplus", Aktif: 2}}, "normal"},
		{"seri: tekanan didahulukan", []domain.HitungIntervensi{{Siklus: "surplus", Aktif: 4}, {Siklus: "tekanan", Aktif: 4}}, "tekanan"},
		{"semua selesai: normal", []domain.HitungIntervensi{{Siklus: "tekanan", Aktif: 0, Selesai: 7}}, "normal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoLengkap(domain.SumberBPS)
			repo.intervensi = tc.intervensi

			res, err := New(repo).Ringkasan(t.Context(), true)

			require.NoError(t, err)
			assert.Equal(t, tc.dominan, res.SiklusDominan)
		})
	}
}

func TestRingkasan_RepositoryErrorsPropagate(t *testing.T) {
	repo := repoLengkap(domain.SumberBPS)
	repo.Err = errors.New("db down")

	_, err := New(repo).Ringkasan(t.Context(), true)

	assert.Error(t, err)
}

func TestRingkasan_UnknownCycleIsAnError(t *testing.T) {
	repo := repoLengkap(domain.SumberBPS)
	repo.intervensi = []domain.HitungIntervensi{{Siklus: "aneh", Aktif: 1}}

	_, err := New(repo).Ringkasan(t.Context(), true)

	assert.Error(t, err)
}
