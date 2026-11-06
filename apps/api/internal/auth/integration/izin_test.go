package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/repository/postgres"
	"github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
)

// 02-C: RBAC berbasis basis data. Rute kpi (kpi.lihat) dipakai sebagai contoh rute yang dikunci izin:
// OPS_MAHASISWA (pengguna biasa di suite ini) tidak memilikinya secara bawaan.

const (
	userAdminID = "test-user-001"
	adminKedua  = "admin-kedua"
	emailKedua  = "admin.kedua@gardainflasi.id"
)

// suiteDenganIzinTTL seperti setupSuite dengan umur cache izin tertentu (perubahan di luar proses ini
// berlaku paling lambat sebesar itu).
func suiteDenganIzinTTL(t *testing.T, ttl time.Duration) *Suite {
	t.Helper()
	cfg := routes.ConfigFromEnv(os.Getenv)
	cfg.IzinCacheTTL = ttl
	return newSuite(t, seedData, routes.Options{UseCaseConfig: cfg})
}

func (s *Suite) kpi(t *testing.T, access string) int {
	t.Helper()
	return s.panggil(t, http.MethodGet, "/v1/kpi", nil, access).Code
}

// kodeDari adalah kolom `code` pada amplop jawaban.
func (s *Suite) kodeDari(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	kode, _ := s.parseResponse(t, w)["code"].(string)
	return kode
}

// segarkan menukar refresh token seperti klien web (refresh senyap) dan mengembalikan sesi baru.
func (s *Suite) segarkan(t *testing.T, sesi sesiUji) sesiUji {
	t.Helper()
	w := s.refreshRaw(t, sesi.refresh)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.dataObj(t, w)
	baru := sesiUji{access: data["access_token"].(string), refresh: sesi.refresh, sid: sesi.sid}
	if r, _ := data["refresh_token"].(string); r != "" {
		baru.refresh = r
	}
	return baru
}

func (s *Suite) izinToken(t *testing.T, access string) []string {
	t.Helper()
	return mustClaims(t, s.tokenMgr, access).Permissions
}

func (s *Suite) ubahMatriks(t *testing.T, access, peran string, versi int, izin []string) *httptest.ResponseRecorder {
	t.Helper()
	return s.panggil(t, http.MethodPut, "/v1/roles/"+peran+"/permissions", map[string]any{"permissions": izin, "versi": versi}, access)
}

func (s *Suite) beriIzin(t *testing.T, access, userID string, badan map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return s.panggil(t, http.MethodPost, "/v1/users/"+userID+"/permissions", badan, access)
}

