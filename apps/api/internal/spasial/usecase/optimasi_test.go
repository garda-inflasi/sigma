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
}

func TestHitung_CachedDistanceWinsOverEstimate(t *testing.T) {
	repo := &fakeOptimasiRepo{
		neraca:  []domain.Neraca{neracaUji("A", 10), neracaUji("B", -10)},
		markers: petaUji(),
		jarak:   []domain.JarakAntarWilayah{{AsalKode: "A", TujuanKode: "B", JarakKm: 500, JamTempuh: 9, Sumber: "OSRM"}},
	}
	opt, _ := newOpt(repo)

	out, err := opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT", TarifTonKm: 1000})

	require.NoError(t, err)
	assert.Equal(t, 500.0, out.Rute[0].JarakKm)
	assert.Equal(t, 9.0, out.Rute[0].JamTempuh)
	assert.EqualValues(t, 10*500*1000, out.TotalBiaya)
}

func TestHitung_TravelTimeLimitLeavesDemandUnmet(t *testing.T) {
	repo := &fakeOptimasiRepo{neraca: []domain.Neraca{neracaUji("A", 10), neracaUji("C", -10)}, markers: petaUji()}
	opt, _ := newOpt(repo)

	// A-C = 222,4 km x 1,35 = 300,2 km = 6,7 jam; batas 5 jam melarangnya.
	out, err := opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT", MaksJamTempuh: 5})

	require.NoError(t, err)
	assert.Empty(t, out.Rute)
	assert.Equal(t, []Sisa{{Kode: "C", Nama: "Wilayah C", Ton: 10}}, out.BelumTerpenuhi)
	assert.Equal(t, []Sisa{{Kode: "A", Nama: "Wilayah A", Ton: 10}}, out.SurplusSisa)
}

func TestHitung_AdjustmentsSimulateScenarioWithoutSaving(t *testing.T) {
	repo := &fakeOptimasiRepo{neraca: []domain.Neraca{neracaUji("A", 30), neracaUji("B", -20)}, markers: petaUji()}
	opt, _ := newOpt(repo)

	// Panen A gagal: surplus 30 turun 25 ton, tersisa 5 ton untuk defisit B 20 ton.
	out, err := opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT", Penyesuaian: []Penyesuaian{{KabupatenKode: "A", Ton: -25}}})

	require.NoError(t, err)
	assert.Equal(t, 5.0, out.TotalVolumeTon)
	assert.Equal(t, []Sisa{{Kode: "B", Nama: "Wilayah B", Ton: 15}}, out.BelumTerpenuhi)

	// Penyesuaian dapat membalik posisi wilayah: B defisit 20 + 30 ton = surplus 10.
	out, err = opt.Hitung(t.Context(), OptimasiInput{KomoditasKode: "CBM-KRT", Penyesuaian: []Penyesuaian{{KabupatenKode: "B", Ton: 30}}})
	require.NoError(t, err)
	assert.Empty(t, out.Rute)
	assert.Len(t, out.SurplusSisa, 2)
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
