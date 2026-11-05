package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/audit/audittest"
	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOptimasiRepo struct {
	neraca  []domain.Neraca
	markers []domain.Marker
	jarak   []domain.JarakAntarWilayah

	query   domain.NeracaQuery
	rencana *domain.OptimasiRute
	rute    []domain.RuteDistribusi
	cache   []domain.JarakAntarWilayah
	err     error
	saveErr error
}

func (f *fakeOptimasiRepo) ListMarker(context.Context) ([]domain.Marker, error) {
	return f.markers, f.err
}

func (f *fakeOptimasiRepo) ListNeraca(_ context.Context, q domain.NeracaQuery) ([]domain.Neraca, error) {
	f.query = q
	return f.neraca, f.err
}

func (f *fakeOptimasiRepo) ListJarak(context.Context) ([]domain.JarakAntarWilayah, error) {
	return f.jarak, f.err
}

func (f *fakeOptimasiRepo) SimpanOptimasi(_ context.Context, r domain.OptimasiRute, rute []domain.RuteDistribusi, jarak []domain.JarakAntarWilayah) error {
	f.rencana, f.rute, f.cache = &r, rute, jarak
	return f.saveErr
}

func (f *fakeOptimasiRepo) ListOptimasi(context.Context, int) ([]domain.RencanaOptimasi, error) {
	return nil, f.err
}

var periodeUji = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func neracaUji(kode string, ton float64) domain.Neraca {
	return domain.Neraca{
		NeracaWilayah: domain.NeracaWilayah{KomoditasKode: "CBM-KRT", KabupatenKode: kode, Periode: periodeUji, SurplusDefisitTon: ton},
		KomoditasNama: "Cabai merah keriting", KabupatenNama: "Wilayah " + kode,
	}
}

// Tiga wilayah segaris di khatulistiwa pada bujur 0, 1, dan 2 derajat (1 derajat = 111,195 km).
func petaUji() []domain.Marker {
	m := func(kode string, lng float64) domain.Marker {
		return domain.Marker{Wilayah: domain.Wilayah{Kode: kode, Nama: kode, Lat: 0, Lng: lng}}
	}
	return []domain.Marker{m("A", 0), m("B", 1), m("C", 2)}
}

func newOpt(repo *fakeOptimasiRepo) (*Optimasi, *audittest.Recorder) {
	rec := &audittest.Recorder{}
	return NewOptimasi(repo, logger.NewDefault(), rec).WithClock(func() time.Time { return time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC) }), rec
}

func TestHitung_PlansFromNetPositionsUsingEstimatedRoadDistance(t *testing.T) {
	// A surplus 30, B defisit 20, C defisit 20: B (111,2 km lurus) lebih dekat daripada C (222,4 km).
	repo := &fakeOptimasiRepo{neraca: []domain.Neraca{neracaUji("A", 30), neracaUji("B", -20), neracaUji("C", -20)}, markers: petaUji()}
	opt, _ := newOpt(repo)

	out, err := opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: " CBM-KRT ", FaktorJalan: 1})

	require.NoError(t, err)
	assert.Equal(t, "CBM-KRT", repo.query.KomoditasKode, "kode dirapikan")
	assert.True(t, repo.query.Periode.IsZero(), "tanpa periode: terbaru")
	assert.Equal(t, periodeUji, out.Periode)
	assert.Equal(t, "Cabai merah keriting", out.KomoditasNama)
	require.Len(t, out.Rute, 2)
	assert.Equal(t, 20.0, out.Rute[0].VolumeTon, "B terpenuhi penuh")
	assert.Equal(t, "B", out.Rute[0].TujuanKode)
	assert.Equal(t, 111.2, out.Rute[0].JarakKm, "jarak dibulatkan 0,1 km")
	assert.Equal(t, 2.5, out.Rute[0].JamTempuh, "111,2 km / 45 km per jam")
	assert.Equal(t, 10.0, out.Rute[1].VolumeTon)
	assert.Equal(t, 30.0, out.TotalVolumeTon)
	assert.Equal(t, []Sisa{{Kode: "C", Nama: "Wilayah C", Ton: 10}}, out.BelumTerpenuhi)
	assert.Empty(t, out.SurplusSisa)
	// 20 ton x 111,2 km x Rp4.200 + 10 ton x 222,4 km x Rp4.200
	assert.EqualValues(t, 20*111.2*4200+10*222.4*4200, out.TotalBiaya)
	assert.Empty(t, out.OptimasiID, "tidak disimpan")
	assert.Nil(t, repo.rencana)
	require.NotNil(t, out.BiayaPembanding, "satu pemasok: rencana serakah menyalurkan volume yang sama")
	assert.Equal(t, out.TotalBiaya, *out.BiayaPembanding)
	assert.Equal(t, 0.0, *out.PenghematanPersen, "tidak ada yang bisa dihemat")
}

