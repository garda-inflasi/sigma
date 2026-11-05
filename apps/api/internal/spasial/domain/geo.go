package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
)

// Geometri wilayah disimpan sebagai GeoJSON (jsonb) tanpa ekstensi PostGIS;
// perhitungan jarak dan titik-di-dalam-poligon dilakukan di sini (lihat
// docs/todos/12-peta-spasial.md, 12-B).

// Tipe geometri GeoJSON yang diterima.
const (
	TipePolygon      = "Polygon"
	TipeMultiPolygon = "MultiPolygon"
)

// radiusBumiKm adalah jari-jari bumi rata-rata untuk rumus haversine.
const radiusBumiKm = 6371.0088

// Titik adalah posisi GeoJSON: {bujur, lintang}.
type Titik [2]float64

// Cincin adalah barisan titik tertutup (titik pertama = terakhir).
type Cincin []Titik

// Poligon adalah cincin luar diikuti nol atau lebih lubang.
type Poligon []Cincin

// Geometri adalah satu atau beberapa poligon.
type Geometri []Poligon

// ErrGeometriTidakValid dibungkus oleh galat validasi GeoJSON.
var ErrGeometriTidakValid = errors.New("geometri tidak valid")

func tidakValid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrGeometriTidakValid, fmt.Sprintf(format, args...))
}

type geojsonRaw struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// ParseGeoJSON membaca dan memvalidasi geometri Polygon/MultiPolygon: tiap cincin
// tertutup dengan minimal empat titik, koordinat berhingga dalam rentang bumi, dan
// cincin luar memiliki luas. Hasilnya selalu berupa daftar poligon.
func ParseGeoJSON(raw []byte) (Geometri, error) {
	var g geojsonRaw
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, tidakValid("bukan JSON: %v", err)
	}
	var polys []Poligon
	switch g.Type {
	case TipePolygon:
		var p Poligon
		if err := json.Unmarshal(g.Coordinates, &p); err != nil {
			return nil, tidakValid("koordinat Polygon: %v", err)
		}
		polys = []Poligon{p}
	case TipeMultiPolygon:
		if err := json.Unmarshal(g.Coordinates, &polys); err != nil {
			return nil, tidakValid("koordinat MultiPolygon: %v", err)
		}
	default:
		return nil, tidakValid("tipe %q (harus Polygon atau MultiPolygon)", g.Type)
	}
	if len(polys) == 0 {
		return nil, tidakValid("tanpa poligon")
	}
	for i, p := range polys {
		if err := p.validasi(); err != nil {
			return nil, tidakValid("poligon %d: %v", i, err)
		}
	}
	return Geometri(polys), nil
}

func (p Poligon) validasi() error {
	if len(p) == 0 {
		return errors.New("tanpa cincin luar")
	}
	for j, c := range p {
		if len(c) < 4 {
			return fmt.Errorf("cincin %d kurang dari 4 titik", j)
		}
		if c[0] != c[len(c)-1] {
			return fmt.Errorf("cincin %d tidak tertutup", j)
		}
		for _, t := range c {
			if math.IsNaN(t[0]) || math.IsNaN(t[1]) || t[0] < -180 || t[0] > 180 || t[1] < -90 || t[1] > 90 {
				return fmt.Errorf("cincin %d memuat koordinat di luar rentang", j)
			}
		}
	}
	if luasCincin(p[0]) == 0 {
		return errors.New("cincin luar tanpa luas")
	}
	return nil
}

// luasCincin menghitung luas bertanda (rumus tali sepatu) dalam derajat persegi.
func luasCincin(c Cincin) float64 {
	var s float64
	for i := 0; i < len(c)-1; i++ {
		s += c[i][0]*c[i+1][1] - c[i+1][0]*c[i][1]
	}
	return s / 2
}

// GeoJSON menyusun kembali dokumen GeoJSON; satu poligon menjadi Polygon.
func (g Geometri) GeoJSON() ([]byte, error) {
	if len(g) == 1 {
		return json.Marshal(geojsonOut{Type: TipePolygon, Coordinates: g[0]})
	}
	return json.Marshal(geojsonOut{Type: TipeMultiPolygon, Coordinates: []Poligon(g)})
}

type geojsonOut struct {
	Type        string `json:"type"`
	Coordinates any    `json:"coordinates"`
}

// BBox adalah kotak pembatas dalam derajat.
type BBox struct{ MinLat, MinLng, MaxLat, MaxLng float64 }

// Beririsan melaporkan apakah dua kotak berbagi area (batas bersentuhan dihitung).
func (b BBox) Beririsan(o BBox) bool {
	return b.MinLng <= o.MaxLng && o.MinLng <= b.MaxLng && b.MinLat <= o.MaxLat && o.MinLat <= b.MaxLat
}

// BBox menghitung kotak pembatas seluruh poligon.
func (g Geometri) BBox() BBox {
	b := BBox{MinLat: math.Inf(1), MinLng: math.Inf(1), MaxLat: math.Inf(-1), MaxLng: math.Inf(-1)}
	for _, p := range g {
		for _, t := range p[0] {
			b.MinLng, b.MaxLng = math.Min(b.MinLng, t[0]), math.Max(b.MaxLng, t[0])
			b.MinLat, b.MaxLat = math.Min(b.MinLat, t[1]), math.Max(b.MaxLat, t[1])
		}
	}
	return b
}