// tunggu mengulang kondisi sampai benar dan mengembalikan lamanya menunggu; gagal bila melewati batas.
func tunggu(t *testing.T, batas time.Duration, pesan string, kondisi func() bool) time.Duration {
	t.Helper()
	mulai := time.Now()
	for !kondisi() {
		if time.Since(mulai) > batas {
			t.Fatalf("melewati %s: %s", batas, pesan)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return time.Since(mulai)
}

func (s *Suite) tambahAdminKedua(t *testing.T) {
	t.Helper()
	require.NoError(t, s.db.Exec(`INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status)
		SELECT ?, 'Admin Kedua', ?, password_hash, peran, lapis, lembaga, 'AKTIF' FROM pengguna WHERE id = ?`,
		adminKedua, emailKedua, userAdminID).Error)
}

func (s *Suite) pemegangKelola(t *testing.T) int64 {
	t.Helper()
	n, err := postgres.New(s.db).HitungPemegangPengelolaan(context.Background())
	require.NoError(t, err)
	return n
}

func daftarString(t *testing.T, v any) []string {
	t.Helper()
	raw, ok := v.([]any)
	require.True(t, ok, "bukan larik: %v", v)
	out := make([]string, len(raw))
	for i, r := range raw {
		out[i] = r.(string)
	}
	return out
}

// ── Katalog dan matriks ───────────────────────────────────────────────

func TestIzin_RolesDanPermissionsMembacaBasisDataDenganVersiDanBawaan(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.panggil(t, http.MethodGet, "/v1/roles", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	roles := s.parseResponse(t, w)["data"].([]any)
	require.Len(t, roles, 14)
	for i, r := range roles {
		role := r.(map[string]any)
		id := role["id"].(string)
		assert.Equal(t, usecase.DaftarPeran()[i].Kunci, id, "urutan baku")
		assert.ElementsMatch(t, usecase.IzinBawaan(id), daftarString(t, role["permissions"]), id)
		assert.ElementsMatch(t, usecase.IzinBawaan(id), daftarString(t, role["bawaan"]), id)
		assert.EqualValues(t, 1, role["versi"])
		assert.NotEmpty(t, role["label"])
		assert.NotEmpty(t, role["lapis"])
	}

	w = s.panggil(t, http.MethodGet, "/v1/permissions", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code)
	perms := s.parseResponse(t, w)["data"].([]any)
	require.Len(t, perms, 35)
	first := perms[0].(map[string]any)
	assert.NotEmpty(t, first["deskripsi"])
	assert.NotEmpty(t, first["kelompok"])
}

func TestIzin_EdisiMatriksBerlakuSeketikaLaluTokenBasiDiperbaruiLewatRefresh(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	require.Equal(t, http.StatusForbidden, s.kpi(t, x.access))
	bawaan := usecase.IzinBawaan("OPS_MAHASISWA")

	w := s.ubahMatriks(t, admin.access, "OPS_MAHASISWA", 1, append(slices.Clone(bawaan), "kpi.lihat"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 2, s.dataObj(t, w)["versi"])

	// Token lama masih membawa izin sebelum perubahan: ditolak dengan kode khusus agar klien menyegarkannya.
	w = s.panggil(t, http.MethodGet, "/v1/kpi", nil, x.access)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, middleware.PermissionsChangedCode, s.kodeDari(t, w))

	x = s.segarkan(t, x)
	assert.Contains(t, s.izinToken(t, x.access), "kpi.lihat", "token hasil refresh membawa izin baru")
	assert.Equal(t, http.StatusOK, s.kpi(t, x.access))

	// Pulihkan matriks bawaan: berlaku seketika juga.
	w = s.ubahMatriks(t, admin.access, "OPS_MAHASISWA", 2, bawaan)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 3, s.dataObj(t, w)["versi"])
	assert.Equal(t, http.StatusUnauthorized, s.kpi(t, x.access), "izin yang dicabut tidak menunggu token 15 menit habis")
	x = s.segarkan(t, x)
	assert.Equal(t, http.StatusForbidden, s.kpi(t, x.access))
	assert.NotContains(t, s.izinToken(t, x.access), "kpi.lihat")
}

func TestIzin_PerubahanSQLDariLuarProsesBerlakuPalingLambatSetelahTTL(t *testing.T) {
	const ttl = 300 * time.Millisecond
	s := suiteDenganIzinTTL(t, ttl)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	require.Equal(t, http.StatusForbidden, s.kpi(t, x.access))

	require.NoError(t, s.db.Exec("INSERT INTO role_permissions (role_kunci, permission_nama) VALUES ('OPS_MAHASISWA', 'kpi.lihat')").Error)

	lama := tunggu(t, 5*time.Second, "perubahan matriks lewat SQL tidak pernah berlaku", func() bool {
		return s.kpi(t, x.access) == http.StatusUnauthorized
	})
	assert.LessOrEqual(t, lama, ttl+2*time.Second, "batas keterlambatan = TTL cache")

	x = s.segarkan(t, x)
	assert.Equal(t, http.StatusOK, s.kpi(t, x.access))

	require.NoError(t, s.db.Exec("DELETE FROM role_permissions WHERE role_kunci = 'OPS_MAHASISWA' AND permission_nama = 'kpi.lihat'").Error)
	tunggu(t, 5*time.Second, "pencabutan lewat SQL tidak pernah berlaku", func() bool {
		return s.kpi(t, x.access) == http.StatusUnauthorized
	})
}

func TestIzin_AkunDinonaktifkanLewatSQLDitolakMeskiTokenMasihSah(t *testing.T) {
	s := suiteDenganIzinTTL(t, 200*time.Millisecond)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	require.Equal(t, http.StatusOK, s.status(t, x.access))

	require.NoError(t, s.db.Exec("UPDATE pengguna SET status = 'NONAKTIF' WHERE id = ?", userBiasa).Error)

	tunggu(t, 5*time.Second, "akun nonaktif tetap diterima", func() bool {
		return s.status(t, x.access) == http.StatusUnauthorized
	})
}

func TestIzin_PenyimpananIzinTidakTersediaGagalTutup503(t *testing.T) {
	s := suiteDenganIzinTTL(t, 100*time.Millisecond)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	require.Equal(t, http.StatusOK, s.status(t, x.access))

	require.NoError(t, s.db.Exec("ALTER TABLE role_permissions RENAME TO role_permissions_rusak").Error)
	t.Cleanup(func() { _ = s.db.Exec("ALTER TABLE IF EXISTS role_permissions_rusak RENAME TO role_permissions").Error })

	tunggu(t, 5*time.Second, "izin yang tidak dapat dipastikan harus 503", func() bool {
		return s.status(t, x.access) == http.StatusServiceUnavailable
	})
	assert.Equal(t, http.StatusServiceUnavailable, s.kpi(t, x.access))

	require.NoError(t, s.db.Exec("ALTER TABLE role_permissions_rusak RENAME TO role_permissions").Error)
	tunggu(t, 5*time.Second, "pulih setelah penyimpanan kembali", func() bool {
		return s.status(t, x.access) == http.StatusOK
	})
}

func TestIzin_EdisiMatriksOptimistisDuaAdminTidakSalingMenimpa(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.ubahMatriks(t, admin.access, "MANAJER_MUTU", 1, []string{"kpi.lihat"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Admin kedua masih memegang versi 1.
	w = s.ubahMatriks(t, admin.access, "MANAJER_MUTU", 1, []string{"harga.lihat"})
	require.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "ROLE_VERSION_CONFLICT", s.kodeDari(t, w))
	saat := s.dataObj(t, w)
	assert.EqualValues(t, 2, saat["versi"], "jawaban memuat keadaan terkini untuk digabung klien")
	assert.Equal(t, []string{"kpi.lihat"}, daftarString(t, saat["permissions"]))

	var izin []string
	require.NoError(t, s.db.Raw("SELECT permission_nama FROM role_permissions WHERE role_kunci = 'MANAJER_MUTU'").Scan(&izin).Error)
	assert.Equal(t, []string{"kpi.lihat"}, izin, "perubahan yang kalah tidak menimpa")
}

func TestIzin_EdisiMatriksSerentakSatuPemenang(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	const n = 12
	var menang, kalah atomic.Int32
	var wg sync.WaitGroup
	mulai := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			badan, _ := json.Marshal(map[string]any{"permissions": []string{usecase.KatalogIzin()[i].Nama}, "versi": 1})
			req := httptest.NewRequest(http.MethodPut, "/v1/roles/MANAJER_MUTU/permissions", bytes.NewReader(badan))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+admin.access)
			<-mulai
			w := httptest.NewRecorder()
			s.router.ServeHTTP(w, req)
			switch w.Code {
			case http.StatusOK:
				menang.Add(1)
			case http.StatusConflict:
				kalah.Add(1)
			}
		}(i)
	}
	close(mulai)
	wg.Wait()

	assert.EqualValues(t, 1, menang.Load(), "tepat satu pemenang")
	assert.EqualValues(t, n-1, kalah.Load())
	var versi int64
	require.NoError(t, s.db.Raw("SELECT versi FROM roles WHERE kunci = 'MANAJER_MUTU'").Scan(&versi).Error)
	assert.EqualValues(t, 2, versi)
}

