package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/internal/audit"
	auditdomain "github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/token"
)

// 21-A: impersonasi peran dan akun (docs/impersonasi.md). Akun uji dari seedData: admin (ADMIN_SISTEM,
// test@gardainflasi.id), biasa (OPS_MAHASISWA AKTIF), nonaktif (OPS_MAHASISWA NONAKTIF), dan mfa (OPS_KOORDINATOR).

const (
	emailQA   = "qa@gardainflasi.id"
	emailOps2 = "ops.dua@gardainflasi.id"
)

var (
	userBiasaID = uid("test-user-biasa")
	userMFAID   = uid("test-user-mfa")
	userQAID    = uid("qa-mutu")
	userOps2ID  = uid("ops-dua")
)

// tambahAkun menambah akun AKTIF berkata sandi sama dengan admin uji (Test1234!).
func (s *Suite) tambahAkun(t *testing.T, id, nama, email, peran, lapis string) {
	t.Helper()
	require.NoError(t, s.db.Exec(`INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status)
		SELECT ?, ?, ?, password_hash, ?::peran, ?::lapis, 'BPPSDMP', 'AKTIF' FROM pengguna WHERE id = ?`,
		id, nama, email, peran, lapis, userAdminID).Error)
}

// suiteImpersonasi menyiapkan suite dengan QA (MANAJER_MUTU) dan satu akun OPS_MAHASISWA kedua. Cache validitas
// impersonasi dan izin dimatikan (ttl negatif) supaya perubahan keadaan di DB langsung terlihat.
func suiteImpersonasi(t *testing.T) *Suite {
	t.Helper()
	cfg := routes.ConfigFromEnv(os.Getenv)
	cfg.IzinCacheTTL = -1
	s := newSuite(t, seedData, routes.Options{UseCaseConfig: cfg})
	s.tambahAkun(t, userQAID, "QA Mutu", emailQA, "MANAJER_MUTU", "MANAJERIAL")
	s.tambahAkun(t, userOps2ID, "Ops Dua", emailOps2, "OPS_MAHASISWA", "OPERASIONAL")
	return s
}

func (s *Suite) mulaiImp(t *testing.T, access string, badan map[string]any) *httpHasil {
	t.Helper()
	w := s.panggil(t, http.MethodPost, "/v1/auth/impersonation/start", badan, access)
	return &httpHasil{w: w, s: s, t: t}
}

type httpHasil struct {
	w *httptest.ResponseRecorder
	s *Suite
	t *testing.T
}

func (h *httpHasil) kode() string { return h.s.kodeDari(h.t, h.w) }

func (s *Suite) simpanKebijakan(t *testing.T, access string, badan map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return s.panggil(t, http.MethodPut, "/v1/admin/impersonation/policy", badan, access)
}

func (s *Suite) sesiDariStart(t *testing.T, w *httptest.ResponseRecorder) sesiUji {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.dataObj(t, w)
	access := data["access_token"].(string)
	claims := mustClaims(t, s.tokenMgr, access)
	return sesiUji{access: access, refresh: data["refresh_token"].(string), sid: claims.SessionID}
}

func (s *Suite) auditAksi(t *testing.T, aksi string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	require.NoError(t, s.db.Raw(`SELECT aktor_id::text AS aktor_id, aktor_peran::text AS aktor_peran, entitas_id, hasil,
			metadata::text AS metadata, metadata->'impersonasi'->>'peran_efektif' AS peran_efektif,
			metadata->'impersonasi'->>'sebagai_pengguna_id' AS sebagai
		  FROM audit_log WHERE aksi = ? ORDER BY id`, aksi).Scan(&rows).Error)
	return rows
}

func (s *Suite) verifikasiRantai(t *testing.T) {
	t.Helper()
	v, err := audit.NewIntegritas(s.db).Verifikasi(t.Context(), "MANUAL")
	require.NoError(t, err)
	assert.Equal(t, auditdomain.StatusValid, v.Status, "rantai hash audit harus utuh setelah impersonasi")
	assert.Zero(t, v.JumlahMasalah)
}

// ── Alur utama: superadmin mulai, bekerja sebagai sasaran, berhenti ───

