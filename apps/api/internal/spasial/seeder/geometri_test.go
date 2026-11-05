package seeder

import (
	"testing"

	coldseeder "github.com/garda-inflasi/api/internal/cold-storage/seeder"
	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pusatUji = map[string]Koordinat{
	"A": {Lat: 5.0, Lng: 95.0},
	"B": {Lat: 5.0, Lng: 95.4},
	"C": {Lat: 4.0, Lng: 96.0},
}

func TestBuildGeometri_ValidDeterministicAndContainsAnchors(t *testing.T) {
	jangkar := map[string][]Koordinat{"A": {{Lat: 5.1, Lng: 94.9}, {Lat: 4.95, Lng: 95.12}}}

	first, err := BuildGeometri(pusatUji, jangkar)
	require.NoError(t, err)
	second, err := BuildGeometri(pusatUji, jangkar)
	require.NoError(t, err)

	assert.Equal(t, first, second, "deterministik")
	require.Len(t, first, 3)
	for kode, g := range first {
		raw, err := g.GeoJSON()
		require.NoError(t, err, kode)
		_, err = domain.ParseGeoJSON(raw)
		assert.NoError(t, err, "%s harus lolos validasi yang sama dengan batas basis data", kode)
		c := pusatUji[kode]
		assert.True(t, g.Mengandung(c.Lat, c.Lng), "%s memuat titik pusatnya", kode)
	}
	for _, j := range jangkar["A"] {
		assert.True(t, first["A"].Mengandung(j.Lat, j.Lng), "jangkar %v di dalam poligon A", j)
	}
}

func TestBuildGeometri_RadiusFollowsNearestNeighbourWithinBounds(t *testing.T) {
	g, err := BuildGeometri(pusatUji, nil)
	require.NoError(t, err)

	lebar := func(kode string) float64 { b := g[kode].BBox(); return b.MaxLng - b.MinLng }

	// A-B terpaut 0,4 derajat -> jari-jari 0,18 (<= 0,3); C jauh dari semuanya -> dibatasi jariMaks.
	assert.Less(t, lebar("A"), 2*jariRatio*0.4*1.15+0.001)
	assert.Greater(t, lebar("C"), lebar("A"), "wilayah terpencil lebih lebar")
	assert.LessOrEqual(t, lebar("C"), 2*jariMaks*1.15+0.001)
}

func TestBuildGeometri_SingleRegionUsesMaxRadius(t *testing.T) {
	g, err := BuildGeometri(map[string]Koordinat{"A": {Lat: 5, Lng: 95}}, nil)

	require.NoError(t, err)
	assert.Greater(t, g["A"].BBox().MaxLat-g["A"].BBox().MinLat, jariMin)
}

func TestSeedDemo_FacilitiesAndMarketsLieInsideTheirOwnRegionPolygon(t *testing.T) {
	db := seededDB(t, "spasial_geometri_anchor")
	ctx := t.Context()
	_, err := coldseeder.SeedReference(ctx, db)
	require.NoError(t, err)

	_, err = SeedDemo(ctx, db, seedNow)
	require.NoError(t, err)

	var rows []domain.WilayahGeometri
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 23, "setiap kabupaten/kota punya geometri")
	poligon := map[string]domain.Geometri{}
	for _, r := range rows {
		assert.Equal(t, SumberDemo, r.Sumber)
		assert.Equal(t, KeteranganGeometri, r.Keterangan)
		g, err := domain.ParseGeoJSON([]byte(r.GeoJSON))
		require.NoError(t, err, r.KabupatenKode)
		assert.Equal(t, g.BBox(), domain.BBox{MinLat: r.MinLat, MinLng: r.MinLng, MaxLat: r.MaxLat, MaxLng: r.MaxLng}, "bbox tersimpan = bbox GeoJSON")
		poligon[r.KabupatenKode] = g
	}

	var titik []struct {
		Sumber, Kabupaten string
		Lat, Lng          float64
	}
	require.NoError(t, db.Raw(`
		SELECT 'cold_storage' AS sumber, kabupaten_kode AS kabupaten, lat, lng FROM cold_storage
		UNION ALL SELECT 'pasar', kabupaten_kode, lat, lng FROM pasar`).Scan(&titik).Error)
	require.GreaterOrEqual(t, len(titik), 5+23)
	for _, tk := range titik {
		assert.True(t, poligon[tk.Kabupaten].Mengandung(tk.Lat, tk.Lng), "%s di %s (%.4f, %.4f) harus di dalam poligonnya", tk.Sumber, tk.Kabupaten, tk.Lat, tk.Lng)
	}
}

func TestMigration_RejectsMalformedGeometry(t *testing.T) {
	db := seededDB(t, "spasial_geometri_checks")
	insert := func(geojson string, minLat, minLng, maxLat, maxLng float64) error {
		return db.Exec(`INSERT INTO wilayah_geometri (kabupaten_kode, geojson, min_lat, min_lng, max_lat, max_lng)
			VALUES ('1171', ?::jsonb, ?, ?, ?, ?)`, geojson, minLat, minLng, maxLat, maxLng).Error
	}

	assert.ErrorContains(t, insert(`{"type":"Point","coordinates":[1,2]}`, 1, 1, 2, 2), "chk_wilayah_geometri_tipe")
	assert.ErrorContains(t, insert(`{"type":"Polygon","coordinates":[]}`, 1, 1, 2, 2), "chk_wilayah_geometri_tipe")
	assert.ErrorContains(t, insert(`{"type":"Polygon","coordinates":"x"}`, 1, 1, 2, 2), "chk_wilayah_geometri_tipe")
	assert.ErrorContains(t, insert(`{"type":"Polygon","coordinates":[[[0,0]]]}`, 2, 1, 1, 2), "chk_wilayah_geometri_bbox")
	assert.NoError(t, insert(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}`, 0, 0, 1, 1))
	assert.ErrorContains(t, db.Exec(`UPDATE wilayah_geometri SET sumber = 'LAIN'`).Error, "chk_wilayah_geometri_sumber")
}