func TestIzin_EdisiMatriksTanpaPerubahanTidakMenaikkanVersiAtauAudit(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.ubahMatriks(t, admin.access, "MANAJER_MUTU", 1, usecase.IzinBawaan("MANAJER_MUTU"))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 1, s.dataObj(t, w)["versi"])
	assert.EqualValues(t, 0, s.auditCount(t, "UPDATE_ROLE_PERMISSIONS", userAdminID, "MANAJER_MUTU"))
	assert.Equal(t, http.StatusOK, s.status(t, admin.access))
}

func TestIzin_EdisiMatriksMasukanTidakValid(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.ubahMatriks(t, admin.access, "MANAJER_MUTU", 1, []string{"kpi.lihat", "izin.palsu"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "UNKNOWN_PERMISSION", s.kodeDari(t, w))

	w = s.ubahMatriks(t, admin.access, "SUPERADMIN", 1, []string{"kpi.lihat"})
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = s.panggil(t, http.MethodPut, "/v1/roles/MANAJER_MUTU/permissions", map[string]any{"versi": 1}, admin.access)
	assert.Equal(t, http.StatusBadRequest, w.Code, "daftar izin wajib (kosong eksplisit boleh, hilang tidak)")

	w = s.panggil(t, http.MethodPut, "/v1/roles/MANAJER_MUTU/permissions", map[string]any{"permissions": []string{}}, admin.access)
	assert.Equal(t, http.StatusBadRequest, w.Code, "versi wajib")

	// Daftar kosong yang eksplisit adalah sah: peran tanpa izin.
	w = s.ubahMatriks(t, admin.access, "MANAJER_MUTU", 1, []string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, daftarString(t, s.dataObj(t, w)["permissions"]))

	var izin []string
	require.NoError(t, s.db.Raw("SELECT permission_nama FROM role_permissions WHERE role_kunci = 'MANAJER_PROGRAM'").Scan(&izin).Error)
	assert.ElementsMatch(t, usecase.IzinBawaan("MANAJER_PROGRAM"), izin, "peran lain tidak tersentuh")
}

// ── Izin langsung ─────────────────────────────────────────────────────

func TestIzin_IzinLangsungBerlakuLaluDicabut(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	require.Equal(t, http.StatusForbidden, s.kpi(t, x.access))

	w := s.beriIzin(t, admin.access, userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "Proyek KPI triwulan"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	g := s.dataObj(t, w)
	assert.Equal(t, "kpi.lihat", g["izin"])
	assert.Equal(t, "Proyek KPI triwulan", g["alasan"])
	assert.Equal(t, userAdminID, g["diberikan_oleh"], "pemberi diambil dari token, bukan dari badan permintaan")
	assert.Nil(t, g["berlaku_sampai"])
	assert.Equal(t, true, g["aktif"])

	assert.Equal(t, http.StatusUnauthorized, s.kpi(t, x.access), "token basi diminta menyegarkan diri")
	x = s.segarkan(t, x)
	assert.Contains(t, s.izinToken(t, x.access), "kpi.lihat")
	assert.Equal(t, http.StatusOK, s.kpi(t, x.access))

	w = s.panggil(t, http.MethodGet, "/v1/users/"+userBiasa+"/permissions", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	ringkas := s.dataObj(t, w)
	assert.Equal(t, "OPS_MAHASISWA", ringkas["peran"])
	assert.ElementsMatch(t, usecase.IzinBawaan("OPS_MAHASISWA"), daftarString(t, ringkas["izin_peran"]))
	assert.Contains(t, daftarString(t, ringkas["efektif"]), "kpi.lihat")
	langsung := ringkas["langsung"].([]any)
	require.Len(t, langsung, 1)
	assert.NotEmpty(t, langsung[0].(map[string]any)["diberikan_oleh_nama"])

	w = s.panggil(t, http.MethodDelete, "/v1/users/"+userBiasa+"/permissions/kpi.lihat", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, http.StatusUnauthorized, s.kpi(t, x.access), "pencabutan berlaku seketika")
	x = s.segarkan(t, x)
	assert.Equal(t, http.StatusForbidden, s.kpi(t, x.access))
	assert.NotContains(t, s.izinToken(t, x.access), "kpi.lihat")

	w = s.panggil(t, http.MethodDelete, "/v1/users/"+userBiasa+"/permissions/kpi.lihat", nil, admin.access)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "DIRECT_PERMISSION_NOT_FOUND", s.kodeDari(t, w))
}

func TestIzin_IzinLangsungKedaluwarsaBerhentiBerlakuTepatWaktuWalauCacheLebihLama(t *testing.T) {
	s := setupSuite(t) // TTL bawaan 15 detik: kedaluwarsa tidak boleh menunggunya
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	sampai := time.Now().Add(3 * time.Second).UTC().Format(time.RFC3339Nano)

	w := s.beriIzin(t, admin.access, userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "sementara", "berlaku_sampai": sampai})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	x = func() sesiUji { // token lama basi, segarkan untuk mendapat izin sementara
		require.Equal(t, http.StatusUnauthorized, s.kpi(t, x.access))
		return s.segarkan(t, x)
	}()
	require.Contains(t, s.izinToken(t, x.access), "kpi.lihat")
	require.Equal(t, http.StatusOK, s.kpi(t, x.access))

	time.Sleep(3300 * time.Millisecond)

	w = s.panggil(t, http.MethodGet, "/v1/kpi", nil, x.access)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "izin sudah lewat: tidak lagi 200 walau cache 15 detik belum habis")
	assert.Equal(t, middleware.PermissionsChangedCode, s.kodeDari(t, w))
	x = s.segarkan(t, x)
	assert.NotContains(t, s.izinToken(t, x.access), "kpi.lihat")
	assert.Equal(t, http.StatusForbidden, s.kpi(t, x.access))

	// Baris kedaluwarsa tetap sebagai riwayat dan terbaca sebagai tidak aktif.
	w = s.panggil(t, http.MethodGet, "/v1/users/"+userBiasa+"/permissions", nil, admin.access)
	langsung := s.dataObj(t, w)["langsung"].([]any)
	require.Len(t, langsung, 1)
	assert.Equal(t, false, langsung[0].(map[string]any)["aktif"])

	// ...dan boleh diberikan lagi (menggantikan baris kedaluwarsa).
	w = s.beriIzin(t, admin.access, userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "diperpanjang"})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestIzin_IzinLangsungDikedaluwarsakanLewatSQLBerlakuSetelahTTL(t *testing.T) {
	const ttl = 300 * time.Millisecond
	s := suiteDenganIzinTTL(t, ttl)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	require.Equal(t, http.StatusCreated, s.beriIzin(t, admin.access, userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "uji"}).Code)
	x = s.segarkan(t, x)
	require.Equal(t, http.StatusOK, s.kpi(t, x.access))

	require.NoError(t, s.db.Exec("UPDATE user_permissions SET berlaku_sampai = now() - interval '1 minute' WHERE pengguna_id = ?", userBiasa).Error)

	tunggu(t, 5*time.Second, "izin langsung yang dikedaluwarsakan lewat SQL tetap berlaku", func() bool {
		return s.kpi(t, x.access) == http.StatusUnauthorized
	})
}