func TestImpersonasi_SuperadminMulaiBekerjaDanBerhenti(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	// Pemilih sasaran: peran yang punya akun AKTIF, tanpa ADMIN_SISTEM, akun nonaktif, dan diri sendiri.
	w := s.panggil(t, http.MethodGet, "/v1/auth/impersonation/targets", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.dataObj(t, w)
	assert.Equal(t, true, data["boleh"])
	assert.Equal(t, true, data["aktif"])
	assert.Equal(t, true, data["wajib_alasan"])
	assert.EqualValues(t, 60, data["durasi_maks_menit"])
	peranTampil := map[string][]any{}
	for _, p := range data["peran"].([]any) {
		pm := p.(map[string]any)
		peranTampil[pm["peran"].(string)] = pm["akun"].([]any)
	}
	assert.NotContains(t, peranTampil, "ADMIN_SISTEM")
	require.Contains(t, peranTampil, "OPS_MAHASISWA")
	assert.Len(t, peranTampil["OPS_MAHASISWA"], 2, "uji.biasa dan ops.dua; akun nonaktif tidak termasuk")
	require.Contains(t, peranTampil, "OPS_KOORDINATOR")

	// Alasan wajib secara bawaan.
	h := s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID})
	assert.Equal(t, http.StatusBadRequest, h.w.Code)
	assert.Equal(t, "ALASAN_WAJIB", h.kode())

	// ADMIN_SISTEM tidak pernah boleh jadi sasaran, dan tidak ada impersonasi ke diri sendiri.
	h = s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userAdminID, "alasan": "uji"})
	assert.Equal(t, http.StatusForbidden, h.w.Code)
	assert.Equal(t, "IMPERSONASI_DITOLAK", h.kode())
	h = s.mulaiImp(t, admin.access, map[string]any{"target_peran": "ADMIN_SISTEM", "alasan": "uji"})
	assert.Equal(t, http.StatusForbidden, h.w.Code)
	// Dua akun OPS_MAHASISWA aktif: peran saja ambigu.
	h = s.mulaiImp(t, admin.access, map[string]any{"target_peran": "OPS_MAHASISWA", "alasan": "uji"})
	assert.Equal(t, http.StatusConflict, h.w.Code)
	assert.Equal(t, "TARGET_AMBIGU", h.kode())
	// Peran dengan tepat satu akun diselesaikan server.
	h = s.mulaiImp(t, admin.access, map[string]any{"target_peran": "OPS_KOORDINATOR", "alasan": "periksa tampilan koordinator"})
	require.Equal(t, http.StatusOK, h.w.Code, h.w.Body.String())
	imp := s.sesiDariStart(t, h.w)
	mulai := s.dataObj(t, h.w)
	assert.Equal(t, userMFAID, mulai["user"].(map[string]any)["id"])
	meta := mulai["impersonasi"].(map[string]any)
	assert.Equal(t, true, meta["aktif"])
	assert.Equal(t, userAdminID, meta["pelaku"].(map[string]any)["id"])
	assert.Equal(t, "ADMIN_SISTEM", meta["pelaku"].(map[string]any)["peran"])
	assert.Equal(t, "OPS_KOORDINATOR", meta["sasaran"].(map[string]any)["peran"])

	// Token milik SASARAN dengan klaim imp yang menunjuk pelaku asli.
	claims := mustClaims(t, s.tokenMgr, imp.access)
	assert.Equal(t, userMFAID, claims.UserID)
	assert.Equal(t, "OPS_KOORDINATOR", claims.Role)
	require.NotNil(t, claims.Imp)
	assert.Equal(t, userAdminID, claims.Imp.By.UserID)
	assert.Equal(t, "ADMIN_SISTEM", claims.Imp.By.Role)
	assert.NotContains(t, claims.Permissions, "impersonasi.kelola")
	assert.NotContains(t, claims.Permissions, "pengguna.kelola")
	assert.False(t, claims.MFASetupRequired || claims.PasswordChangeRequired)

	// Sesi asal pelaku dicabut; token lamanya mati.
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, admin.access).Code)

	// GET /auth/me: profil sasaran + metadata impersonasi.
	w = s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	me := s.dataObj(t, w)
	assert.Equal(t, userMFAID, me["id"])
	assert.Equal(t, "OPS_KOORDINATOR", me["peran"])
	require.Contains(t, me, "impersonasi")
	assert.Equal(t, userAdminID, me["impersonasi"].(map[string]any)["pelaku"].(map[string]any)["id"])

	// Hak efektif = hak sasaran: tidak bisa membuka pengaturan impersonasi maupun mengelola pengguna.
	for _, jalur := range []string{"/v1/admin/impersonation/policy", "/v1/admin/impersonation/riwayat", "/v1/users"} {
		assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, jalur, nil, imp.access).Code, jalur)
	}
	// Rute kredensial dan hak dilarang selama impersonasi (kode khusus), bukan sekadar tanpa izin.
	for _, c := range []struct{ metode, jalur string }{
		{http.MethodPost, "/v1/auth/change-password"},
		{http.MethodPatch, "/v1/auth/me"},
		{http.MethodPost, "/v1/auth/mfa/setup"},
		{http.MethodPost, "/v1/auth/mfa/disable"},
		{http.MethodPost, "/v1/auth/logout-all"},
		{http.MethodPost, "/v1/auth/sessions/revoke-others"},
		{http.MethodPost, "/v1/users"},
		{http.MethodPut, "/v1/roles/OPS_KOORDINATOR/permissions"},
		{http.MethodPut, "/v1/admin/impersonation/policy"},
	} {
		w := s.panggil(t, c.metode, c.jalur, map[string]any{}, imp.access)
		assert.Equal(t, http.StatusForbidden, w.Code, "%s %s", c.metode, c.jalur)
		assert.Equal(t, "IMPERSONATION_RESTRICTED", s.kodeDari(t, w), "%s %s", c.metode, c.jalur)
	}
}

// Pelaku ADMIN_SISTEM memegang seluruh katalog, tetapi sesi impersonasi hanya membawa izin akun sasaran.
func TestImpersonasi_TidakMembawaSeluruhKatalogPelaku(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	require.ElementsMatch(t, usecase.IzinBawaan("ADMIN_SISTEM"), mustClaims(t, s.tokenMgr, admin.access).Permissions)

	imp := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji hak"}).w)

	assert.ElementsMatch(t, usecase.IzinBawaan("OPS_MAHASISWA"), mustClaims(t, s.tokenMgr, imp.access).Permissions)
	for _, jalur := range []string{"/v1/users", "/v1/roles", "/v1/admin/impersonation/policy"} {
		assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, jalur, nil, imp.access).Code, jalur)
	}
}

