package domain

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tabel membuat JarakFunc dari peta "asal>tujuan" -> km; waktu = km / 50 jam.
func tabel(km map[[2]string]float64) JarakFunc {
	return func(asal, tujuan string) Jalur {
		d := km[[2]string{asal, tujuan}]
		return Jalur{JarakKm: d, JamTempuh: d / 50}
	}
}

func alokasiMap(h HasilOptimasi) map[[2]string]float64 {
	out := map[[2]string]float64{}
	for _, a := range h.Alokasi {
		out[[2]string{a.AsalKode, a.TujuanKode}] = a.VolumeTon
	}
	return out
}

// Contoh klasik: serakah (pasangan terdekat dulu) memilih S1-D1 (1 km) lalu terpaksa S2-D2
// (100 km) dengan 10*1 + 10*100 = 1010 ton-km. Optimum silang: S1-D2 + S2-D1 = 10*2 + 10*2 = 40 ton-km.
func TestOptimalkan_BeatsGreedyNearestFirst(t *testing.T) {
	jarak := tabel(map[[2]string]float64{
		{"S1", "D1"}: 1, {"S1", "D2"}: 2, {"S2", "D1"}: 2, {"S2", "D2"}: 100,
	})

	h, err := Optimalkan(
		[]SisaWilayah{{"S1", 10}, {"S2", 10}},
		[]SisaWilayah{{"D1", 10}, {"D2", 10}},
		jarak, ParamOptimasi{TarifTonKm: 1000})

	require.NoError(t, err)
	assert.Equal(t, map[[2]string]float64{{"S1", "D2"}: 10, {"S2", "D1"}: 10}, alokasiMap(h))
	assert.Equal(t, 20.0, h.TotalVolumeTon)
	assert.EqualValues(t, 40*1000, h.TotalBiaya, "40 ton-km x Rp1.000")
	assert.Empty(t, h.BelumTerpenuhi)
	assert.Empty(t, h.SurplusSisa)
}

// Pasokan 30+20, kebutuhan 25+25, hitungan tangan di dokumentasi tugas: optimum = 450 ton-km
// dengan A->X 25, A->Y 5, B->Y 20.
func TestOptimalkan_BalancedHandComputed(t *testing.T) {
	jarak := tabel(map[[2]string]float64{
		{"A", "X"}: 10, {"A", "Y"}: 20, {"B", "X"}: 30, {"B", "Y"}: 5,
	})

	h, err := Optimalkan(
		[]SisaWilayah{{"A", 30}, {"B", 20}},
		[]SisaWilayah{{"X", 25}, {"Y", 25}},
		jarak, ParamOptimasi{TarifTonKm: 100})

	require.NoError(t, err)
	assert.Equal(t, map[[2]string]float64{{"A", "X"}: 25, {"A", "Y"}: 5, {"B", "Y"}: 20}, alokasiMap(h))
	assert.EqualValues(t, 450*100, h.TotalBiaya)
	assert.Equal(t, 50.0, h.TotalVolumeTon)
}

func TestOptimalkan_ShortSupplyShipsEverythingAndReportsUnmetDemand(t *testing.T) {
	// Pasokan 10 ton, kebutuhan 8 + 6 = 14. Jarak ke D1 lebih dekat, tetapi volume tersalur
	// dimaksimalkan dulu (10 ton), lalu ton-km diminimalkan: D1 penuh 8, sisa 2 ke D2.
	jarak := tabel(map[[2]string]float64{{"S", "D1"}: 10, {"S", "D2"}: 40})

	h, err := Optimalkan([]SisaWilayah{{"S", 10}}, []SisaWilayah{{"D1", 8}, {"D2", 6}}, jarak, ParamOptimasi{TarifTonKm: 1})

	require.NoError(t, err)
	assert.Equal(t, map[[2]string]float64{{"S", "D1"}: 8, {"S", "D2"}: 2}, alokasiMap(h))
	assert.Equal(t, []SisaWilayah{{"D2", 4}}, h.BelumTerpenuhi)
	assert.Empty(t, h.SurplusSisa)
	assert.EqualValues(t, 8*10+2*40, h.TotalBiaya)
}