func TestIzin_IzinLangsungMasukanTidakValid(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	lampau := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)

	cases := []struct {
		nama   string
		user   string
		badan  map[string]any
		status int
		kode   string
	}{
		{"izin di luar katalog", userBiasa, map[string]any{"izin": "izin.palsu", "alasan": "x"}, http.StatusBadRequest, "UNKNOWN_PERMISSION"},
		{"alasan kosong", userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": ""}, http.StatusBadRequest, ""},
		{"alasan hanya spasi", userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "   "}, http.StatusBadRequest, ""},
		{"alasan tidak ada", userBiasa, map[string]any{"izin": "kpi.lihat"}, http.StatusBadRequest, ""},
		{"berlaku sampai di masa lalu", userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "x", "berlaku_sampai": lampau}, http.StatusBadRequest, ""},
		{"berlaku sampai bukan tanggal", userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "x", "berlaku_sampai": "besok"}, http.StatusBadRequest, ""},
		{"pengguna tidak ada", "hantu", map[string]any{"izin": "kpi.lihat", "alasan": "x"}, http.StatusNotFound, ""},
		{"sudah dimiliki lewat peran", userBiasa, map[string]any{"izin": "harga.input", "alasan": "x"}, http.StatusConflict, "PERMISSION_ALREADY_VIA_ROLE"},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			w := s.beriIzin(t, admin.access, tc.user, tc.badan)
			assert.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.kode != "" {
				assert.Equal(t, tc.kode, s.kodeDari(t, w))
			}
		})
	}
	assert.EqualValues(t, 0, s.count(t, "SELECT count(*) FROM user_permissions"), "pemberian yang ditolak tidak meninggalkan baris")
	assert.EqualValues(t, 0, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'GRANT_USER_PERMISSION'"))

	// Izin langsung yang masih berlaku tidak ditimpa diam-diam.
	require.Equal(t, http.StatusCreated, s.beriIzin(t, admin.access, userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "pertama"}).Code)
	w := s.beriIzin(t, admin.access, userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "kedua"})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "DIRECT_PERMISSION_EXISTS", s.kodeDari(t, w))
	var alasan string
	require.NoError(t, s.db.Raw("SELECT alasan FROM user_permissions WHERE pengguna_id = ?", userBiasa).Scan(&alasan).Error)
	assert.Equal(t, "pertama", alasan)
}