// Sasaran tidak melihat sesi impersonasi di daftar sesinya sendiri (diperiksa lewat akun OPS_MAHASISWA: akun ber-MFA tidak
// bisa login biasa tanpa OTP).
func TestImpersonasi_SesiTidakTampilDiDaftarSesiSasaran(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	imp := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji daftar sesi"}).w)

	sasaran := s.mulaiSesi(t, emailBiasa, uaMac)
	daftar := s.daftarSesi(t, sasaran.access)
	require.Len(t, daftar, 1, "hanya sesi perangkat milik sasaran sendiri")
	assert.Equal(t, sasaran.sid, daftar[0]["id"])

	// Jalur riwayat sesi sasaran juga tidak memuat sesi impersonasi.
	w := s.panggil(t, http.MethodGet, "/v1/auth/me/riwayat-sesi", nil, sasaran.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Len(t, s.dataObj(t, w)["items"], 1)

	// Sesi impersonasi tetap hidup walau sasaran melihat daftarnya.
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)
}

func TestImpersonasi_BerhentiMengembalikanPelakuAsliDariCatatanServer(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	imp := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "periksa menu mahasiswa"}).w)

	// Sesi biasa (bukan impersonasi) tidak bisa memanggil berhenti.
	lain := s.mulaiSesi(t, emailQA, uaMac)
	w := s.panggil(t, http.MethodPost, "/v1/auth/impersonation/stop", nil, lain.access)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "TIDAK_ADA_IMPERSONASI", s.kodeDari(t, w))

	// Berhenti terjangkau dari peran sasaran yang tidak punya izin apa pun (OPS_MAHASISWA) dan tanpa body/header pelaku.
	w = s.panggil(t, http.MethodPost, "/v1/auth/impersonation/stop", nil, imp.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.dataObj(t, w)
	assert.Equal(t, userAdminID, data["user"].(map[string]any)["id"])
	kembali := sesiUji{access: data["access_token"].(string), refresh: data["refresh_token"].(string)}
	claims := mustClaims(t, s.tokenMgr, kembali.access)
	assert.Equal(t, userAdminID, claims.UserID)
	assert.Equal(t, "ADMIN_SISTEM", claims.Role)
	assert.Nil(t, claims.Imp)
	assert.Contains(t, claims.Permissions, "impersonasi.kelola")

	// Sesi impersonasi mati seketika (access dan refresh), sesi pelaku yang baru hidup.
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, imp.refresh).Code)
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/admin/impersonation/policy", nil, kembali.access).Code)
	// Memanggil berhenti lagi dengan token impersonasi yang sama ditolak.
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodPost, "/v1/auth/impersonation/stop", nil, imp.access).Code)

	var rec struct {
		AlasanBerakhir *string
		BerakhirPada   *time.Time
	}
	require.NoError(t, s.db.Raw("SELECT alasan_berakhir, berakhir_pada FROM impersonasi_sesi").Scan(&rec).Error)
	require.NotNil(t, rec.AlasanBerakhir)
	assert.Equal(t, "STOP", *rec.AlasanBerakhir)

	// Audit: mulai dan berhenti tercatat atas nama pelaku asli; berhenti membawa metadata impersonasi.
	mulai := s.auditAksi(t, "IMPERSONASI_MULAI")
	require.Len(t, mulai, 1)
	assert.Equal(t, userAdminID, mulai[0]["aktor_id"])
	assert.Equal(t, userBiasaID, mulai[0]["entitas_id"])
	berhenti := s.auditAksi(t, "IMPERSONASI_BERHENTI")
	require.Len(t, berhenti, 1)
	assert.Equal(t, userAdminID, berhenti[0]["aktor_id"])
	assert.Equal(t, "OPS_MAHASISWA", berhenti[0]["peran_efektif"])
	s.verifikasiRantai(t)
}

func TestImpersonasi_MutasiSelamaImpersonasiDicatatAtasNamaPelakuAsli(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	imp := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji mutasi"}).w)

	// Mutasi yang ditolak karena tanpa izin (OPS_MAHASISWA tidak punya intervensi.kelola) tetap tercatat.
	w := s.panggil(t, http.MethodPost, "/v1/intervensi", map[string]any{}, imp.access)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	aksi := s.auditAksi(t, "IMPERSONASI_AKSI")
	require.NotEmpty(t, aksi, "setiap mutasi selama impersonasi tercatat")
	last := aksi[len(aksi)-1]
	assert.Equal(t, userAdminID, last["aktor_id"], "aktor_id adalah akun ASLI")
	assert.Equal(t, "ADMIN_SISTEM", last["aktor_peran"])
	assert.Equal(t, "OPS_MAHASISWA", last["peran_efektif"])
	assert.Equal(t, userBiasaID, last["sebagai"])
	assert.Equal(t, "DITOLAK", last["hasil"])

	// Percobaan rute terlarang (kredensial) juga tercatat sebagai ditolak.
	require.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodPost, "/v1/auth/change-password", map[string]any{}, imp.access).Code)
	aksi = s.auditAksi(t, "IMPERSONASI_AKSI")
	assert.Contains(t, aksi[len(aksi)-1]["metadata"], "/v1/auth/change-password")

	// Pembacaan tidak menambah entri aksi.
	sebelum := len(s.auditAksi(t, "IMPERSONASI_AKSI"))
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)
	assert.Len(t, s.auditAksi(t, "IMPERSONASI_AKSI"), sebelum)

	// Logout dari sesi impersonasi menutup catatannya dan tercatat atas nama pelaku asli.
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, imp.access).Code)
	logout := s.auditAksi(t, "LOGOUT")
	require.NotEmpty(t, logout)
	assert.Equal(t, userAdminID, logout[len(logout)-1]["aktor_id"])
	assert.Equal(t, "OPS_MAHASISWA", logout[len(logout)-1]["peran_efektif"])
	var alasan string
	require.NoError(t, s.db.Raw("SELECT alasan_berakhir FROM impersonasi_sesi").Scan(&alasan).Error)
	assert.Equal(t, "LOGOUT", alasan)
	s.verifikasiRantai(t)
}