func TestOptimalkan_SurplusLeftoverStaysAtFarthestSupplier(t *testing.T) {
	// Kebutuhan 5 ton hanya cukup dari pemasok terdekat; pemasok jauh tidak dipakai.
	jarak := tabel(map[[2]string]float64{{"DEKAT", "D"}: 3, {"JAUH", "D"}: 90})

	h, err := Optimalkan([]SisaWilayah{{"DEKAT", 8}, {"JAUH", 8}}, []SisaWilayah{{"D", 5}}, jarak, ParamOptimasi{TarifTonKm: 1})

	require.NoError(t, err)
	assert.Equal(t, map[[2]string]float64{{"DEKAT", "D"}: 5}, alokasiMap(h))
	assert.Equal(t, []SisaWilayah{{"DEKAT", 3}, {"JAUH", 8}}, h.SurplusSisa)
	assert.Empty(t, h.BelumTerpenuhi)
}

func TestOptimalkan_TravelTimeLimitForbidsSlowRoutes(t *testing.T) {
	// 100 km = 2 jam, 400 km = 8 jam. Batas 4 jam melarang rute S->D2.
	jarak := tabel(map[[2]string]float64{{"S", "D1"}: 100, {"S", "D2"}: 400})

	h, err := Optimalkan([]SisaWilayah{{"S", 20}}, []SisaWilayah{{"D1", 5}, {"D2", 10}}, jarak, ParamOptimasi{TarifTonKm: 1, MaksJamTempuh: 4})

	require.NoError(t, err)
	assert.Equal(t, map[[2]string]float64{{"S", "D1"}: 5}, alokasiMap(h))
	assert.Equal(t, []SisaWilayah{{"D2", 10}}, h.BelumTerpenuhi, "D2 hanya terjangkau lewat rute terlarang")
	assert.Equal(t, []SisaWilayah{{"S", 15}}, h.SurplusSisa)
	assert.Equal(t, 2.0, h.Alokasi[0].JamTempuh)
}

func TestOptimalkan_TenthOfATonResolutionAndZeroVolumes(t *testing.T) {
	jarak := tabel(map[[2]string]float64{{"S", "D"}: 10})

	h, err := Optimalkan([]SisaWilayah{{"S", 0.3}, {"Z", 0}}, []SisaWilayah{{"D", 0.2}, {"N", 0.04}}, jarak, ParamOptimasi{TarifTonKm: 10})

	require.NoError(t, err)
	assert.Equal(t, map[[2]string]float64{{"S", "D"}: 0.2}, alokasiMap(h))
	assert.Equal(t, []SisaWilayah{{"S", 0.1}}, h.SurplusSisa)
	assert.EqualValues(t, 20, h.TotalBiaya, "0,2 ton x 10 km x Rp10")
	assert.Empty(t, h.BelumTerpenuhi, "kebutuhan di bawah 0,05 ton dibulatkan menjadi nol")
}

func TestOptimalkan_EmptySidesGiveEmptyPlan(t *testing.T) {
	h, err := Optimalkan(nil, []SisaWilayah{{"D", 5}}, tabel(nil), ParamOptimasi{})

	require.NoError(t, err)
	assert.Empty(t, h.Alokasi)
	assert.Equal(t, []SisaWilayah{{"D", 5}}, h.BelumTerpenuhi)

	h, err = Optimalkan([]SisaWilayah{{"S", 5}}, nil, tabel(nil), ParamOptimasi{})
	require.NoError(t, err)
	assert.Equal(t, []SisaWilayah{{"S", 5}}, h.SurplusSisa)
}

func TestOptimalkan_RejectsInvalidInput(t *testing.T) {
	jarak := tabel(map[[2]string]float64{{"A", "B"}: 1})
	cases := map[string]struct {
		sup, dem []SisaWilayah
		p        ParamOptimasi
	}{
		"wilayah di kedua sisi": {[]SisaWilayah{{"A", 1}}, []SisaWilayah{{"A", 1}}, ParamOptimasi{}},
		"kode ganda":            {[]SisaWilayah{{"A", 1}, {"A", 2}}, []SisaWilayah{{"B", 1}}, ParamOptimasi{}},
		"kode kosong":           {[]SisaWilayah{{"", 1}}, []SisaWilayah{{"B", 1}}, ParamOptimasi{}},
		"volume negatif":        {[]SisaWilayah{{"A", -1}}, []SisaWilayah{{"B", 1}}, ParamOptimasi{}},
		"volume tak hingga":     {[]SisaWilayah{{"A", math.Inf(1)}}, []SisaWilayah{{"B", 1}}, ParamOptimasi{}},
		"tarif negatif":         {[]SisaWilayah{{"A", 1}}, []SisaWilayah{{"B", 1}}, ParamOptimasi{TarifTonKm: -1}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Optimalkan(tc.sup, tc.dem, jarak, tc.p)

			assert.ErrorIs(t, err, ErrOptimasiTidakValid)
		})
	}

	_, err := Optimalkan([]SisaWilayah{{"A", 1}}, []SisaWilayah{{"B", 1}}, func(string, string) Jalur { return Jalur{JarakKm: -1} }, ParamOptimasi{})
	assert.ErrorIs(t, err, ErrOptimasiTidakValid, "jarak negatif")
}