func TestIzin_IzinLangsungDihapusBersamaPenggunanya(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	require.NoError(t, s.db.Exec(`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES ('sementara', 'S', 's@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'L')`).Error)
	require.Equal(t, http.StatusCreated, s.beriIzin(t, admin.access, "sementara", map[string]any{"izin": "kpi.lihat", "alasan": "uji"}).Code)

	w := s.panggil(t, http.MethodDelete, "/v1/users/sementara", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.EqualValues(t, 0, s.count(t, "SELECT count(*) FROM user_permissions WHERE pengguna_id = 'sementara'"))
}

// ── Audit ─────────────────────────────────────────────────────────────

func TestIzin_PerubahanTercatatDiAuditDenganSebelumDanSesudah(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	require.Equal(t, http.StatusOK, s.ubahMatriks(t, admin.access, "MANAJER_MUTU", 1, []string{"kpi.lihat", "harga.lihat"}).Code)
	require.Equal(t, http.StatusCreated, s.beriIzin(t, admin.access, userBiasa, map[string]any{"izin": "kpi.lihat", "alasan": "audit uji"}).Code)
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodDelete, "/v1/users/"+userBiasa+"/permissions/kpi.lihat", nil, admin.access).Code)

	baris := func(aksi, entitasID string) (sebelum, sesudah, metadata map[string]any) {
		var row struct{ Sebelum, Sesudah, Metadata *string }
		require.NoError(t, s.db.Raw("SELECT sebelum::text, sesudah::text, metadata::text FROM audit_log WHERE aksi = ? AND entitas_id = ? AND aktor_id = ?",
			aksi, entitasID, userAdminID).Scan(&row).Error)
		ur := func(p *string) map[string]any {
			if p == nil {
				return nil
			}
			var m map[string]any
			require.NoError(t, json.Unmarshal([]byte(*p), &m))
			return m
		}
		return ur(row.Sebelum), ur(row.Sesudah), ur(row.Metadata)
	}

	sebelum, sesudah, meta := baris("UPDATE_ROLE_PERMISSIONS", "MANAJER_MUTU")
	assert.ElementsMatch(t, usecase.IzinBawaan("MANAJER_MUTU"), daftarString(t, sebelum["izin"]))
	assert.ElementsMatch(t, []string{"kpi.lihat", "harga.lihat"}, daftarString(t, sesudah["izin"]))
	assert.EqualValues(t, 1, sebelum["versi"])
	assert.EqualValues(t, 2, sesudah["versi"])
	assert.NotEmpty(t, daftarString(t, meta["dicabut"]))

	sebelum, sesudah, meta = baris("GRANT_USER_PERMISSION", userBiasa)
	assert.Nil(t, sebelum)
	assert.Equal(t, "audit uji", sesudah["alasan"])
	assert.Equal(t, userAdminID, sesudah["diberikan_oleh"])
	assert.Equal(t, "kpi.lihat", meta["izin"])

	sebelum, sesudah, _ = baris("REVOKE_USER_PERMISSION", userBiasa)
	assert.Equal(t, "kpi.lihat", sebelum["izin"])
	assert.Nil(t, sesudah)
}