func TestImpersonasi_RefreshMempertahankanImpersonasi(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	imp := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji refresh"}).w)

	baru := s.segarkan(t, imp)
	claims := mustClaims(t, s.tokenMgr, baru.access)
	assert.Equal(t, userBiasaID, claims.UserID)
	require.NotNil(t, claims.Imp, "token hasil refresh tetap token impersonasi")
	assert.Equal(t, userAdminID, claims.Imp.By.UserID)
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, baru.access).Code)

	// Setelah berhenti, refresh token impersonasi (yang sudah dirotasi) tidak bisa lagi.
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/impersonation/stop", nil, baru.access).Code)
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, baru.refresh).Code)
}

// ── Kebijakan: eksklusif superadmin, versi optimistis, validasi ───────

func TestImpersonasi_KebijakanHanyaSuperadmin(t *testing.T) {
	s := suiteImpersonasi(t)
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)
	qa := s.mulaiSesi(t, emailQA, uaMac)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	for _, a := range []string{biasa.access, qa.access} {
		assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/admin/impersonation/policy", nil, a).Code)
		assert.Equal(t, http.StatusForbidden, s.simpanKebijakan(t, a, map[string]any{"versi": 0, "aktif": true, "wajib_alasan": true, "pelaku": []any{}}).Code)
		assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/admin/impersonation/riwayat", nil, a).Code)
	}
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/admin/impersonation/policy", nil, "").Code)

	// Hanya superadmin yang membawa izin itu; peran lain tidak walau role_permissions diubah langsung di basis data.
	assert.Contains(t, s.izinToken(t, admin.access), "impersonasi.kelola")
	require.NoError(t, s.db.Exec("INSERT INTO role_permissions (role_kunci, permission_nama) VALUES ('MANAJER_MUTU', 'impersonasi.kelola')").Error)
	qa2 := s.mulaiSesi(t, emailQA, uaMac)
	assert.NotContains(t, s.izinToken(t, qa2.access), "impersonasi.kelola")
	assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/admin/impersonation/policy", nil, qa2.access).Code)

	// Kebijakan bawaan sebelum pernah disimpan.
	w := s.panggil(t, http.MethodGet, "/v1/admin/impersonation/policy", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	k := s.dataObj(t, w)
	assert.EqualValues(t, 0, k["versi"])
	assert.Equal(t, true, k["aktif"])
	assert.Equal(t, true, k["wajib_alasan"])
	assert.EqualValues(t, 60, k["durasi_maks_menit"])
	assert.Empty(t, k["pelaku"])
}

func TestImpersonasi_IzinEksklusifTidakBisaDiberikanLewatMatriksAtauIzinLangsung(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	// Matriks: peran lain tidak boleh memuatnya.
	w := s.panggil(t, http.MethodGet, "/v1/roles", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var versi float64
	var izin []string
	for _, r := range s.parseResponse(t, w)["data"].([]any) {
		rm := r.(map[string]any)
		if rm["id"] == "MANAJER_MUTU" {
			versi = rm["versi"].(float64)
			izin = daftarString(t, rm["permissions"])
		}
	}
	w = s.ubahMatriks(t, admin.access, "MANAJER_MUTU", int(versi), append(izin, "impersonasi.kelola"))
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "IZIN_EKSKLUSIF", s.kodeDari(t, w))

	// Izin langsung: tidak untuk siapa pun.
	w = s.beriIzin(t, admin.access, userQAID, map[string]any{"izin": "impersonasi.kelola", "alasan": "uji"})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "IZIN_EKSKLUSIF", s.kodeDari(t, w))
}

