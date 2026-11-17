package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRowScope_KoordinatorEWS(t *testing.T) {
	if testing.Short() {
		t.Skip("tes integrasi dilewati (-short, tanpa Docker)")
	}
	s := sharedSuite(t)

	type alertItem struct {
		KabupatenKode string `json:"kabupaten_kode"`
	}

	// Cari alert di luar kabupaten 1171 (kabupaten tugas koordinator).
	var kodeLuar string
	err := s.db.Raw(`SELECT kode FROM ews_alert WHERE kabupaten_kode <> '1171' AND kode IS NOT NULL LIMIT 1`).Scan(&kodeLuar).Error
	require.NoError(t, err)
	if kodeLuar == "" {
		t.Skip("tidak ada alert luar kabupaten 1171 pada data demo")
	}

	// Koordinator (1171) tidak boleh melihat alert kabupaten lain via GetByID -> 404.
	wGet := s.call(t, http.MethodGet, "/v1/ews/"+kodeLuar, emailAkui, nil)
	assert.Equal(t, http.StatusNotFound, wGet.Code)

	// Daftar alert untuk Koordinator hanya mengembalikan kabupaten 1171.
	wList := s.call(t, http.MethodGet, "/v1/ews", emailAkui, nil)
	require.Equal(t, http.StatusOK, wList.Code)
	items := data[[]alertItem](t, wList)
	for _, a := range items {
		assert.Equal(t, "1171", a.KabupatenKode, "koordinator hanya melihat alert kabupaten 1171")
	}

	// Meminta filter kabupaten lain mengembalikan daftar kosong.
	wFilter := s.call(t, http.MethodGet, "/v1/ews?kabupaten_kode=1108", emailAkui, nil)
	require.Equal(t, http.StatusOK, wFilter.Code)
	assert.Empty(t, data[[]alertItem](t, wFilter))
}
