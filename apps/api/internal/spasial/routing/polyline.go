package routing

import (
	"math"

	"github.com/garda-inflasi/api/internal/spasial/domain"
)

// DecodePolyline6 mengubah encoded polyline Valhalla (presisi 1e-6) menjadi daftar koordinat GeoJSON [lng, lat].
func DecodePolyline6(encoded string) [][]float64 {
	if encoded == "" {
		return nil
	}
	coords := make([][]float64, 0, len(encoded)/4)
	var lat, lng int64
	i := 0
	for i < len(encoded) {
		var shift, result uint64
		for {
			if i >= len(encoded) {
				return coords
			}
			b := uint64(encoded[i]) - 63
			i++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lat += ^int64(result >> 1)
		} else {
			lat += int64(result >> 1)
		}

		shift, result = 0, 0
		for {
			if i >= len(encoded) {
				return coords
			}
			b := uint64(encoded[i]) - 63
			i++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			lng += ^int64(result >> 1)
		} else {
			lng += int64(result >> 1)
		}

		coords = append(coords, []float64{float64(lng) * 1e-6, float64(lat) * 1e-6})
	}
	return coords
}

// PointToPolylineMeters menghitung jarak minimum (meter) dari titik (lat, lng) ke segmen-segmen polyline GeoJSON [lng, lat].
func PointToPolylineMeters(lat, lng float64, coords [][]float64) float64 {
	if len(coords) == 0 {
		return math.Inf(1)
	}
	if len(coords) == 1 {
		return domain.JarakKm(lat, lng, coords[0][1], coords[0][0]) * 1000.0
	}
	minDist := math.Inf(1)
	for i := 0; i < len(coords)-1; i++ {
		d := pointToSegmentMeters(lat, lng, coords[i][1], coords[i][0], coords[i+1][1], coords[i+1][0])
		if d < minDist {
			minDist = d
		}
	}
	return minDist
}

func pointToSegmentMeters(pLat, pLng, aLat, aLng, bLat, bLng float64) float64 {
	// Proyeksi ekuirektangular lokal akurat pada skala jalan Aceh (sub-km hingga puluhan km).
	cosLat := math.Cos(pLat * math.Pi / 180.0)
	toX := func(lng float64) float64 { return lng * 111320.0 * cosLat }
	toY := func(lat float64) float64 { return lat * 110540.0 }

	px, py := toX(pLng), toY(pLat)
	ax, ay := toX(aLng), toY(aLat)
	bx, by := toX(bLng), toY(bLat)

	dx, dy := bx-ax, by-ay
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	projX, projY := ax+t*dx, ay+t*dy
	return math.Hypot(px-projX, py-projY)
}
