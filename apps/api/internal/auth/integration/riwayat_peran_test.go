package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Riwayat peran per pengguna (GET /v1/users/:id/riwayat-peran), dibaca dari audit_log.
// Pelaku: test-user-001 "Test User" (A) dan adm-2 "Admin Kedua" (B), keduanya ADMIN_SISTEM.

type aktorRiwayat struct {
	ID   *string `json:"id"`
	Nama *string `json:"nama"`
}

type entriRiwayat struct {
	ID    string        `json:"id"`
	Waktu time.Time     `json:"waktu"`
	Aksi  string        `json:"aksi"`
	Dari  *string       `json:"dari"`
	Ke    string        `json:"ke"`
	Aktor *aktorRiwayat `json:"aktor"`
}

type hasilRiwayat struct {
	Items     []entriRiwayat `json:"items"`
	Terpotong bool           `json:"terpotong"`
}

func seedDuaAdmin(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedData(t, db)
	require.NoError(t, db.Exec(`INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status)
		SELECT 'adm-2', 'Admin Kedua', 'adm2@gardainflasi.id', password_hash, 'ADMIN_SISTEM', 'SISTEM', 'BPPSDMP', 'AKTIF' FROM pengguna WHERE id = 'test-user-001'`).Error)
}

func (s *Suite) riwayat(t *testing.T, token, id, query string) hasilRiwayat {
	t.Helper()
	path := "/v1/users/" + id + "/riwayat-peran"
	if query != "" {
		path += "?" + query
	}
	w := s.panggil(t, http.MethodGet, path, nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var h hasilRiwayat
	s.decodeData(t, w.Body.Bytes(), &h)
	require.NotNil(t, h.Items, "items larik kosong, bukan null")
	return h
}

func (s *Suite) buatPengguna(t *testing.T, token, id, peran, lapis string) {
	t.Helper()
	w := s.panggil(t, http.MethodPost, "/v1/users", map[string]any{
		"id": id, "nama": "Pengguna " + id, "email": id + "@gardainflasi.id", "password": "Kuat#12345",
		"peran": peran, "lapis": lapis, "lembaga": "BPPSDMP",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func (s *Suite) ubahPeran(t *testing.T, method, token, id, peran string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	path := "/v1/users/" + id
	if method == http.MethodPut {
		path += "/roles"
	}
	w := s.panggil(t, method, path, map[string]any{"peran": peran}, token)
	require.Equal(t, wantStatus, w.Code, w.Body.String())
	return w
}

func str(s string) *string { return &s }

// ringkas memetakan entri ke (aksi, dari, ke, nama aktor) untuk dibandingkan.
func ringkas(h hasilRiwayat) [][4]string {
	out := make([][4]string, len(h.Items))
	for i, e := range h.Items {
		dari, nama := "", ""
		if e.Dari != nil {
			dari = *e.Dari
		}
		if e.Aktor != nil && e.Aktor.Nama != nil {
			nama = *e.Aktor.Nama
		}
		out[i] = [4]string{e.Aksi, dari, e.Ke, nama}
	}
	return out
}

func TestRiwayatPeran_SetiapJalurTercatatBerurutanDenganNamaAktor(t *testing.T) {
	s := setupSuiteWith(t, seedDuaAdmin)
	a := s.mulaiSesi(t, emailAdmin, uaMac).access
	b := s.mulaiSesi(t, "adm2@gardainflasi.id", uaIPhone).access

	s.buatPengguna(t, a, "riw-1", "OPS_MAHASISWA", "OPERASIONAL")                  // CREATE_USER: peran awal
	s.ubahPeran(t, http.MethodPut, a, "riw-1", "MANAJER_DATA", http.StatusOK)      // ASSIGN_ROLE
	s.ubahPeran(t, http.MethodPatch, b, "riw-1", "OPS_KOORDINATOR", http.StatusOK) // UPDATE_USER oleh admin lain
	s.panggil(t, http.MethodPatch, "/v1/users/riw-1", map[string]any{"nama": "Nama Baru"}, a)
	s.ubahPeran(t, http.MethodPut, a, "riw-1", "OPS_KOORDINATOR", http.StatusOK)     // peran sama: bukan perubahan
	s.ubahPeran(t, http.MethodPatch, a, "riw-1", "TIDAK_ADA", http.StatusBadRequest) // ditolak: tak tercatat
	s.ubahPeran(t, http.MethodPut, a, "riw-1", "TIDAK_ADA", http.StatusBadRequest)
	w := s.panggil(t, http.MethodPatch, "/v1/users/riw-1", map[string]any{"peran": "OPS_AGREGATOR", "lembaga": "FMIPA USK"}, a)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	// Aksi lain pada pengguna yang sama tidak masuk riwayat peran.
	s.panggil(t, http.MethodPost, "/v1/users/bulk", map[string]any{"action": "nonaktifkan", "ids": []string{"riw-1"}}, a)
	s.panggil(t, http.MethodPost, "/v1/users/bulk", map[string]any{"action": "aktifkan", "ids": []string{"riw-1"}}, a)
	s.panggil(t, http.MethodPost, "/v1/users/riw-1/revoke-sessions", nil, a)
	s.panggil(t, http.MethodPost, "/v1/users/riw-1/reset-mfa", nil, a)
	s.panggil(t, http.MethodPost, "/v1/users/riw-1/buka-kunci", nil, a)

	h := s.riwayat(t, a, "riw-1", "")

	assert.Equal(t, [][4]string{
		{"UPDATE_USER", "OPS_KOORDINATOR", "OPS_AGREGATOR", "Test User"},
		{"UPDATE_USER", "MANAJER_DATA", "OPS_KOORDINATOR", "Admin Kedua"},
		{"ASSIGN_ROLE", "OPS_MAHASISWA", "MANAJER_DATA", "Test User"},
		{"CREATE_USER", "", "OPS_MAHASISWA", "Test User"},
	}, ringkas(h), "terbaru dulu; hanya perubahan peran sungguhan")
	assert.False(t, h.Terpotong)

	for i, e := range h.Items {
		assert.NotEmpty(t, e.ID, "id baris audit sumber")
		require.NotNil(t, e.Aktor, "entri %d", i)
		assert.NotNil(t, e.Aktor.ID)
		if i > 0 {
			assert.True(t, !e.Waktu.After(h.Items[i-1].Waktu), "waktu menurun")
		}
	}
	assert.Nil(t, h.Items[3].Dari, "peran awal: dari null")
	assert.Equal(t, "adm-2", *h.Items[1].Aktor.ID)
	assert.Equal(t, "test-user-001", *h.Items[0].Aktor.ID)

	t.Run("setiap id dapat dibuka di audit", func(t *testing.T) {
		for _, e := range h.Items {
			w := s.panggil(t, http.MethodGet, "/v1/audit/"+e.ID, nil, a)
			assert.Equal(t, http.StatusOK, w.Code, e.ID)
		}
	})

	t.Run("limit memotong dan menandai terpotong", func(t *testing.T) {
		dua := s.riwayat(t, a, "riw-1", "limit=2")
		assert.Len(t, dua.Items, 2)
		assert.True(t, dua.Terpotong)
		assert.Equal(t, h.Items[:2][1].ID, dua.Items[1].ID)
		tepat := s.riwayat(t, a, "riw-1", "limit=4")
		assert.Len(t, tepat.Items, 4)
		assert.False(t, tepat.Terpotong, "tepat habis bukan terpotong")
	})
}

// Setiap jalur yang mengubah peran menyimpan peran sebelum dan sesudah di audit; riwayat bergantung padanya.
func TestRiwayatPeran_AuditMenyimpanPeranSebelumDanSesudahDiSetiapJalur(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac).access
	s.buatPengguna(t, a, "riw-2", "OPS_MAHASISWA", "OPERASIONAL")
	s.ubahPeran(t, http.MethodPut, a, "riw-2", "MANAJER_DATA", http.StatusOK)
	s.ubahPeran(t, http.MethodPatch, a, "riw-2", "OPS_AGREGATOR", http.StatusOK)

	var baris []struct {
		Aksi    string
		Sebelum *string
		Sesudah *string
	}
	require.NoError(t, s.db.Raw(`SELECT aksi, sebelum->>'peran' AS sebelum, sesudah->>'peran' AS sesudah FROM audit_log
		WHERE entitas = 'pengguna' AND entitas_id = 'riw-2' AND aksi IN ('CREATE_USER', 'ASSIGN_ROLE', 'UPDATE_USER') ORDER BY waktu`).Scan(&baris).Error)

	require.Len(t, baris, 3)
	assert.Equal(t, "CREATE_USER", baris[0].Aksi)
	assert.Nil(t, baris[0].Sebelum, "pembuatan: tak ada keadaan sebelum")
	assert.Equal(t, str("OPS_MAHASISWA"), baris[0].Sesudah)
	assert.Equal(t, "ASSIGN_ROLE", baris[1].Aksi)
	assert.Equal(t, str("OPS_MAHASISWA"), baris[1].Sebelum)
	assert.Equal(t, str("MANAJER_DATA"), baris[1].Sesudah)
	assert.Equal(t, "UPDATE_USER", baris[2].Aksi)
	assert.Equal(t, str("MANAJER_DATA"), baris[2].Sebelum)
	assert.Equal(t, str("OPS_AGREGATOR"), baris[2].Sesudah)
}

// Jalur baru yang mencatat snapshot dengan peran (undangan, impor CSV) ikut tanpa mengubah kueri; baris yang
// bukan perubahan peran pengguna ini tidak ikut.
func TestRiwayatPeran_MembacaAksiApaPunYangMencatatPeranDanMenyaringSisanya(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac).access
	s.buatPengguna(t, a, "riw-3", "OPS_MAHASISWA", "OPERASIONAL")

	sisip := func(aksi, entitas, entitasID, hasil, sebelum, sesudah, waktu string) {
		t.Helper()
		var sb, sd any
		if sebelum != "" {
			sb = sebelum
		}
		if sesudah != "" {
			sd = sesudah
		}
		require.NoError(t, s.db.Exec(`INSERT INTO audit_log (id, waktu, aksi, entitas, entitas_id, hasil, sebelum, sesudah)
			VALUES (md5(random()::text || clock_timestamp()::text), ?::timestamptz, ?, ?, ?, ?, ?::jsonb, ?::jsonb)`,
			waktu, aksi, entitas, entitasID, hasil, sb, sd).Error)
	}
	sisip("INVITE_ACCEPT", "pengguna", "riw-3", "BERHASIL", `{"peran":"OPS_MAHASISWA"}`, `{"peran":"OPS_AGREGATOR"}`, "2099-01-01 00:00:00+00")
	sisip("UPDATE_USER", "pengguna", "riw-3", "GAGAL", `{"peran":"OPS_AGREGATOR"}`, `{"peran":"ADMIN_SISTEM"}`, "2099-01-02 00:00:00+00")                           // gagal
	sisip("UPDATE_USER", "pengguna", "riw-3", "DITOLAK", `{"peran":"OPS_AGREGATOR"}`, `{"peran":"ADMIN_SISTEM"}`, "2099-01-03 00:00:00+00")                         // ditolak
	sisip("UPDATE_USER", "pengguna", "riw-lain", "BERHASIL", `{"peran":"OPS_AGREGATOR"}`, `{"peran":"ADMIN_SISTEM"}`, "2099-01-04 00:00:00+00")                     // pengguna lain
	sisip("UPDATE_USER", "entri_harga", "riw-3", "BERHASIL", `{"peran":"OPS_AGREGATOR"}`, `{"peran":"ADMIN_SISTEM"}`, "2099-01-05 00:00:00+00")                     // entitas lain
	sisip("UPDATE_USER", "pengguna", "riw-3", "BERHASIL", `{"peran":"OPS_AGREGATOR","nama":"A"}`, `{"peran":"OPS_AGREGATOR","nama":"B"}`, "2099-01-06 00:00:00+00") // peran tetap
	sisip("UPDATE_USER", "pengguna", "riw-3", "BERHASIL", `{"nama":"A"}`, `{"nama":"B"}`, "2099-01-07 00:00:00+00")                                                 // tanpa peran
	sisip("LOGIN", "pengguna", "riw-3", "BERHASIL", "", "", "2099-01-08 00:00:00+00")                                                                               // tanpa snapshot
	sisip("UPDATE_USER", "pengguna", "riw-3", "BERHASIL", "", `{"peran":"MANAJER_RISIKO"}`, "2099-01-09 00:00:00+00")                                               // peran awal dari aksi lain

	h := s.riwayat(t, a, "riw-3", "")

	assert.Equal(t, [][4]string{
		{"UPDATE_USER", "", "MANAJER_RISIKO", ""},
		{"INVITE_ACCEPT", "OPS_MAHASISWA", "OPS_AGREGATOR", ""},
		{"CREATE_USER", "", "OPS_MAHASISWA", "Test User"},
	}, ringkas(h))
	assert.Nil(t, h.Items[0].Aktor, "aksi tanpa pelaku: aktor null")
}

func TestRiwayatPeran_AktorYangDihapusTetapTerbacaNamanya(t *testing.T) {
	s := setupSuiteWith(t, seedDuaAdmin)
	a := s.mulaiSesi(t, emailAdmin, uaMac).access
	b := s.mulaiSesi(t, "adm2@gardainflasi.id", uaIPhone).access
	s.buatPengguna(t, a, "riw-4", "OPS_MAHASISWA", "OPERASIONAL")
	s.ubahPeran(t, http.MethodPut, b, "riw-4", "MANAJER_DATA", http.StatusOK)

	w := s.panggil(t, http.MethodDelete, "/v1/users/adm-2", nil, a)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	h := s.riwayat(t, a, "riw-4", "")

	require.Len(t, h.Items, 2)
	require.NotNil(t, h.Items[0].Aktor)
	assert.Nil(t, h.Items[0].Aktor.ID, "akun aktor sudah tiada")
	assert.Equal(t, str("Admin Kedua"), h.Items[0].Aktor.Nama, "snapshot nama di audit tetap terbaca")
}

func TestRiwayatPeran_PenggunaTanpaPerubahanKosong(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac).access

	h := s.riwayat(t, a, userBiasa, "") // akun seed: tanpa audit pembuatan

	assert.Empty(t, h.Items)
	assert.False(t, h.Terpotong)
}

func TestRiwayatPeran_PenggunaTidakAdaDanOtorisasiDanMasukan(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac).access
	biasa := s.mulaiSesi(t, emailBiasa, uaIPhone).access
	s.buatPengguna(t, a, "riw-5", "OPS_MAHASISWA", "OPERASIONAL")
	s.ubahPeran(t, http.MethodPut, a, "riw-5", "MANAJER_DATA", http.StatusOK)

	t.Run("pengguna tidak ada", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, s.panggil(t, http.MethodGet, "/v1/users/tidak-ada/riwayat-peran", nil, a).Code)
	})
	t.Run("pengguna yang sudah dihapus tidak lagi punya riwayat", func(t *testing.T) {
		s.buatPengguna(t, a, "riw-6", "OPS_MAHASISWA", "OPERASIONAL")
		require.Equal(t, http.StatusOK, s.panggil(t, http.MethodDelete, "/v1/users/riw-6", nil, a).Code)
		assert.Equal(t, http.StatusNotFound, s.panggil(t, http.MethodGet, "/v1/users/riw-6/riwayat-peran", nil, a).Code)
	})
	t.Run("tanpa pengguna.kelola 403", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/users/riw-5/riwayat-peran", nil, biasa).Code)
	})
	t.Run("tanpa token 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/users/riw-5/riwayat-peran", nil, "").Code)
	})
	t.Run("limit di luar rentang 400", func(t *testing.T) {
		for _, q := range []string{"limit=201", "limit=-1", "limit=abc", "limit=1.5"} {
			assert.Equal(t, http.StatusBadRequest, s.panggil(t, http.MethodGet, "/v1/users/riw-5/riwayat-peran?"+q, nil, a).Code, q)
		}
		assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/users/riw-5/riwayat-peran?limit=200", nil, a).Code)
	})
}