func TestImpersonasi_SimpanKebijakanValidasiDanVersi(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	dasar := func() map[string]any {
		return map[string]any{"versi": 0, "aktif": true, "wajib_alasan": true, "durasi_maks_menit": 30, "pelaku": []any{}}
	}
	tolak := func(badan map[string]any, ket string) {
		t.Helper()
		w := s.simpanKebijakan(t, admin.access, badan)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", ket, w.Body.String())
		assert.Equal(t, "KEBIJAKAN_TIDAK_VALID", s.kodeDari(t, w), ket)
	}
	pelaku := func(tipe, ref string, target ...map[string]any) map[string]any {
		p := map[string]any{"tipe": tipe, "aktif": true, "target": target}
		if tipe == "PERAN" {
			p["peran"] = ref
		} else {
			p["pengguna_id"] = ref
		}
		return p
	}
	peran := func(r string) map[string]any { return map[string]any{"tipe": "PERAN", "peran": r} }
	akun := func(id string) map[string]any { return map[string]any{"tipe": "PENGGUNA", "pengguna_id": id} }

	b := dasar()
	b["durasi_maks_menit"] = 3
	tolak(b, "durasi terlalu pendek")
	b = dasar()
	b["durasi_maks_menit"] = 2000
	tolak(b, "durasi terlalu panjang")
	b = dasar()
	b["pelaku"] = []any{pelaku("PERAN", "ADMIN_SISTEM", peran("OPS_MAHASISWA"))}
	tolak(b, "ADMIN_SISTEM bukan pelaku")
	b = dasar()
	b["pelaku"] = []any{pelaku("PERAN", "MANAJER_MUTU", peran("ADMIN_SISTEM"))}
	tolak(b, "ADMIN_SISTEM bukan sasaran")
	b = dasar()
	b["pelaku"] = []any{pelaku("PERAN", "MANAJER_MUTU", akun(userAdminID))}
	tolak(b, "akun ADMIN_SISTEM bukan sasaran")
	b = dasar()
	b["pelaku"] = []any{pelaku("PERAN", "BUKAN_PERAN", peran("OPS_MAHASISWA"))}
	tolak(b, "peran tak dikenal")
	b = dasar()
	b["pelaku"] = []any{pelaku("PERAN", "MANAJER_MUTU", peran("OPS_MAHASISWA"), peran("OPS_MAHASISWA"))}
	tolak(b, "sasaran duplikat")
	b = dasar()
	b["pelaku"] = []any{pelaku("PERAN", "MANAJER_MUTU"), pelaku("PERAN", "MANAJER_MUTU")}
	tolak(b, "pelaku duplikat")
	b = dasar()
	b["pelaku"] = []any{pelaku("PENGGUNA", "00000000-0000-4000-8000-000000000001", peran("OPS_MAHASISWA"))}
	tolak(b, "akun tidak ada")
	b = dasar()
	b["pelaku"] = []any{pelaku("PENGGUNA", "bukan-uuid")}
	tolak(b, "bukan uuid")

	// Sah: QA (peran) boleh ke OPS_MAHASISWA; akun Ops Dua boleh ke akun biasa.
	b = dasar()
	b["pelaku"] = []any{
		pelaku("PERAN", "MANAJER_MUTU", peran("OPS_MAHASISWA"), peran("OPS_AGREGATOR")),
		pelaku("PENGGUNA", userOps2ID, akun(userBiasaID)),
	}
	w := s.simpanKebijakan(t, admin.access, b)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	k := s.dataObj(t, w)
	assert.EqualValues(t, 1, k["versi"])
	assert.EqualValues(t, 30, k["durasi_maks_menit"])
	require.Len(t, k["pelaku"], 2)
	assert.Equal(t, userAdminID, k["diperbarui_oleh"])

	// Versi usang -> 409 beserta kebijakan terkini.
	w = s.simpanKebijakan(t, admin.access, b)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, "VERSI_KEBIJAKAN_BERUBAH", s.kodeDari(t, w))
	assert.EqualValues(t, 1, s.dataObj(t, w)["versi"])

	// Dibaca ulang lengkap dengan identitas akun.
	w = s.panggil(t, http.MethodGet, "/v1/admin/impersonation/policy", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code)
	baca := s.dataObj(t, w)
	var akunPelaku map[string]any
	for _, p := range baca["pelaku"].([]any) {
		if p.(map[string]any)["tipe"] == "PENGGUNA" {
			akunPelaku = p.(map[string]any)
		}
	}
	require.NotNil(t, akunPelaku)
	assert.Equal(t, "Ops Dua", akunPelaku["pengguna_nama"])
	assert.Equal(t, userBiasaID, akunPelaku["target"].([]any)[0].(map[string]any)["pengguna_id"])

	// Perubahan kebijakan tercatat dengan sebelum/sesudah.
	assert.Len(t, s.auditAksi(t, "IMPERSONASI_KEBIJAKAN_UBAH"), 1)
	s.verifikasiRantai(t)
}

// ── Pelaku non-superadmin (QA) mengikuti kebijakan ────────────────────

