package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encodePolyline6 membuat encoded polyline presisi 1e-6 untuk koordinat [lat, lng] di test.
func encodePolyline6(points [][2]float64) string {
	var b strings.Builder
	var prevLat, prevLng int64
	encodeVal := func(v int64) {
		sv := uint64(v << 1)
		if v < 0 {
			sv = ^sv
		}
		for sv >= 0x20 {
			b.WriteByte(byte((0x20 | (sv & 0x1f)) + 63))
			sv >>= 5
		}
		b.WriteByte(byte(sv + 63))
	}
	for _, p := range points {
		lat := int64(p[0]*1e6 + 0.5)
		lng := int64(p[1]*1e6 + 0.5)
		encodeVal(lat - prevLat)
		encodeVal(lng - prevLng)
		prevLat, prevLng = lat, lng
	}
	return b.String()
}

func TestDecodePolyline6AndDistance(t *testing.T) {
	shape := encodePolyline6([][2]float64{
		{4.6270, 96.8460},
		{5.0000, 96.1000},
		{5.5577, 95.3222},
	})
	coords := DecodePolyline6(shape)
	require.Len(t, coords, 3)
	assert.InDelta(t, 96.8460, coords[0][0], 1e-4)
	assert.InDelta(t, 4.6270, coords[0][1], 1e-4)
	assert.InDelta(t, 95.3222, coords[2][0], 1e-4)
	assert.InDelta(t, 5.5577, coords[2][1], 1e-4)

	// Titik tepat pada titik tengah koridor -> jarak hampir 0 meter.
	dOnLine := PointToPolylineMeters(5.0000, 96.1000, coords)
	assert.Less(t, dOnLine, 20.0)

	// Titik jauh dari koridor (Sabang) -> puluhan km.
	dFar := PointToPolylineMeters(5.8940, 95.3190, coords)
	assert.Greater(t, dFar, 30000.0)
}

func TestFetchRoadRoutesAndMatrix(t *testing.T) {
	mainShape := encodePolyline6([][2]float64{{4.627, 96.846}, {5.1, 96.2}, {5.557, 95.322}})
	altShape := encodePolyline6([][2]float64{{4.627, 96.846}, {4.9, 95.7}, {5.557, 95.322}})

	var seenExclude int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/route":
			var req valhallaRouteReq
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			seenExclude = len(req.ExcludeLocations)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"trip": map[string]any{
					"status":  0,
					"summary": map[string]any{"length": 248.4, "time": 16200.0},
					"legs": []any{map[string]any{
						"shape":     mainShape,
						"maneuvers": []any{map[string]any{"street_names": []string{"Jl. Nasional Bireuen-Takengon", "Jl. Banda Aceh-Medan"}}},
					}},
				},
				"alternates": []any{map[string]any{
					"trip": map[string]any{
						"status":  0,
						"summary": map[string]any{"length": 274.2, "time": 18000.0},
						"legs": []any{map[string]any{
							"shape":     altShape,
							"maneuvers": []any{map[string]any{"street_names": []string{"Lintas Barat Aceh"}}},
						}},
					},
				}},
			})
		case "/sources_to_targets":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sources_to_targets": [][]map[string]any{{
					{"distance": 12.4, "time": 1320.0},
					{"distance": 6.2, "time": 660.0},
				}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "truck", srv.Client())
	routes, err := client.FetchRoadRoutes(context.Background(), 4.627, 96.846, 5.557, 95.322, []GangguanRef{
		{Kode: "G1", Lat: 5.1, Lng: 96.2},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, seenExclude)
	require.Len(t, routes, 2)
	assert.Equal(t, "Jalur Utama", routes[0].Tipe)
	assert.Equal(t, "Jl. Nasional Bireuen-Takengon → Jl. Banda Aceh-Medan", routes[0].RingkasanJalan)
	assert.InDelta(t, 248.4, routes[0].JarakKm, 0.01)
	assert.Equal(t, 270, routes[0].DurasiMenit)
	assert.Equal(t, "TINGGI", routes[0].RisikoFoodLoss)
	assert.Equal(t, "Jalur Alternatif", routes[1].Tipe)

	cells, err := client.FetchMatrix(context.Background(), 4.627, 96.846, []MatrixTarget{
		{Lat: 5.55, Lng: 95.32},
		{Lat: 4.70, Lng: 96.80},
	})
	require.NoError(t, err)
	require.Len(t, cells, 2)
	assert.True(t, cells[1].Valid)
	assert.InDelta(t, 6.2, cells[1].JarakKm, 0.01)
	assert.Equal(t, 11, cells[1].DurasiMenit)
}

