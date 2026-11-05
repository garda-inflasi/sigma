package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kotak 2x2 derajat dengan lubang 0,5x0,5 di tengah.
const kotakBerlubang = `{"type":"Polygon","coordinates":[
	[[0,0],[2,0],[2,2],[0,2],[0,0]],
	[[0.75,0.75],[1.25,0.75],[1.25,1.25],[0.75,1.25],[0.75,0.75]]]}`

func TestParseGeoJSON_ValidShapes(t *testing.T) {
	g, err := ParseGeoJSON([]byte(kotakBerlubang))
	require.NoError(t, err)
	require.Len(t, g, 1)
	assert.Len(t, g[0], 2, "cincin luar + satu lubang")

	multi, err := ParseGeoJSON([]byte(`{"type":"MultiPolygon","coordinates":[
		[[[0,0],[1,0],[1,1],[0,0]]],
		[[[5,5],[6,5],[6,6],[5,5]]]]}`))
	require.NoError(t, err)
	assert.Len(t, multi, 2)
}

func TestParseGeoJSON_Rejects(t *testing.T) {
	cases := map[string]string{
		"bukan JSON":            `{rusak`,
		"tipe titik":            `{"type":"Point","coordinates":[1,2]}`,
		"tanpa poligon":         `{"type":"MultiPolygon","coordinates":[]}`,
		"tanpa cincin":          `{"type":"Polygon","coordinates":[]}`,
		"cincin kurang titik":   `{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]}`,
		"cincin tak tertutup":   `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`,
		"bujur di luar rentang": `{"type":"Polygon","coordinates":[[[0,0],[181,0],[1,1],[0,0]]]}`,
		"lintang di luar":       `{"type":"Polygon","coordinates":[[[0,0],[1,91],[1,1],[0,0]]]}`,
		"tanpa luas":            `{"type":"Polygon","coordinates":[[[0,0],[1,1],[2,2],[0,0]]]}`,
		"koordinat bukan angka": `{"type":"Polygon","coordinates":[[["a","b"],[1,0],[1,1],[0,0]]]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseGeoJSON([]byte(raw))

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrGeometriTidakValid)
		})
	}
}

func TestGeometri_GeoJSONRoundTrip(t *testing.T) {
	g, err := ParseGeoJSON([]byte(kotakBerlubang))
	require.NoError(t, err)

	raw, err := g.GeoJSON()
	require.NoError(t, err)

	assert.JSONEq(t, kotakBerlubang, string(raw))
}

func TestGeometri_MengandungHonoursHoles(t *testing.T) {
	g, err := ParseGeoJSON([]byte(kotakBerlubang))
	require.NoError(t, err)

	assert.True(t, g.Mengandung(0.3, 0.3), "di tepi, di luar lubang")
	assert.True(t, g.Mengandung(1.8, 1.8))
	assert.False(t, g.Mengandung(1, 1), "di dalam lubang")
	assert.False(t, g.Mengandung(3, 1), "di luar kotak")
	assert.False(t, g.Mengandung(-0.1, 1))
}

func TestGeometri_BBoxAndOverlap(t *testing.T) {
	g, err := ParseGeoJSON([]byte(kotakBerlubang))
	require.NoError(t, err)

	b := g.BBox()

	assert.Equal(t, BBox{MinLat: 0, MinLng: 0, MaxLat: 2, MaxLng: 2}, b)
	assert.True(t, b.Beririsan(BBox{MinLat: 1, MinLng: 1, MaxLat: 3, MaxLng: 3}))
	assert.True(t, b.Beririsan(BBox{MinLat: 2, MinLng: 2, MaxLat: 3, MaxLng: 3}), "bersentuhan di sudut")
	assert.False(t, b.Beririsan(BBox{MinLat: 2.1, MinLng: 0, MaxLat: 3, MaxLng: 1}))
}

func TestJarakKm(t *testing.T) {
	assert.Zero(t, JarakKm(5.5, 95.3, 5.5, 95.3))
	// Satu derajat bujur di khatulistiwa = 2πR/360 dengan R = 6371,0088 km.
	assert.InDelta(t, 111.195, JarakKm(0, 0, 0, 1), 0.001)
	// Jakarta - Bandung, sekitar 116 km menurut selisih koordinat yang dihitung tangan.
	assert.InDelta(t, 116.3, JarakKm(-6.2088, 106.8456, -6.9175, 107.6191), 0.5)
	assert.InDelta(t, JarakKm(1, 2, 3, 4), JarakKm(3, 4, 1, 2), 1e-9, "simetris")
}

func TestToleransiZoom_DecreasesWithZoom(t *testing.T) {
	assert.Greater(t, ToleransiZoom(5), ToleransiZoom(8))
	assert.Greater(t, ToleransiZoom(8), ToleransiZoom(10))
	assert.Zero(t, ToleransiZoom(14))
}

func TestSederhanakan(t *testing.T) {
	// Kotak 2x2 dengan titik-titik hampir segaris di sisi bawah, kanan, dan atas (menyimpang <= 0,001).
	g, err := ParseGeoJSON([]byte(`{"type":"Polygon","coordinates":[[
		[0,0],[1,0.001],[2,0],[2,1.0005],[2,2],[1,2.0005],[0,2],[0,0]]]}`))
	require.NoError(t, err)

	s := g.Sederhanakan(0.01)

	require.Len(t, s, 1)
	assert.Equal(t, Cincin{{0, 0}, {2, 0}, {2, 2}, {0, 2}, {0, 0}}, s[0][0], "tersisa empat sudut kotak + penutup")
	assert.Equal(t, g, g.Sederhanakan(0), "toleransi nol tanpa perubahan")
	assert.Len(t, g[0][0], 8, "sumber tidak dimodifikasi")
	assert.Len(t, g.Sederhanakan(0.0001)[0][0], 7, "simpangan 0,001 dan 0,0005 melebihi toleransi; titik segaris sempurna tetap dibuang")
}

func TestSederhanakan_KeepsTinyRingsAndDropsCollapsedHoles(t *testing.T) {
	g, err := ParseGeoJSON([]byte(`{"type":"Polygon","coordinates":[
		[[0,0],[0.001,0],[0.001,0.001],[0,0.001],[0,0]],
		[[0.0002,0.0002],[0.0004,0.0002],[0.0004,0.0004],[0.0002,0.0004],[0.0002,0.0002]]]}`))
	require.NoError(t, err)

	s := g.Sederhanakan(1)

	require.Len(t, s, 1)
	require.GreaterOrEqual(t, len(s[0][0]), 4, "cincin luar tidak boleh runtuh")
	assert.Len(t, s[0], 1, "lubang yang runtuh dibuang")
	_, err = s.GeoJSON()
	assert.NoError(t, err)
}

func TestCembungHull(t *testing.T) {
	hull, err := CembungHull([]Titik{{0, 0}, {4, 0}, {4, 4}, {0, 4}, {2, 2}, {1, 3}, {2, 0}})

	require.NoError(t, err)
	assert.Equal(t, Cincin{{0, 0}, {4, 0}, {4, 4}, {0, 4}, {0, 0}}, hull, "titik dalam dan titik di sisi dibuang")
	assert.Positive(t, luasCincin(hull), "berlawanan arah jarum jam")

	_, err = CembungHull([]Titik{{0, 0}, {1, 1}, {2, 2}})
	assert.ErrorIs(t, err, ErrGeometriTidakValid, "titik segaris")
	_, err = CembungHull([]Titik{{0, 0}, {1, 1}})
	assert.Error(t, err)
}