func TestImpersonasi_PelakuNonSuperadminMengikutiKebijakan(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	// Tanpa kebijakan, QA tidak berhak.
	qa := s.mulaiSesi(t, emailQA, uaMac)
	w := s.panggil(t, http.MethodGet, "/v1/auth/impersonation/targets", nil, qa.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, false, s.dataObj(t, w)["boleh"])
	assert.Empty(t, s.dataObj(t, w)["peran"])
	h := s.mulaiImp(t, qa.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji"})
	assert.Equal(t, http.StatusForbidden, h.w.Code)
	assert.Equal(t, "IMPERSONASI_DITOLAK", h.kode())

	// Superadmin mengizinkan peran MANAJER_MUTU ke OPS_MAHASISWA saja.
	w = s.simpanKebijakan(t, admin.access, map[string]any{
		"versi": 0, "aktif": true, "wajib_alasan": false, "durasi_maks_menit": nil,
		"pelaku": []any{map[string]any{"tipe": "PERAN", "peran": "MANAJER_MUTU", "aktif": true,
			"target": []any{map[string]any{"tipe": "PERAN", "peran": "OPS_MAHASISWA"}}}},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = s.panggil(t, http.MethodGet, "/v1/auth/impersonation/targets", nil, qa.access)
	data := s.dataObj(t, w)
	assert.Equal(t, true, data["boleh"])
	assert.Equal(t, false, data["wajib_alasan"])
	assert.Nil(t, data["durasi_maks_menit"])
	require.Len(t, data["peran"], 1)
	assert.Equal(t, "OPS_MAHASISWA", data["peran"].([]any)[0].(map[string]any)["peran"])

	// Di luar daftar sasaran: ditolak, termasuk ADMIN_SISTEM.
	for _, badan := range []map[string]any{
		{"target_pengguna_id": userMFAID}, {"target_pengguna_id": userAdminID}, {"target_peran": "OPS_KOORDINATOR"}, {"target_peran": "ADMIN_SISTEM"},
	} {
		h = s.mulaiImp(t, qa.access, badan)
		assert.Equal(t, http.StatusForbidden, h.w.Code, "%v: %s", badan, h.w.Body.String())
	}

	// Alasan tidak wajib lagi; mulai ke akun yang diizinkan. Durasi tak terbatas: kedaluwarsa_pada kosong.
	imp := s.sesiDariStart(t, s.mulaiImp(t, qa.access, map[string]any{"target_pengguna_id": userBiasaID}).w)
	var kedaluwarsa struct{ KedaluwarsaPada *time.Time }
	require.NoError(t, s.db.Raw("SELECT kedaluwarsa_pada FROM impersonasi_sesi WHERE alasan_berakhir IS NULL").Scan(&kedaluwarsa).Error)
	assert.Nil(t, kedaluwarsa.KedaluwarsaPada)

	// Sedang impersonasi: pemilih sasaran dan hak tetap menurut pelaku asli (QA), bukan peran sasaran.
	w = s.panggil(t, http.MethodGet, "/v1/auth/impersonation/targets", nil, imp.access)
	assert.Equal(t, true, s.dataObj(t, w)["boleh"])

	// Ganti sasaran (bukan bersarang): hak dievaluasi terhadap QA; catatan lama BERGANTI; token lama mati.
	ganti := s.sesiDariStart(t, s.mulaiImp(t, imp.access, map[string]any{"target_pengguna_id": userOps2ID}).w)
	claims := mustClaims(t, s.tokenMgr, ganti.access)
	assert.Equal(t, userOps2ID, claims.UserID)
	require.NotNil(t, claims.Imp)
	assert.Equal(t, userQAID, claims.Imp.By.UserID, "pelaku tetap QA, bukan akun sasaran pertama")
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)
	var berganti int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM impersonasi_sesi WHERE alasan_berakhir = 'BERGANTI'").Scan(&berganti).Error)
	assert.EqualValues(t, 1, berganti)
	assert.Len(t, s.auditAksi(t, "IMPERSONASI_GANTI"), 1)
	// Ganti ke sasaran yang tidak diizinkan tetap ditolak, dan tidak memutus sesi yang berjalan.
	h = s.mulaiImp(t, ganti.access, map[string]any{"target_pengguna_id": userMFAID})
	assert.Equal(t, http.StatusForbidden, h.w.Code)
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, ganti.access).Code)

	// Berhenti mengembalikan QA (bukan admin).
	w = s.panggil(t, http.MethodPost, "/v1/auth/impersonation/stop", nil, ganti.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, userQAID, s.dataObj(t, w)["user"].(map[string]any)["id"])
	// Sesi asal QA dicabut saat impersonasi dimulai; yang hidup adalah sesi baru hasil berhenti.
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, qa.access).Code)
	qaKembali := s.dataObj(t, w)["access_token"].(string)

	// QA tidak pernah mengelola kebijakan, bahkan sesudah punya hak impersonasi.
	assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/admin/impersonation/policy", nil, qaKembali).Code)
	s.verifikasiRantai(t)
}

func TestImpersonasi_SaklarGlobalMenolakDanMengakhiriSesiBerjalan(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	qa := s.mulaiSesi(t, emailQA, uaMac)
	badan := map[string]any{
		"versi": 0, "aktif": true, "wajib_alasan": false, "durasi_maks_menit": 60,
		"pelaku": []any{map[string]any{"tipe": "PERAN", "peran": "MANAJER_MUTU", "aktif": true,
			"target": []any{map[string]any{"tipe": "PERAN", "peran": "OPS_MAHASISWA"}}}},
	}
	require.Equal(t, http.StatusOK, s.simpanKebijakan(t, admin.access, badan).Code)
	imp := s.sesiDariStart(t, s.mulaiImp(t, qa.access, map[string]any{"target_pengguna_id": userBiasaID}).w)
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)

	// Superadmin mematikan saklar: sesi QA berakhir pada request berikutnya (refresh juga ditolak), dan mulai baru ditolak.
	badan["versi"], badan["aktif"] = 1, false
	require.Equal(t, http.StatusOK, s.simpanKebijakan(t, admin.access, badan).Code)
	w := s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "IMPERSONATION_ENDED", s.kodeDari(t, w))
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, imp.refresh).Code)

	qa2 := s.mulaiSesi(t, emailQA, uaMac)
	h := s.mulaiImp(t, qa2.access, map[string]any{"target_pengguna_id": userBiasaID})
	assert.Equal(t, http.StatusForbidden, h.w.Code)
	assert.Equal(t, "IMPERSONASI_NONAKTIF", h.kode())
	// Superadmin pun terkena saklar global (kill switch), tetapi tetap bisa mengaturnya kembali.
	h = s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji"})
	assert.Equal(t, http.StatusForbidden, h.w.Code)
	assert.Equal(t, "IMPERSONASI_NONAKTIF", h.kode())

	var sebab string
	require.NoError(t, s.db.Raw("SELECT alasan_berakhir FROM impersonasi_sesi").Scan(&sebab).Error)
	assert.Equal(t, "KEBIJAKAN", sebab)
	assert.Len(t, s.auditAksi(t, "IMPERSONASI_BERAKHIR"), 1)
}