func TestCacheDisruptionInvalidation(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache := NewCacheWithClient(rdb)
	ctx := context.Background()

	v0 := cache.Version(ctx)
	key0 := RouteCacheKey(4.627, 96.846, 5.557, 95.322, "truck", v0)
	cache.SetRoute(ctx, key0, HasilRute{SumberRute: "VALHALLA_OSM"})

	var got HasilRute
	require.True(t, cache.GetRoute(ctx, key0, &got))
	assert.Equal(t, "VALHALLA_OSM", got.SumberRute)

	cache.InvalidateDisruptions(ctx)
	v1 := cache.Version(ctx)
	assert.NotEqual(t, v0, v1)
	key1 := RouteCacheKey(4.627, 96.846, 5.557, 95.322, "truck", v1)
	assert.False(t, cache.GetRoute(ctx, key1, &got))
}

func TestOSRMFallbackAndMatrixAndDetour(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/route/v1/driving/"):
			if strings.Contains(r.URL.Path, ";") && strings.Count(r.URL.Path, ";") == 2 {
				// Detour via waypoint tegak lurus
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code": "Ok",
					"routes": []any{map[string]any{
						"distance": 13400.0,
						"duration": 1080.0,
						"geometry": map[string]any{
							"type":        "LineString",
							"coordinates": [][]float64{{95.335, 5.568}, {95.310, 5.540}, {95.365, 5.518}},
						},
						"legs": []any{map[string]any{
							"steps": []any{map[string]any{"name": "Jalan Soekarno-Hatta"}},
						}},
					}},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": "Ok",
				"routes": []any{
					map[string]any{
						"distance": 10800.0,
						"duration": 840.0,
						"geometry": map[string]any{
							"type":        "LineString",
							"coordinates": [][]float64{{95.335, 5.568}, {95.350, 5.543}, {95.365, 5.518}},
						},
						"legs": []any{map[string]any{
							"steps": []any{map[string]any{"name": "Jalan T. Nyak Arief"}, map[string]any{"name": "Jalan Banda Aceh-Medan"}},
						}},
					},
					map[string]any{
						"distance": 12200.0,
						"duration": 960.0,
						"geometry": map[string]any{
							"type":        "LineString",
							"coordinates": [][]float64{{95.335, 5.568}, {95.382, 5.548}, {95.365, 5.518}},
						},
						"legs": []any{map[string]any{
							"steps": []any{map[string]any{"name": "Jalan Tengku Chik Ditiro"}},
						}},
					},
				},
			})
		case strings.HasPrefix(r.URL.Path, "/table/v1/driving/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":      "Ok",
				"distances": [][]float64{{10800.0, 5200.0}},
				"durations": [][]float64{{840.0, 420.0}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient("", "truck", srv.Client()).WithOSRM(srv.URL)
	routes, sumber, err := client.FetchRoadRoutesWithSource(context.Background(), 5.568, 95.335, 5.518, 95.365, nil)
	require.NoError(t, err)
	assert.Equal(t, "OSRM_OSM", sumber)
	require.Len(t, routes, 2)
	assert.Equal(t, "Jalan T. Nyak Arief → Jalan Banda Aceh-Medan", routes[0].RingkasanJalan)
	assert.InDelta(t, 10.8, routes[0].JarakKm, 0.01)
	assert.Equal(t, 16, routes[0].DurasiMenit) // 840 * 1.15 / 60 = 16 mnt untuk profil truck

	detour, err := client.FetchOSRMDetour(context.Background(), 5.568, 95.335, 5.518, 95.365, GangguanRef{
		Lat:     5.543,
		Lng:     95.350,
		RadiusM: 400,
	})
	require.NoError(t, err)
	require.Len(t, detour, 1)
	assert.Equal(t, "Jalan Soekarno-Hatta", detour[0].RingkasanJalan)

	cells, err := client.FetchMatrix(context.Background(), 5.568, 95.335, []MatrixTarget{
		{Lat: 5.518, Lng: 95.365},
		{Lat: 5.550, Lng: 95.340},
	})
	require.NoError(t, err)
	require.Len(t, cells, 2)
	assert.InDelta(t, 5.2, cells[1].JarakKm, 0.01)
	assert.Equal(t, 8, cells[1].DurasiMenit)
}