// bruteForce mencari ongkos ton-km minimum (di antara rencana dengan volume maksimum)
// dengan menelusuri semua alokasi bulat 2 pemasok x 2 peminta.
func bruteForce(sup, dem [2]int, km [2][2]float64) (volume int, biaya float64) {
	biaya = math.Inf(1)
	for a := 0; a <= min(sup[0], dem[0]); a++ {
		for b := 0; b <= min(sup[0]-a, dem[1]); b++ {
			for c := 0; c <= min(sup[1], dem[0]-a); c++ {
				for d := 0; d <= min(sup[1]-c, dem[1]-b); d++ {
					v := a + b + c + d
					cost := float64(a)*km[0][0] + float64(b)*km[0][1] + float64(c)*km[1][0] + float64(d)*km[1][1]
					if v > volume || (v == volume && cost < biaya) {
						volume, biaya = v, cost
					}
				}
			}
		}
	}
	return volume, biaya
}

func TestOptimalkan_MatchesBruteForceOnRandomSmallInstances(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	for range 300 {
		sup := [2]int{rng.IntN(9), rng.IntN(9)}
		dem := [2]int{rng.IntN(9), rng.IntN(9)}
		var km [2][2]float64
		for i := range km {
			for j := range km[i] {
				km[i][j] = float64(1 + rng.IntN(60))
			}
		}
		jarak := tabel(map[[2]string]float64{{"S0", "D0"}: km[0][0], {"S0", "D1"}: km[0][1], {"S1", "D0"}: km[1][0], {"S1", "D1"}: km[1][1]})

		h, err := Optimalkan(
			[]SisaWilayah{{"S0", float64(sup[0])}, {"S1", float64(sup[1])}},
			[]SisaWilayah{{"D0", float64(dem[0])}, {"D1", float64(dem[1])}},
			jarak, ParamOptimasi{TarifTonKm: 1})
		require.NoError(t, err)

		wantVol, wantCost := bruteForce(sup, dem, km)
		var gotCost float64
		for _, a := range h.Alokasi {
			gotCost += a.VolumeTon * a.JarakKm
		}
		require.Equal(t, float64(wantVol), h.TotalVolumeTon, "sup=%v dem=%v km=%v", sup, dem, km)
		require.InDelta(t, wantCost, gotCost, 1e-9, "sup=%v dem=%v km=%v: %v", sup, dem, km, h.Alokasi)
	}
}

func TestOptimalkan_PlanRespectsCapacities(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	for range 100 {
		var sup, dem []SisaWilayah
		for i := range 1 + rng.IntN(5) {
			sup = append(sup, SisaWilayah{Kode: string(rune('A' + i)), Ton: float64(rng.IntN(400)) / 10})
		}
		for j := range 1 + rng.IntN(5) {
			dem = append(dem, SisaWilayah{Kode: string(rune('a' + j)), Ton: float64(rng.IntN(400)) / 10})
		}
		jarak := func(a, b string) Jalur { return Jalur{JarakKm: float64(len(a)+int(a[0])+int(b[0])) / 7, JamTempuh: 1} }

		h, err := Optimalkan(sup, dem, jarak, ParamOptimasi{TarifTonKm: 1})

		require.NoError(t, err)
		keluar, masuk := map[string]float64{}, map[string]float64{}
		for _, a := range h.Alokasi {
			assert.Positive(t, a.VolumeTon)
			keluar[a.AsalKode] += a.VolumeTon
			masuk[a.TujuanKode] += a.VolumeTon
		}
		for _, s := range sup {
			assert.LessOrEqual(t, keluar[s.Kode], s.Ton+1e-9, "pemasok %s tidak boleh melampaui surplus", s.Kode)
		}
		for _, d := range dem {
			assert.LessOrEqual(t, masuk[d.Kode], d.Ton+1e-9, "peminta %s tidak boleh melampaui defisit", d.Kode)
		}
		var sisa float64
		for _, b := range h.BelumTerpenuhi {
			sisa += b.Ton
		}
		var totalDem float64
		for _, d := range dem {
			totalDem += d.Ton
		}
		assert.InDelta(t, totalDem, h.TotalVolumeTon+sisa, 1e-6, "volume tersalur + belum terpenuhi = kebutuhan")
	}
}