func TestImpersonasi_KebijakanDicabutMenghentikanPelakuYangTidakLagiBerhak(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	qa := s.mulaiSesi(t, emailQA, uaMac)
	badan := map[string]any{
		"versi": 0, "aktif": true, "wajib_alasan": false, "durasi_maks_menit": nil,
		"pelaku": []any{map[string]any{"tipe": "PERAN", "peran": "MANAJER_MUTU", "aktif": true,
			"target": []any{map[string]any{"tipe": "PERAN", "peran": "OPS_MAHASISWA"}}}},
	}
	require.Equal(t, http.StatusOK, s.simpanKebijakan(t, admin.access, badan).Code)
	imp := s.sesiDariStart(t, s.mulaiImp(t, qa.access, map[string]any{"target_pengguna_id": userBiasaID}).w)

	// Sasaran dikeluarkan dari daftar: sesi berjalan tidak lagi sah.
	badan["versi"] = 1
	badan["pelaku"] = []any{map[string]any{"tipe": "PERAN", "peran": "MANAJER_MUTU", "aktif": true,
		"target": []any{map[string]any{"tipe": "PERAN", "peran": "OPS_AGREGATOR"}}}}
	require.Equal(t, http.StatusOK, s.simpanKebijakan(t, admin.access, badan).Code)
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)
}

// ── Berakhir: kedaluwarsa, dinonaktifkan, dipaksa admin ───────────────

func TestImpersonasi_KedaluwarsaAkunNonaktifDanPengakhiranPaksa(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)

	// Kedaluwarsa menurut batas durasi.
	a := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji kedaluwarsa"}).w)
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, a.access).Code)
	require.NoError(t, s.db.Exec("UPDATE impersonasi_sesi SET kedaluwarsa_pada = now() - interval '1 minute' WHERE alasan_berakhir IS NULL").Error)
	w := s.panggil(t, http.MethodGet, "/v1/auth/me", nil, a.access)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "IMPERSONATION_ENDED", s.kodeDari(t, w))
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code)
	var sebab string
	require.NoError(t, s.db.Raw("SELECT alasan_berakhir FROM impersonasi_sesi").Scan(&sebab).Error)
	assert.Equal(t, "KEDALUWARSA", sebab)

	// Admin kehilangan sesinya saat mulai; masuk lagi, impersonasi baru, lalu sasaran dinonaktifkan.
	admin = s.mulaiSesi(t, emailAdmin, uaMac)
	b := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userOps2ID, "alasan": "uji nonaktif"}).w)
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, b.access).Code)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET status = 'NONAKTIF' WHERE id = ?", userOps2ID).Error)
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, b.access).Code)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET status = 'AKTIF' WHERE id = ?", userOps2ID).Error)

	// Pengakhiran paksa oleh superadmin dari sesi lain; riwayat mencatat sebab dan statusnya.
	admin = s.mulaiSesi(t, emailAdmin, uaMac)
	c := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji paksa"}).w)
	admin2 := s.mulaiSesi(t, emailAdmin, uaMac)

	w = s.panggil(t, http.MethodGet, "/v1/admin/impersonation/riwayat?status=aktif", nil, admin2.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	aktif := s.dataObj(t, w)["items"].([]any)
	require.Len(t, aktif, 1)
	idAktif := aktif[0].(map[string]any)["id"].(string)
	assert.Equal(t, true, aktif[0].(map[string]any)["aktif"])
	assert.Equal(t, "uji paksa", aktif[0].(map[string]any)["alasan"])

	w = s.panggil(t, http.MethodPost, "/v1/admin/impersonation/riwayat/"+idAktif+"/akhiri", nil, admin2.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, c.access).Code)
	// Sudah berakhir: 404, tidak ada efek ganda.
	assert.Equal(t, http.StatusNotFound, s.panggil(t, http.MethodPost, "/v1/admin/impersonation/riwayat/"+idAktif+"/akhiri", nil, admin2.access).Code)
	assert.Len(t, s.auditAksi(t, "IMPERSONASI_DIAKHIRI"), 1)

	w = s.panggil(t, http.MethodGet, "/v1/admin/impersonation/riwayat", nil, admin2.access)
	require.Equal(t, http.StatusOK, w.Code)
	semua := s.dataObj(t, w)["items"].([]any)
	require.Len(t, semua, 3)
	sebabPerId := map[string]string{}
	for _, it := range semua {
		m := it.(map[string]any)
		if m["alasan_berakhir"] != nil {
			sebabPerId[m["id"].(string)] = m["alasan_berakhir"].(string)
		}
		assert.Equal(t, userAdminID, m["pelaku"].(map[string]any)["id"])
	}
	assert.Equal(t, "DIAKHIRI", sebabPerId[idAktif])
	assert.Len(t, sebabPerId, 3, "ketiganya berakhir dengan sebab")
	s.verifikasiRantai(t)

	// Riwayat dan pengakhiran hanya untuk superadmin.
	assert.Equal(t, http.StatusForbidden, s.panggil(t, http.MethodGet, "/v1/admin/impersonation/riwayat", nil, s.mulaiSesi(t, emailBiasa, uaMac).access).Code)
}

func TestImpersonasi_SesiDicabutDiLuarAlurTampilBerakhirDicabut(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	imp := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji paksa keluar"}).w)
	admin2 := s.mulaiSesi(t, emailAdmin, uaMac)

	// Admin memaksa sasaran keluar dari semua sesi (termasuk sesi impersonasi yang dipinjam).
	w := s.panggil(t, http.MethodPost, "/v1/users/"+userBiasaID+"/revoke-sessions", nil, admin2.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)

	w = s.panggil(t, http.MethodGet, "/v1/admin/impersonation/riwayat", nil, admin2.access)
	items := s.dataObj(t, w)["items"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, false, items[0].(map[string]any)["aktif"])
	assert.Equal(t, "DICABUT", items[0].(map[string]any)["alasan_berakhir"])
}

