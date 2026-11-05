package seeder

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/garda-inflasi/api/internal/spasial/domain"
)

// KeteranganGeometri dicatat pada tiap baris wilayah_geometri demo.
const KeteranganGeometri = "Poligon perkiraan (selimut cembung di sekitar titik pusat, pasar, dan cold storage). Bukan batas administratif resmi; ganti dengan impor batas BPS/Geoportal bila tersedia."

const (
	// jariRatio menentukan jari-jari poligon dasar sebagai pecahan jarak ke tetangga terdekat,
	// dibatasi [jariMin, jariMaks] derajat agar wilayah tidak terlalu kecil atau besar.
	jariRatio = 0.45
	jariMin   = 0.08
	jariMaks  = 0.3
	// sisiJangkar adalah setengah sisi kotak kecil di sekitar titik jangkar (pasar, fasilitas),
	// yang menjamin titik itu berada di dalam poligon kabupatennya.
	sisiJangkar = 0.02
	// segiDasar adalah jumlah titik poligon dasar di sekeliling pusat.
	segiDasar = 10
)

// BuildGeometri menyusun poligon perkiraan tiap wilayah: selimut cembung dari
// segi-10 tak beraturan (deterministik menurut kode wilayah) di sekitar titik pusat
// dan kotak kecil di sekitar setiap titik jangkar wilayah itu. Hasilnya valid
// (tertutup, berluas) dan menjamin setiap jangkar berada di dalam poligonnya.
func BuildGeometri(pusat map[string]Koordinat, jangkar map[string][]Koordinat) (map[string]domain.Geometri, error) {
	kode := KodeWilayah(pusat)
	out := make(map[string]domain.Geometri, len(kode))
	for _, k := range kode {
		c := pusat[k]
		jari := math.Min(jariMaks, math.Max(jariMin, jariRatio*jarakTetanggaDerajat(k, pusat)))
		h := fnv.New64a()
		_, _ = h.Write([]byte(k))
		rng := rand.New(rand.NewPCG(h.Sum64(), 12))

		titik := make([]domain.Titik, 0, segiDasar+4*len(jangkar[k]))
		for i := range segiDasar {
			sudut := 2 * math.Pi * float64(i) / segiDasar
			r := jari * (0.85 + 0.3*rng.Float64())
			titik = append(titik, domain.Titik{round5(c.Lng + r*math.Cos(sudut)), round5(c.Lat + r*math.Sin(sudut))})
		}
		for _, j := range jangkar[k] {
			for _, d := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
				titik = append(titik, domain.Titik{round5(j.Lng + d[0]*sisiJangkar), round5(j.Lat + d[1]*sisiJangkar)})
			}
		}
		ring, err := domain.CembungHull(titik)
		if err != nil {
			return nil, fmt.Errorf("geometri %s: %w", k, err)
		}
		out[k] = domain.Geometri{domain.Poligon{ring}}
	}
	return out, nil
}

// jarakTetanggaDerajat adalah jarak (derajat, bidang datar) dari wilayah k ke pusat terdekat lainnya.
func jarakTetanggaDerajat(k string, pusat map[string]Koordinat) float64 {
	c := pusat[k]
	jarak := make([]float64, 0, len(pusat))
	for lain, p := range pusat {
		if lain != k {
			jarak = append(jarak, math.Hypot(p.Lat-c.Lat, p.Lng-c.Lng))
		}
	}
	if len(jarak) == 0 {
		return jariMaks / jariRatio
	}
	sort.Float64s(jarak)
	return jarak[0]
}

func round5(v float64) float64 { return math.Round(v*1e5) / 1e5 }
