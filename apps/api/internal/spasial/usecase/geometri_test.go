package usecase

import (
	"errors"
	"testing"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kotakA berada di bujur 0-2, kotakB di bujur 1-3 (tumpang tindih di bujur 1-2); lintang 0-2.
const (
	kotakA = `{"type":"Polygon","coordinates":[[[0,0],[2,0],[2,2],[0,2],[0,0]]]}`
	kotakB = `{"type":"Polygon","coordinates":[[[1,0],[3,0],[3,2],[1,2],[1,0]]]}`
)

func geometriUji(kode, geojson string) domain.GeometriWilayah {
	return domain.GeometriWilayah{
		WilayahGeometri: domain.WilayahGeometri{KabupatenKode: kode, GeoJSON: geojson, Sumber: "DEMO"},
		Nama:            "Wilayah " + kode, Level: "kabupaten",
	}
}

func pusat(kode string, lat, lng float64) domain.Marker {
	return domain.Marker{Wilayah: domain.Wilayah{Kode: kode, Nama: "Wilayah " + kode, Lat: lat, Lng: lng}}
}

func newUC(repo *fakeRepository) *UseCase { return New(repo, logger.NewDefault()) }

func TestGeometri_PassesBBoxAndSimplifiesByZoom(t *testing.T) {
	ramai := `{"type":"Polygon","coordinates":[[[0,0],[1,0.0001],[2,0],[2,2],[0,2],[0,0]]]}`
	repo := &fakeRepository{geometri: []domain.GeometriWilayah{geometriUji("A", ramai)}}
	bbox := &domain.BBox{MinLat: 0, MinLng: 0, MaxLat: 1, MaxLng: 1}

	rendah, err := newUC(repo).Geometri(t.Context(), 4, bbox)
	require.NoError(t, err)
	assert.Equal(t, bbox, repo.bbox, "bbox diteruskan ke repository")
	tinggi, err := newUC(repo).Geometri(t.Context(), 14, nil)
	require.NoError(t, err)

	require.Len(t, rendah, 1)
	g1, _ := domain.ParseGeoJSON(rendah[0].Geometri)
	g2, _ := domain.ParseGeoJSON(tinggi[0].Geometri)
	assert.Len(t, g1[0][0], 5, "titik hampir segaris dibuang pada zoom rendah")
	assert.Len(t, g2[0][0], 6, "zoom tinggi menjaga semua titik")
}

func TestGeometri_NegativeZoomUsesDefaultAndBadGeometryFails(t *testing.T) {
	repo := &fakeRepository{geometri: []domain.GeometriWilayah{geometriUji("A", kotakA)}}
	got, err := newUC(repo).Geometri(t.Context(), -1, &domain.BBox{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, &domain.BBox{}, repo.bbox)

	repo.geometri = []domain.GeometriWilayah{geometriUji("RUSAK", `{"type":"Point"}`)}
	_, err = newUC(repo).Geometri(t.Context(), 8, nil)
	assert.ErrorIs(t, err, domain.ErrGeometriTidakValid)

	repo.err = errors.New("db mati")
	_, err = newUC(repo).Geometri(t.Context(), 8, nil)
	assert.Error(t, err)
}

func TestLokasi(t *testing.T) {
	repo := &fakeRepository{
		markers:  []domain.Marker{pusat("A", 1, 0.5), pusat("B", 1, 2.5), pusat("C", 5, 5)},
		geometri: []domain.GeometriWilayah{geometriUji("A", kotakA), geometriUji("B", kotakB)},
	}
	uc := newUC(repo)

	t.Run("di dalam satu poligon", func(t *testing.T) {
		l, err := uc.Lokasi(t.Context(), 1, 0.3)

		require.NoError(t, err)
		assert.Equal(t, "A", l.Kode)
		assert.True(t, l.DiDalam)
	})

	t.Run("poligon tumpang tindih: pusat terdekat menang", func(t *testing.T) {
		// (1, 1.8) berada di A dan B; pusat B (1, 2.5) lebih dekat daripada A (1, 0.5).
		l, err := uc.Lokasi(t.Context(), 1, 1.8)

		require.NoError(t, err)
		assert.Equal(t, "B", l.Kode)
		assert.True(t, l.DiDalam)
	})

	t.Run("di luar semua poligon: pusat terdekat", func(t *testing.T) {
		l, err := uc.Lokasi(t.Context(), 4.8, 4.9)

		require.NoError(t, err)
		assert.Equal(t, "C", l.Kode)
		assert.False(t, l.DiDalam)
		assert.InDelta(t, domain.JarakKm(4.8, 4.9, 5, 5), l.JarakPusatKm, 1e-9)
	})

	t.Run("belum ada wilayah", func(t *testing.T) {
		_, err := newUC(&fakeRepository{}).Lokasi(t.Context(), 1, 1)

		assert.ErrorIs(t, err, ErrTidakAdaWilayah)
	})

	t.Run("geometri rusak menggagalkan pencarian", func(t *testing.T) {
		bad := &fakeRepository{markers: repo.markers, geometri: []domain.GeometriWilayah{geometriUji("A", `{}`)}}

		_, err := newUC(bad).Lokasi(t.Context(), 1, 1)

		assert.ErrorIs(t, err, domain.ErrGeometriTidakValid)
	})
}