// ── Otorisasi ─────────────────────────────────────────────────────────

func TestIzin_SemuaEndpointButuhPenggunaKelola(t *testing.T) {
	s := setupSuite(t)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	badanPeran := map[string]any{"permissions": []string{"kpi.lihat"}, "versi": 1}
	badanBeri := map[string]any{"izin": "kpi.lihat", "alasan": "x"}

	endpoint := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/v1/roles", nil},
		{http.MethodGet, "/v1/permissions", nil},
		{http.MethodPut, "/v1/roles/OPS_MAHASISWA/permissions", badanPeran},
		{http.MethodGet, "/v1/users/" + userBiasa + "/permissions", nil},
		{http.MethodPost, "/v1/users/" + userBiasa + "/permissions", badanBeri},
		{http.MethodDelete, "/v1/users/" + userBiasa + "/permissions/kpi.lihat", nil},
	}
	for _, e := range endpoint {
		assert.Equal(t, http.StatusForbidden, s.panggil(t, e.method, e.path, e.body, x.access).Code, "%s %s tanpa pengguna.kelola", e.method, e.path)
		assert.Equal(t, http.StatusUnauthorized, s.panggil(t, e.method, e.path, e.body, "").Code, "%s %s tanpa token", e.method, e.path)
	}

	// Menaikkan diri sendiri tidak mungkin: tidak ada efek samping.
	assert.EqualValues(t, 0, s.count(t, "SELECT count(*) FROM user_permissions"))
	assert.EqualValues(t, 1, s.count(t, "SELECT versi FROM roles WHERE kunci = 'OPS_MAHASISWA'"))
}

func TestIzin_IzinLangsungPenggunaKelolaMembukaEndpointAdmin(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	x := s.mulaiSesi(t, emailBiasa, uaIPhone)
	require.Equal(t, http.StatusCreated, s.beriIzin(t, admin.access, userBiasa, map[string]any{"izin": "pengguna.kelola", "alasan": "delegasi sementara"}).Code)

	x = s.segarkan(t, x)

	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/roles", nil, x.access).Code, "izin langsung pengguna.kelola berfungsi")
	assert.EqualValues(t, 1, s.pemegangKelola(t), "tetapi tidak dihitung sebagai pemegang tetap")
}

// ── Penjaga pemegang terakhir pengguna.kelola ─────────────────────────

