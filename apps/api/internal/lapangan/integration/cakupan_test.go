package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRowScope_KoordinatorTerbatasKabupaten(t *testing.T) {
	if testing.Short() {
		t.Skip("tes integrasi dilewati (-short, tanpa Docker)")
	}
	s := sharedSuite(t)

	// Buat tugas di kabupaten lain (mis. 1173 Lhokseumawe) milik mahasiswa_10.
	// Koordinator Banda Aceh (1171) tidak boleh melihat atau menutupnya.
	var kodeLuar string
	err := s.db.Raw(`
		SELECT t.kode FROM tugas_lapangan t
		  JOIN pasar p ON p.kode = t.pasar_id
		 WHERE p.kabupaten_kode <> '1171' AND t.kode IS NOT NULL
		 LIMIT 1`).Scan(&kodeLuar).Error
	require.NoError(t, err)
	if kodeLuar == "" {
		t.Skip("tidak ada tugas luar kabupaten 1171 pada data demo")
	}

	// Koordinator (1171) tidak boleh membuka detail tugas kabupaten lain -> 404.
	wGet := s.call(t, http.MethodGet, "/v1/lapangan/tugas/"+kodeLuar, emailKoordinator, nil)
	assert.Equal(t, http.StatusNotFound, wGet.Code, "koordinator 1171 tidak boleh membuka tugas luar kabupaten")

	// Aksi bulk pada tugas kabupaten lain -> gagal = 1.
	wBulk := s.call(t, http.MethodPost, "/v1/lapangan/tugas/bulk", emailKoordinator, map[string]any{
		"action": "unduh_data_acuan",
		"ids":    []string{kodeLuar},
	})
	require.Equal(t, http.StatusOK, wBulk.Code)
	assert.Equal(t, hasilBulk{Berhasil: 0, Gagal: 1}, data[hasilBulk](t, wBulk))
}