func TestHitung_ReportsSavingsAgainstGreedyComparatorAndStoresIt(t *testing.T) {
	// Contoh klasik: serakah 10x1 + 10x100 = 1010 ton-km; optimum silang 10x2 + 10x2 = 40 ton-km.
	m := func(kode string, lng float64) domain.Marker {
		return domain.Marker{Wilayah: domain.Wilayah{Kode: kode, Nama: kode, Lng: lng}}
	}
	cache := func(a, b string, km float64) domain.JarakAntarWilayah {
		return domain.JarakAntarWilayah{AsalKode: a, TujuanKode: b, JarakKm: km, JamTempuh: km / 50, Sumber: "OSRM"}
	}
	repo := &fakeOptimasiRepo{
		neraca:  []domain.Neraca{neracaUji("S1", 10), neracaUji("S2", 10), neracaUji("D1", -10), neracaUji("D2", -10)},
		markers: []domain.Marker{m("S1", 0), m("S2", 1), m("D1", 2), m("D2", 3)},
		jarak:   []domain.JarakAntarWilayah{cache("S1", "D1", 1), cache("S1", "D2", 2), cache("S2", "D1", 2), cache("S2", "D2", 100)},
	}
	opt, _ := newOpt(repo)

	out, err := opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT", TarifTonKm: 1000, Simpan: true})

	require.NoError(t, err)
	assert.EqualValues(t, 40*1000, out.TotalBiaya)
	require.NotNil(t, out.BiayaPembanding)
	assert.EqualValues(t, 1010*1000, *out.BiayaPembanding)
	assert.Equal(t, 96.0, *out.PenghematanPersen, "100 × (1010 − 40) ÷ 1010 = 96,04%")
	require.NotNil(t, repo.rencana.BiayaPembanding)
	assert.EqualValues(t, 1010*1000, *repo.rencana.BiayaPembanding, "disimpan agar KPI efisiensi dapat dihitung")
}

func TestHitung_NoComparatorWhenGreedyShipsLessVolumeThanOptimum(t *testing.T) {
	// S2 hanya dapat menjangkau D1 dalam batas waktu. Serakah memakai S1 untuk D1 (pasangan
	// terdekat) sehingga D2 tak terlayani: 10 ton. Optimum S2->D1 + S1->D2 menyalurkan 20 ton.
	// Biaya keduanya tidak sebanding, jadi pembanding tidak dilaporkan.
	m := func(kode string, lng float64) domain.Marker {
		return domain.Marker{Wilayah: domain.Wilayah{Kode: kode, Nama: kode, Lng: lng}}
	}
	cache := func(a, b string, km, jam float64) domain.JarakAntarWilayah {
		return domain.JarakAntarWilayah{AsalKode: a, TujuanKode: b, JarakKm: km, JamTempuh: jam, Sumber: "OSRM"}
	}
	repo := &fakeOptimasiRepo{
		neraca:  []domain.Neraca{neracaUji("S1", 10), neracaUji("S2", 10), neracaUji("D1", -10), neracaUji("D2", -10)},
		markers: []domain.Marker{m("S1", 0), m("S2", 1), m("D1", 2), m("D2", 3)},
		jarak: []domain.JarakAntarWilayah{
			cache("S1", "D1", 1, 1), cache("S1", "D2", 5, 1), cache("S2", "D1", 2, 1), cache("S2", "D2", 60, 9),
		},
	}
	opt, _ := newOpt(repo)

	out, err := opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT", MaksJamTempuh: 2, Simpan: true})

	require.NoError(t, err)
	assert.Equal(t, 20.0, out.TotalVolumeTon)
	assert.Nil(t, out.BiayaPembanding)
	assert.Nil(t, out.PenghematanPersen)
	assert.Nil(t, repo.rencana.BiayaPembanding)
}