func TestIzin_PemegangTerakhirTidakBisaDihilangkanLewatMatriksMaupunPerubahanAkun(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	tanpaKelola := slices.DeleteFunc(usecase.IzinBawaan("ADMIN_SISTEM"), func(p string) bool { return p == "pengguna.kelola" })

	w := s.ubahMatriks(t, admin.access, "ADMIN_SISTEM", 1, tanpaKelola)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "LAST_PERMISSION_HOLDER", s.kodeDari(t, w))

	w = s.panggil(t, http.MethodPut, "/v1/users/"+userAdminID+"/roles", map[string]any{"peran": "OPS_MAHASISWA"}, admin.access)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "LAST_PERMISSION_HOLDER", s.kodeDari(t, w))

	w = s.panggil(t, http.MethodPatch, "/v1/users/"+userAdminID, map[string]any{"peran": "OPS_MAHASISWA"}, admin.access)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	w = s.panggil(t, http.MethodPatch, "/v1/users/"+userAdminID, map[string]any{"status": "NONAKTIF"}, admin.access)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	w = s.panggil(t, http.MethodPost, "/v1/users/bulk", map[string]any{"action": "nonaktifkan", "ids": []string{userAdminID, userBiasa}}, admin.access)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "LAST_PERMISSION_HOLDER", s.kodeDari(t, w))

	assert.EqualValues(t, 1, s.pemegangKelola(t))
	var peran, status string
	require.NoError(t, s.db.Raw("SELECT peran::text FROM pengguna WHERE id = ?", userAdminID).Scan(&peran).Error)
	require.NoError(t, s.db.Raw("SELECT status::text FROM pengguna WHERE id = ?", userBiasa).Scan(&status).Error)
	assert.Equal(t, "ADMIN_SISTEM", peran, "tidak ada yang tertulis")
	assert.Equal(t, "AKTIF", status, "aksi massal ditolak seluruhnya, bukan sebagian")
	assert.Equal(t, http.StatusOK, s.status(t, admin.access), "sesi admin tidak tercabut oleh perubahan yang ditolak")
	assert.EqualValues(t, 0, s.auditCount(t, "ASSIGN_ROLE", userAdminID, userAdminID))
}

func TestIzin_PemegangLainMembolehkanPerubahanSampaTinggalSatu(t *testing.T) {
	s := setupSuite(t)
	s.tambahAdminKedua(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	kedua := s.mulaiSesi(t, emailKedua, uaMac)
	require.EqualValues(t, 2, s.pemegangKelola(t))

	// Admin pertama menurunkan admin kedua: masih ada dia sendiri.
	w := s.panggil(t, http.MethodPut, "/v1/users/"+adminKedua+"/roles", map[string]any{"peran": "OPS_MAHASISWA"}, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, http.StatusUnauthorized, s.status(t, kedua.access), "sesi admin kedua dicabut karena perannya berganti")

	// Sekarang dia pemegang terakhir.
	w = s.panggil(t, http.MethodPut, "/v1/users/"+userAdminID+"/roles", map[string]any{"peran": "OPS_MAHASISWA"}, admin.access)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

func TestIzin_MatriksBolehMencabutKelolaDariPeranBilaPeranLainMemegangnya(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	repo := postgres.New(s.db)
	// test-user-mfa (OPS_KOORDINATOR, aktif) menjadi pemegang lewat peran lain.
	require.NoError(t, s.db.Exec("INSERT INTO role_permissions (role_kunci, permission_nama) VALUES ('OPS_KOORDINATOR', 'pengguna.kelola')").Error)
	tanpaKelola := slices.DeleteFunc(usecase.IzinBawaan("ADMIN_SISTEM"), func(p string) bool { return p == "pengguna.kelola" })

	w := s.ubahMatriks(t, admin.access, "ADMIN_SISTEM", 1, tanpaKelola)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Kini hanya OPS_KOORDINATOR yang memegang: mencabutnya dari peran itu ditolak.
	_, err := repo.SetIzinPeran(context.Background(), domain.SetIzinPeran{
		Peran: "OPS_KOORDINATOR", Versi: 1, Izin: slices.DeleteFunc(append(usecase.IzinBawaan("OPS_KOORDINATOR"), "pengguna.kelola"), func(p string) bool { return p == "pengguna.kelola" }),
	})
	assert.ErrorIs(t, err, domain.ErrPemegangTerakhir)
	assert.EqualValues(t, 1, s.pemegangKelola(t))
}

func TestIzin_PemegangTerakhirKebalBalapan(t *testing.T) {
	s := setupSuite(t)
	s.tambahAdminKedua(t)
	repo := postgres.New(s.db)
	demosi := "OPS_MAHASISWA"

	for putaran := 0; putaran < 8; putaran++ {
		require.NoError(t, s.db.Exec("UPDATE pengguna SET peran = 'ADMIN_SISTEM', lapis = 'SISTEM', status = 'AKTIF' WHERE id IN (?, ?)", userAdminID, adminKedua).Error)
		var wg sync.WaitGroup
		mulai := make(chan struct{})
		hasil := make([]error, 2)
		for i, id := range []string{userAdminID, adminKedua} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-mulai
				_, _, hasil[i] = repo.PatchUser(context.Background(), id, domain.PenggunaPatch{Peran: &demosi})
			}()
		}
		close(mulai)
		wg.Wait()

		sukses, ditolak := 0, 0
		for _, err := range hasil {
			switch {
			case err == nil:
				sukses++
			case assert.ErrorIs(t, err, domain.ErrPemegangTerakhir):
				ditolak++
			}
		}
		assert.Equal(t, 1, sukses, "putaran %d: tepat satu penurunan lolos", putaran)
		assert.Equal(t, 1, ditolak, "putaran %d", putaran)
		assert.EqualValues(t, 1, s.pemegangKelola(t), "putaran %d: sistem tidak pernah kehilangan pemegang terakhir", putaran)
	}
}

