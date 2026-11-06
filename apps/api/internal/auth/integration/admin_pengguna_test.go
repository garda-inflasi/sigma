package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tes jalur admin atas pengguna yang tertinggal dari 01-E: PATCH /v1/users/:id harus sama tegasnya dengan
// PUT /v1/users/:id/roles, dan galat masukan tidak boleh menjadi 5xx.

func TestAdmin_PatchGantiPeranMencabutSesiSeketika(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	require.Equal(t, http.StatusOK, s.status(t, x.access))

	w := s.panggil(t, http.MethodPatch, "/v1/users/"+userBiasa, map[string]any{"peran": "OPS_KOORDINATOR"}, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access),
		"access token lama masih membawa izin peran sebelumnya: harus mati seperti pada PUT /roles")
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, x.refresh).Code)
	assert.EqualValues(t, 0, s.liveTokens(t, x.sid))
	assert.Equal(t, http.StatusOK, s.status(t, admin.access), "sesi admin tidak ikut tercabut")

	// Masuk lagi: peran baru berlaku di token yang baru.
	baru := s.mulaiSesi(t, emailBiasa, uaIPhone)
	assert.Equal(t, "OPS_KOORDINATOR", mustClaims(t, s.tokenMgr, baru.access).Role)
}

func TestAdmin_PatchTanpaPerubahanPeranTidakMengeluarkanPengguna(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)

	for _, body := range []map[string]any{
		{"nama": "Nama Baru"},
		{"peran": "OPS_MAHASISWA"}, // peran yang sama
		{"status": "AKTIF"},
	} {
		w := s.panggil(t, http.MethodPatch, "/v1/users/"+userBiasa, body, admin.access)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}

	assert.Equal(t, http.StatusOK, s.status(t, x.access), "perubahan biasa tidak boleh mengeluarkan pengguna")
	assert.EqualValues(t, 1, s.liveTokens(t, x.sid))
}

func TestAdmin_BuatPenggunaKataSandiLemahDijawab400(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	badan := func(password string) map[string]any {
		return map[string]any{
			"id": uid("u-baru-1"), "nama": "Baru", "email": "baru1@gardainflasi.id", "password": password,
			"peran": "OPS_MAHASISWA", "lapis": "OPERASIONAL", "lembaga": "BPPSDMP",
		}
	}

	w := s.panggil(t, http.MethodPost, "/v1/users", badan("semuahurufkecil"), admin.access)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	var n int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM pengguna WHERE id = garda_uuid_stabil('pengguna', 'u-baru-1')").Scan(&n).Error)
	assert.Zero(t, n, "pengguna dengan kata sandi lemah tidak dibuat")

	w = s.panggil(t, http.MethodPost, "/v1/users", badan("Kuat#12345"), admin.access)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestLogin_MencatatTerakhirMasukTanpaMenyentuhKolomLain(t *testing.T) {
	s := setupSuite(t)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET terakhir_masuk = NULL, nama = 'Nama Disimpan' WHERE id = ?", userBiasa).Error)

	s.mulaiSesi(t, emailBiasa, uaMac)

	var row struct {
		Nama          string
		TerakhirMasuk *string
		Status        string
		Peran         string
	}
	require.NoError(t, s.db.Raw("SELECT nama, terakhir_masuk::text AS terakhir_masuk, status, peran FROM pengguna WHERE id = ?", userBiasa).Scan(&row).Error)
	require.NotNil(t, row.TerakhirMasuk, "terakhir_masuk terisi sesudah login")
	assert.Equal(t, "Nama Disimpan", row.Nama)
	assert.Equal(t, "AKTIF", row.Status)
	assert.Equal(t, "OPS_MAHASISWA", row.Peran)
}
