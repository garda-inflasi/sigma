package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garda-inflasi/api/internal/spasial/routing"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeTestPoly6(pts [][2]float64) string {
	var b strings.Builder
	var pLat, pLng int64
	enc := func(v int64) {
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
	for _, p := range pts {
		lat := int64(p[0]*1e6 + 0.5)
		lng := int64(p[1]*1e6 + 0.5)
		enc(lat - pLat)
		enc(lng - pLng)
		pLat, pLng = lat, lng
	}
	return b.String()
}

func TestHitungJarakDuaTitik(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewTitik(nil)
	r := gin.New()
	r.GET("/jarak", h.HitungJarak)

	// Takengon (4.627, 96.846) ke Banda Aceh (5.5577, 95.3222) ~ 197 km garis lurus
	w := httptest.NewRecorder()
	req := httptest.NewRequest(stdhttp.MethodGet, "/jarak?asal_lat=4.627&asal_lng=96.846&tujuan_lat=5.5577&tujuan_lng=95.3222&faktor_jalan=1.3", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, stdhttp.StatusOK, w.Code)

	var body struct {
		Data JarakResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.Data.JarakLurusKm)
	require.NotNil(t, body.Data.JarakJalanKm)
	assert.Greater(t, *body.Data.JarakLurusKm, 180.0)
	assert.Less(t, *body.Data.JarakLurusKm, 215.0)
	assert.Greater(t, *body.Data.JarakJalanKm, *body.Data.JarakLurusKm)
	assert.Equal(t, "TINGGI", body.Data.RisikoFoodLoss)
	assert.Equal(t, "ESTIMASI_HAVERSINE", body.Data.SumberRute)
	require.Len(t, body.Data.Rute, 1)

	// Koordinat invalid -> 400
	wErr := httptest.NewRecorder()
	r.ServeHTTP(wErr, httptest.NewRequest(stdhttp.MethodGet, "/jarak?asal_lat=abc&asal_lng=95", nil))
	assert.Equal(t, stdhttp.StatusBadRequest, wErr.Code)
}

func TestEvaluateDisruptionsAndAutoReroute(t *testing.T) {
	normalMain := encodeTestPoly6([][2]float64{{4.627, 96.846}, {5.020, 96.150}, {5.557, 95.322}})
	normalAlt := encodeTestPoly6([][2]float64{{4.627, 96.846}, {4.950, 95.600}, {5.557, 95.322}})
	detourMain := encodeTestPoly6([][2]float64{{4.627, 96.846}, {4.750, 95.650}, {5.557, 95.322}})

	srv := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		var req struct {
			ExcludeLocations []struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"exclude_locations"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if len(req.ExcludeLocations) > 0 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"trip": map[string]any{
					"status":  0,
					"summary": map[string]any{"length": 279.0, "time": 19500.0},
					"legs": []any{map[string]any{
						"shape":     detourMain,
						"maneuvers": []any{map[string]any{"street_names": []string{"Lintas Tengah-Barat Aceh"}}},
					}},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"trip": map[string]any{
				"status":  0,
				"summary": map[string]any{"length": 245.0, "time": 16200.0},
				"legs": []any{map[string]any{
					"shape":     normalMain,
					"maneuvers": []any{map[string]any{"street_names": []string{"Jl. Takengon-Bireuen"}}},
				}},
			},
			"alternates": []any{map[string]any{
				"trip": map[string]any{
					"status":  0,
					"summary": map[string]any{"length": 288.0, "time": 20400.0},
					"legs": []any{map[string]any{
						"shape":     normalAlt,
						"maneuvers": []any{map[string]any{"street_names": []string{"Jl. Geumpang"}}},
					}},
				},
			}},
		})
	}))
	defer srv.Close()

	client := routing.NewClient(srv.URL, "truck", srv.Client())
	rawRoutes, err := client.FetchRoadRoutes(context.Background(), 4.627, 96.846, 5.557, 95.322, nil)
	require.NoError(t, err)

	// Gangguan TUTUP_TOTAL tepat pada koridor utama (5.020, 96.150) harus memicu pengalihan.
	disruptions := []routing.GangguanRef{{
		ID:      "g1",
		Kode:    "LONGSOR-01",
		Judul:   "Longsor Ruas Takengon-Bireuen",
		Jenis:   "LONGSOR",
		Tingkat: "TUTUP_TOTAL",
		Lat:     5.020,
		Lng:     96.150,
		RadiusM: 400,
	}}
	res := evaluateDisruptionsAndReroute(context.Background(), client, "VALHALLA_OSM", 4.627, 96.846, 5.557, 95.322, rawRoutes, disruptions)
	assert.Equal(t, "VALHALLA_OSM", res.SumberRute)
	assert.True(t, res.JalurUtamaDiblokir)
	require.NotNil(t, res.RuteDiblokir)
	assert.Equal(t, "Jalur Terdampak", res.RuteDiblokir.Tipe)
	require.Len(t, res.Rute, 1)
	assert.Equal(t, "Jalur Pengalihan", res.Rute[0].Tipe)
	assert.InDelta(t, 34.0, res.Rute[0].SelisihKm, 0.1)
	assert.Equal(t, 55, res.Rute[0].SelisihMenit)
	require.Len(t, res.GangguanTerkait, 1)
	assert.Equal(t, "LONGSOR-01", res.GangguanTerkait[0].Kode)

	// Uji juga integrasi HandlerTitik dengan Valhalla mock server aktif.
	gin.SetMode(gin.TestMode)
	h := NewTitikWithRouting(nil, client, nil)
	r := gin.New()
	r.GET("/jarak", h.HitungJarak)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(stdhttp.MethodGet, "/jarak?asal_lat=4.627&asal_lng=96.846&tujuan_lat=5.557&tujuan_lng=95.322", nil))
	require.Equal(t, stdhttp.StatusOK, w.Code)
	var out struct {
		Data JarakResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "VALHALLA_OSM", out.Data.SumberRute)
	assert.Equal(t, "truck", out.Data.ProfilKendaraan)
	assert.Len(t, out.Data.Rute, 2)
}