func TestIzin_RepositoriMenjagaPenghapusanDanPenonaktifan(t *testing.T) {
	s := setupSuite(t)
	repo := postgres.New(s.db)
	ctx := context.Background()
	require.NoError(t, s.db.Exec(`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga) VALUES ('bukan-admin', 'B', 'b@x.id', 'OPS_MAHASISWA', 'OPERASIONAL', 'L')`).Error)

	assert.ErrorIs(t, repo.DeleteUser(ctx, userAdminID), domain.ErrPemegangTerakhir)
	_, err := repo.BulkUpdateUsers(ctx, []string{userAdminID}, domain.BulkNonaktifkan)
	assert.ErrorIs(t, err, domain.ErrPemegangTerakhir)
	require.NoError(t, repo.DeleteUser(ctx, "bukan-admin"), "pengguna biasa tetap bisa dihapus")
	n, err := repo.BulkUpdateUsers(ctx, []string{userBiasa}, domain.BulkNonaktifkan)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)

	s.tambahAdminKedua(t)
	require.NoError(t, repo.DeleteUser(ctx, adminKedua), "ada pemegang lain")
	assert.EqualValues(t, 1, s.pemegangKelola(t))
}

func TestIzin_BasisDataTanpaAdminTidakMenghalangiPerubahanLain(t *testing.T) {
	s := setupSuite(t)
	repo := postgres.New(s.db)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET peran = 'OPS_MAHASISWA' WHERE peran = 'ADMIN_SISTEM'").Error)
	require.EqualValues(t, 0, s.pemegangKelola(t))
	status := "NONAKTIF"

	_, _, err := repo.PatchUser(context.Background(), userBiasa, domain.PenggunaPatch{Status: &status})

	assert.NoError(t, err, "tidak ada pemegang yang dilindungi, perubahan lain tidak dihalangi")
}

// ── Izin efektif = peran ∪ izin langsung (lapisan repositori) ─────────

func TestIzin_IzinEfektifGabunganPeranDanLangsungTanpaDuplikat(t *testing.T) {
	s := setupSuite(t)
	repo := postgres.New(s.db)
	ctx := context.Background()

	awal, err := repo.IzinEfektifPengguna(ctx, userBiasa)
	require.NoError(t, err)
	assert.Equal(t, "OPS_MAHASISWA", awal.Peran)
	assert.ElementsMatch(t, usecase.IzinBawaan("OPS_MAHASISWA"), awal.Izin)
	assert.Zero(t, awal.KedaluwarsaDalam)

	// Izin langsung yang kemudian juga dimiliki peran tidak menggandakan hasil.
	require.NoError(t, s.db.Exec(`INSERT INTO user_permissions (pengguna_id, permission_nama, alasan, berlaku_sampai)
		VALUES (?, 'kpi.lihat', 'a', now() + interval '2 hours'), (?, 'harga.input', 'b', NULL)`, userBiasa, userBiasa).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO user_permissions (pengguna_id, permission_nama, alasan, berlaku_sampai)
		VALUES (?, 'audit.lihat', 'kedaluwarsa', now() - interval '1 second')`, userBiasa).Error)

	hasil, err := repo.IzinEfektifPengguna(ctx, userBiasa)
	require.NoError(t, err)
	assert.ElementsMatch(t, append(usecase.IzinBawaan("OPS_MAHASISWA"), "kpi.lihat"), hasil.Izin, "gabungan, tanpa duplikat, tanpa yang kedaluwarsa")
	assert.True(t, slices.IsSorted(hasil.Izin))
	assert.InDelta(t, (2 * time.Hour).Seconds(), hasil.KedaluwarsaDalam.Seconds(), 5, "sisa waktu dari jam basis data")

	_, err = repo.IzinEfektifPengguna(ctx, "hantu")
	assert.Error(t, err)
	_, err = repo.IzinEfektifPengguna(ctx, "test-user-002") // NONAKTIF
	assert.Error(t, err, "akun nonaktif tidak punya izin efektif")
}