// Token ber-klaim imp yang catatannya tidak cocok (dipalsukan atau dari sesi lain) ditolak walau tanda tangannya sah.
func TestImpersonasi_TokenDenganKlaimImpTidakCocokDitolak(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	imp := s.sesiDariStart(t, s.mulaiImp(t, admin.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji klaim"}).w)
	claims := mustClaims(t, s.tokenMgr, imp.access)

	forge := func(opts ...token.ClaimOption) string {
		t.Helper()
		tok, err := s.tokenMgr.CreateAccessToken(userMFAID, "OPS_KOORDINATOR", []string{}, opts...)
		require.NoError(t, err)
		return tok
	}
	// imp.id milik sesi lain / akun lain / pelaku lain.
	for nama, tok := range map[string]string{
		"id acak": forge(token.WithSession(claims.SessionID), token.WithImpersonation(token.Impersonation{
			ID: "00000000-0000-4000-8000-000000000009", By: token.ImpersonatedBy{UserID: userAdminID, Role: "ADMIN_SISTEM"}})),
		"akun beda dengan catatan": forge(token.WithSession(claims.SessionID), token.WithImpersonation(*claims.Imp)),
		"klaim cacat":              forge(token.WithSession(claims.SessionID), token.WithImpersonation(token.Impersonation{})),
		"tanpa sid":                forge(token.WithImpersonation(*claims.Imp)),
	} {
		w := s.panggil(t, http.MethodGet, "/v1/auth/me", nil, tok)
		assert.Equal(t, http.StatusUnauthorized, w.Code, nama)
	}
	// Token asli tetap sah.
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)

	// Penyerang tidak bisa berhenti sebagai pelaku dengan menempelkan klaim imp pada sesinya sendiri.
	biasa := s.mulaiSesi(t, emailBiasa, uaMac)
	palsu, err := s.tokenMgr.CreateAccessToken(userBiasaID, "OPS_MAHASISWA", mustClaims(t, s.tokenMgr, biasa.access).Permissions,
		token.WithSession(biasa.sid), token.WithImpersonation(*claims.Imp))
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodPost, "/v1/auth/impersonation/stop", nil, palsu).Code)
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code, "sesi impersonasi asli tidak terganggu")
}

func TestImpersonasi_AkunSasaranTidakAdaDanPeranTanpaAkun(t *testing.T) {
	s := suiteImpersonasi(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	for nama, badan := range map[string]map[string]any{
		"akun tidak ada":    {"target_pengguna_id": "00000000-0000-4000-8000-000000000001", "alasan": "uji"},
		"bukan uuid":        {"target_pengguna_id": "bukan-uuid", "alasan": "uji"},
		"akun nonaktif":     {"target_pengguna_id": uid("test-user-002"), "alasan": "uji"},
		"peran tanpa akun":  {"target_peran": "STRATEGIS_BPS", "alasan": "uji"},
		"peran tak dikenal": {"target_peran": "TUKANG", "alasan": "uji"},
	} {
		w := s.panggil(t, http.MethodPost, "/v1/auth/impersonation/start", badan, admin.access)
		assert.Equal(t, http.StatusNotFound, w.Code, "%s: %s", nama, w.Body.String())
	}
	w := s.panggil(t, http.MethodPost, "/v1/auth/impersonation/start", map[string]any{"alasan": "uji"}, admin.access)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = s.panggil(t, http.MethodPost, "/v1/auth/impersonation/start", map[string]any{"target_pengguna_id": userBiasaID, "alasan": fmt.Sprintf("%0501d", 0)}, admin.access)
	assert.Equal(t, http.StatusBadRequest, w.Code, "alasan > 500 karakter")
	// Penolakan tercatat DITOLAK di audit atas nama pelaku, dan sesi admin tidak terputus oleh percobaan yang gagal.
	assert.NotEmpty(t, s.auditAksi(t, "IMPERSONASI_MULAI"))
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, admin.access).Code)
}

// Pelaku yang dipaksa keluar (atau mencabut semua sesinya sendiri) tidak boleh tetap memegang sesi impersonasi: sesi itu
// tercatat atas user_id sasaran, jadi pencabutan sesi pelaku harus ikut menjangkaunya.
func TestImpersonasi_PaksaKeluarPelakuMencabutSesiImpersonasiYangDimulainya(t *testing.T) {
	s := suiteImpersonasi(t)
	s.tambahAdminKedua(t)
	pertama := s.mulaiSesi(t, emailAdmin, uaMac)
	imp := s.sesiDariStart(t, s.mulaiImp(t, pertama.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji paksa keluar pelaku"}).w)
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)

	kedua := s.mulaiSesi(t, emailKedua, uaMac)
	w := s.panggil(t, http.MethodPost, "/v1/users/"+userAdminID+"/revoke-sessions", nil, kedua.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code, "sesi impersonasi milik pelaku ikut mati")
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, imp.refresh).Code)
}

// "Keluar dari semua perangkat" oleh pelaku di sesi biasa lain juga mengakhiri impersonasi yang ia mulai di perangkat lain.
func TestImpersonasi_KeluarDariSemuaPerangkatMenjangkauSesiImpersonasi(t *testing.T) {
	s := suiteImpersonasi(t)
	perangkatA := s.mulaiSesi(t, emailAdmin, uaMac)
	imp := s.sesiDariStart(t, s.mulaiImp(t, perangkatA.access, map[string]any{"target_pengguna_id": userBiasaID, "alasan": "uji logout-all"}).w)
	perangkatB := s.mulaiSesi(t, emailAdmin, uaMac+"-B")

	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/sessions/revoke-others", nil, perangkatB.access).Code)

	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, imp.access).Code)
	assert.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/auth/me", nil, perangkatB.access).Code, "sesi yang dipertahankan tetap hidup")
}
