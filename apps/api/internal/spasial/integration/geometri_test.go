package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type featureCollection struct {
	Type     string `json:"type"`
	Features []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Properties struct {
			Kode      string `json:"kode"`
			Nama      string `json:"nama"`
			Level     string `json:"level"`
			Sumber    string `json:"sumber"`
			Perkiraan bool   `json:"perkiraan"`
		} `json:"properties"`
		Geometry json.RawMessage `json:"geometry"`
	} `json:"features"`
}

func vertices(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	g, err := domain.ParseGeoJSON(raw)
	require.NoError(t, err)
	n := 0
	for _, p := range g {
		for _, c := range p {
			n += len(c)
		}
	}
	return n
}

func TestGeometri_Authorization(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{"/v1/spasial/geometri", "/v1/spasial/lokasi?lat=5.5&lng=95.3"} {
		assert.Equal(t, http.StatusUnauthorized, s.call(t, path, "").Code, path)
		assert.Equal(t, http.StatusForbidden, s.call(t, path, emailTanpaIzin).Code, path)
		assert.Equal(t, http.StatusOK, s.call(t, path, emailLihat).Code, path)
	}
}

func TestGeometri_FeatureCollectionIsLabelledAsApproximate(t *testing.T) {
	s := sharedSuite(t)

	w := s.call(t, "/v1/spasial/geometri", emailLihat)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	fc := data[featureCollection](t, w)
	assert.Equal(t, "FeatureCollection", fc.Type)
	require.Len(t, fc.Features, 23, "setiap kabupaten/kota punya geometri")
	levels := map[string]int{}
	for _, f := range fc.Features {
		assert.Equal(t, "Feature", f.Type)
		assert.Equal(t, f.Properties.Kode, f.ID)
		assert.Equal(t, "DEMO", f.Properties.Sumber)
		assert.True(t, f.Properties.Perkiraan, "tidak boleh tampil seperti batas resmi")
		assert.NotEmpty(t, f.Properties.Nama)
		levels[f.Properties.Level]++
		_, err := domain.ParseGeoJSON(f.Geometry)
		assert.NoError(t, err, f.ID)
	}
	assert.Equal(t, map[string]int{"kabupaten": 18, "kota": 5}, levels)
}

func TestGeometri_ZoomSimplifiesAndFullDetailKeepsEveryVertex(t *testing.T) {
	s := sharedSuite(t)
	count := func(zoom string) int {
		fc := data[featureCollection](t, s.call(t, "/v1/spasial/geometri?zoom="+zoom, emailLihat))
		total := 0
		for _, f := range fc.Features {
			total += vertices(t, f.Geometry)
		}
		return total
	}

	rendah, tinggi := count("4"), count("14")

	assert.Less(t, rendah, tinggi, "zoom rendah menyederhanakan geometri")
	assert.Equal(t, tinggi, count("11"), "zoom >= 11 tanpa penyederhanaan")
}

func TestGeometri_BBoxFiltersByOverlap(t *testing.T) {
	s := sharedSuite(t)
	kode := func(query string) []string {
		w := s.call(t, "/v1/spasial/geometri"+query, emailLihat)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var out []string
		for _, f := range data[featureCollection](t, w).Features {
			out = append(out, f.ID)
		}
		return out
	}

	sekitarBandaAceh := kode("?bbox=95.30,5.53,95.34,5.57")
	assert.Contains(t, sekitarBandaAceh, "1171")
	assert.NotContains(t, sekitarBandaAceh, "1175", "Subulussalam jauh di selatan")
	assert.Less(t, len(sekitarBandaAceh), 23)

	assert.Len(t, kode("?bbox=90,0,100,8"), 23, "kotak yang memuat seluruh Aceh")
	assert.Empty(t, kode("?bbox=100,10,101,11"), "kotak di luar Aceh")
}

func TestGeometri_RejectsInvalidParams(t *testing.T) {
	s := sharedSuite(t)

	for _, path := range []string{
		"/v1/spasial/geometri?zoom=99",
		"/v1/spasial/geometri?zoom=-1",
		"/v1/spasial/geometri?zoom=abc",
		"/v1/spasial/geometri?bbox=1,2,3",
		"/v1/spasial/geometri?bbox=a,b,c,d",
		"/v1/spasial/geometri?bbox=96,5,95,6",
		"/v1/spasial/geometri?bbox=95,6,96,5",
		"/v1/spasial/geometri?bbox=95,5,96,95",
		"/v1/spasial/lokasi",
		"/v1/spasial/lokasi?lat=5.5",
		"/v1/spasial/lokasi?lat=95&lng=95",
		"/v1/spasial/lokasi?lat=5&lng=300",
	} {
		assert.Equal(t, http.StatusBadRequest, s.call(t, path, emailLihat).Code, path)
	}
}

func TestLokasi(t *testing.T) {
	s := sharedSuite(t)
	type lokasi struct {
		Kode         string  `json:"kode"`
		Nama         string  `json:"nama"`
		DiDalam      bool    `json:"di_dalam"`
		JarakPusatKm float64 `json:"jarak_pusat_km"`
	}

	t.Run("titik di dalam poligon", func(t *testing.T) {
		// Pasar Inpres Takengon (master) adalah jangkar poligon Aceh Tengah.
		w := s.call(t, "/v1/spasial/lokasi?lat=4.627&lng=96.846", emailLihat)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		l := data[lokasi](t, w)

		assert.Equal(t, "1106", l.Kode)
		assert.True(t, l.DiDalam)
		assert.Less(t, l.JarakPusatKm, 2.0)
	})

	t.Run("titik di laut jatuh ke wilayah terdekat", func(t *testing.T) {
		l := data[lokasi](t, s.call(t, "/v1/spasial/lokasi?lat=6.5&lng=95.0", emailLihat))

		assert.False(t, l.DiDalam)
		assert.Equal(t, "1172", l.Kode, "Sabang paling dekat dengan titik di utara Banda Aceh")
		assert.Greater(t, l.JarakPusatKm, 50.0)
	})
}