// Mengandung melaporkan apakah titik berada di dalam salah satu poligon dan
// tidak di dalam lubangnya (algoritma ray casting; titik tepat di sisi dapat
// jatuh ke salah satu sisi).
func (g Geometri) Mengandung(lat, lng float64) bool {
	for _, p := range g {
		if !dalamCincin(p[0], lat, lng) {
			continue
		}
		lubang := false
		for _, h := range p[1:] {
			if dalamCincin(h, lat, lng) {
				lubang = true
				break
			}
		}
		if !lubang {
			return true
		}
	}
	return false
}

func dalamCincin(c Cincin, lat, lng float64) bool {
	dalam := false
	for i, j := 0, len(c)-1; i < len(c); j, i = i, i+1 {
		xi, yi, xj, yj := c[i][0], c[i][1], c[j][0], c[j][1]
		if (yi > lat) != (yj > lat) && lng < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			dalam = !dalam
		}
	}
	return dalam
}

// JarakKm adalah jarak garis lurus (haversine) antara dua titik lintang/bujur, dalam km.
func JarakKm(lat1, lng1, lat2, lng2 float64) float64 {
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLng := rad(lat2-lat1), rad(lng2-lng1)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * radiusBumiKm * math.Asin(math.Min(1, math.Sqrt(h)))
}

// ToleransiZoom memetakan tingkat zoom peta ke toleransi penyederhanaan (derajat).
// Zoom tinggi (>= 11) tidak disederhanakan.
func ToleransiZoom(zoom int) float64 {
	switch {
	case zoom <= 6:
		return 0.05
	case zoom <= 8:
		return 0.02
	case zoom <= 10:
		return 0.008
	default:
		return 0
	}
}

// Sederhanakan menyederhanakan semua cincin dengan Douglas-Peucker. Cincin yang
// akan runtuh (kurang dari 3 titik berbeda) dipertahankan apa adanya; lubang yang
// runtuh dibuang. Toleransi <= 0 mengembalikan geometri tanpa perubahan.
func (g Geometri) Sederhanakan(toleransi float64) Geometri {
	if toleransi <= 0 {
		return g
	}
	out := make(Geometri, len(g))
	for i, p := range g {
		np := Poligon{sederhanakanCincin(p[0], toleransi, true)}
		for _, h := range p[1:] {
			if s := sederhanakanCincin(h, toleransi, false); len(s) >= 4 {
				np = append(np, s)
			}
		}
		out[i] = np
	}
	return out
}

func sederhanakanCincin(c Cincin, tol float64, pertahankan bool) Cincin {
	terbuka := c[:len(c)-1]
	keep := make([]bool, len(terbuka))
	keep[0], keep[len(terbuka)-1] = true, true
	douglasPeucker(terbuka, 0, len(terbuka)-1, tol, keep)

	hasil := make(Cincin, 0, len(terbuka)+1)
	for i, t := range terbuka {
		if keep[i] {
			hasil = append(hasil, t)
		}
	}
	if len(hasil) < 3 {
		if pertahankan {
			return c
		}
		return nil
	}
	return append(hasil, hasil[0])
}

func douglasPeucker(pts Cincin, awal, akhir int, tol float64, keep []bool) {
	if akhir <= awal+1 {
		return
	}
	maks, idx := 0.0, -1
	for i := awal + 1; i < akhir; i++ {
		if d := jarakKeRuas(pts[i], pts[awal], pts[akhir]); d > maks {
			maks, idx = d, i
		}
	}
	if idx >= 0 && maks > tol {
		keep[idx] = true
		douglasPeucker(pts, awal, idx, tol, keep)
		douglasPeucker(pts, idx, akhir, tol, keep)
	}
}

// jarakKeRuas adalah jarak titik p ke ruas a-b dalam derajat (bidang datar).
func jarakKeRuas(p, a, b Titik) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	if dx == 0 && dy == 0 {
		return math.Hypot(p[0]-a[0], p[1]-a[1])
	}
	t := math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dy)/(dx*dx+dy*dy)))
	return math.Hypot(p[0]-(a[0]+t*dx), p[1]-(a[1]+t*dy))
}

// CembungHull mengembalikan cincin tertutup selimut cembung (Andrew monotone chain)
// dari kumpulan titik; berlawanan arah jarum jam. Butuh minimal tiga titik tak segaris.
func CembungHull(pts []Titik) (Cincin, error) {
	p := append([]Titik(nil), pts...)
	sort.Slice(p, func(i, j int) bool {
		if p[i][0] != p[j][0] {
			return p[i][0] < p[j][0]
		}
		return p[i][1] < p[j][1]
	})
	if len(p) < 3 {
		return nil, tidakValid("selimut cembung butuh minimal 3 titik")
	}
	silang := func(o, a, b Titik) float64 { return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0]) }
	var hull []Titik
	for _, t := range p {
		for len(hull) >= 2 && silang(hull[len(hull)-2], hull[len(hull)-1], t) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, t)
	}
	bawah := len(hull) + 1
	for i := len(p) - 2; i >= 0; i-- {
		for len(hull) >= bawah && silang(hull[len(hull)-2], hull[len(hull)-1], p[i]) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p[i])
	}
	if len(hull) < 4 {
		return nil, tidakValid("titik segaris")
	}
	return Cincin(hull), nil
}