func TestHitung_SaveWritesPlanRoutesCacheAndAudit(t *testing.T) {
	repo := &fakeOptimasiRepo{
		neraca:  []domain.Neraca{neracaUji("A", 30), neracaUji("B", -20), neracaUji("C", -10)},
		markers: petaUji(),
		jarak:   []domain.JarakAntarWilayah{{AsalKode: "A", TujuanKode: "C", JarakKm: 300, JamTempuh: 6, Sumber: "OSRM"}},
	}
	opt, rec := newOpt(repo)

	out, err := opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT", Simpan: true, ActorID: "manajer_program"})

	require.NoError(t, err)
	require.NotNil(t, repo.rencana)
	assert.Equal(t, out.OptimasiID, repo.rencana.ID)
	assert.Regexp(t, `^OPT-[0-9A-F]{12}$`, out.OptimasiID)
	assert.Equal(t, "manajer_program", *repo.rencana.DibuatOleh)
	assert.Equal(t, 30.0, repo.rencana.TotalVolumeTon)
	assert.JSONEq(t, `{"tarif_ton_km":4200,"faktor_jalan":1.35,"kecepatan_km":45,"maks_jam_tempuh":0}`, repo.rencana.Parameter)

	require.Len(t, repo.rute, 2)
	for _, r := range repo.rute {
		assert.Equal(t, "USULAN", r.Status)
		assert.Equal(t, "OPTIMASI", r.Sumber)
		assert.Equal(t, out.OptimasiID, *r.OptimasiID)
		assert.Equal(t, "CBM-KRT", r.KomoditasKode)
	}
	assert.Regexp(t, `^RT-OPT-[0-9A-F]{12}-01$`, repo.rute[0].ID)

	require.Len(t, repo.cache, 1, "hanya pasangan hasil estimasi (A-B) yang di-cache; A-C sudah ada di cache")
	assert.Equal(t, "B", repo.cache[0].TujuanKode)
	assert.Equal(t, "ESTIMASI", repo.cache[0].Sumber)

	last, ok := rec.Last()
	require.True(t, ok)
	assert.Equal(t, "OPTIMASI_RUTE", last.Aksi)
	assert.Equal(t, out.OptimasiID, last.EntitasID)
	assert.Equal(t, "manajer_program", last.AktorID)
}

func TestHitung_RejectsInvalidInputBeforeTouchingTheDatabase(t *testing.T) {
	repo := &fakeOptimasiRepo{neraca: []domain.Neraca{neracaUji("A", 10), neracaUji("B", -10)}, markers: petaUji()}
	opt, _ := newOpt(repo)
	dasar := OptimasiInput{KomoditasKode: "CBM-KRT"}

	with := func(f func(*OptimasiInput)) OptimasiInput { in := dasar; f(&in); return in }
	cases := map[string]OptimasiInput{
		"tarif negatif":             with(func(i *OptimasiInput) { i.TarifTonKm = -1 }),
		"tarif terlalu besar":       with(func(i *OptimasiInput) { i.TarifTonKm = 1e9 }),
		"faktor jalan < 1":          with(func(i *OptimasiInput) { i.FaktorJalan = 0.5 }),
		"faktor jalan > 3":          with(func(i *OptimasiInput) { i.FaktorJalan = 4 }),
		"batas waktu negatif":       with(func(i *OptimasiInput) { i.MaksJamTempuh = -1 }),
		"batas waktu besar":         with(func(i *OptimasiInput) { i.MaksJamTempuh = 1000 }),
		"penyesuaian ganda":         with(func(i *OptimasiInput) { i.Penyesuaian = []Penyesuaian{{"A", 1}, {"A", 2}} }),
		"penyesuaian tanpa kode":    with(func(i *OptimasiInput) { i.Penyesuaian = []Penyesuaian{{"", 1}} }),
		"penyesuaian terlalu besar": with(func(i *OptimasiInput) { i.Penyesuaian = []Penyesuaian{{"A", 1e12}} }),
		"wilayah tanpa neraca":      with(func(i *OptimasiInput) { i.Penyesuaian = []Penyesuaian{{"Z", 1}} }),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := opt.Hitung(t.Context(), in)

			var input *InputError
			assert.ErrorAs(t, err, &input)
		})
	}

	_, err := opt.Hitung(t.Context(), OptimasiInput{})
	assert.ErrorIs(t, err, ErrKomoditasKosong)
	_, err = opt.Hitung(t.Context(), with(func(i *OptimasiInput) { i.Simpan = true; i.Penyesuaian = []Penyesuaian{{"A", 1}} }))
	assert.ErrorIs(t, err, ErrSkenarioTidakSimpan)
	assert.Nil(t, repo.rencana)
}

func TestHitung_NoBalanceAndRepositoryErrors(t *testing.T) {
	opt, _ := newOpt(&fakeOptimasiRepo{markers: petaUji()})
	_, err := opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT"})
	assert.ErrorIs(t, err, ErrNeracaKosong)

	boom := errors.New("db mati")
	opt, _ = newOpt(&fakeOptimasiRepo{err: boom})
	_, err = opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT"})
	assert.ErrorIs(t, err, boom)

	opt, rec := newOpt(&fakeOptimasiRepo{
		neraca: []domain.Neraca{neracaUji("A", 10), neracaUji("B", -10)}, markers: petaUji(), saveErr: boom,
	})
	_, err = opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT", Simpan: true})
	assert.ErrorIs(t, err, boom)
	_, ok := rec.Last()
	assert.False(t, ok, "gagal menyimpan tidak menulis audit")
}
