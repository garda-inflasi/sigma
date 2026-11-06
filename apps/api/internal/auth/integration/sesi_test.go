package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	uaMac     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	uaAndroid = "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"
	uaIPhone  = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1"

	emailAdmin = "test@gardainflasi.id"
	userBiasa  = "test-user-biasa"
)

// sesiUji adalah satu sesi hasil login: pasangan token dan id sesi (sid).
type sesiUji struct {
	access  string
	refresh string
	sid     string
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// mulaiSesi login sebagai email dengan user agent tertentu.
func (s *Suite) mulaiSesi(t *testing.T, email, ua string) sesiUji {
	t.Helper()
	w := s.serveRequest(t, http.MethodPost, "/v1/auth/login",
		map[string]any{"email": email, "password": "Test1234!"}, map[string]string{"User-Agent": ua})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.parseResponse(t, w)["data"].(map[string]any)
	access, refresh := data["access_token"].(string), data["refresh_token"].(string)
	claims, err := s.tokenMgr.VerifyToken(access)
	require.NoError(t, err)
	require.NotEmpty(t, claims.SessionID, "access token membawa sid")
	return sesiUji{access: access, refresh: refresh, sid: claims.SessionID}
}

// panggil mengirim request terotentikasi dan mengembalikan rekaman respons.
func (s *Suite) panggil(t *testing.T, method, path string, body any, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	var headers map[string]string
	if accessToken != "" {
		headers = s.authHeader(accessToken)
	}
	return s.serveRequest(t, method, path, body, headers)
}

func (s *Suite) refreshRaw(t *testing.T, refreshToken string) *httptest.ResponseRecorder {
	t.Helper()
	return s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]any{"refresh_token": refreshToken}, nil)
}

// ageRotation memundurkan waktu rotasi token ke satu menit lalu: di luar jendela toleransi.
func (s *Suite) ageRotation(t *testing.T, refreshToken string) {
	t.Helper()
	res := s.db.Exec("UPDATE refresh_tokens SET rotated_at = now() - interval '1 minute' WHERE token_hash = ? AND rotated_at IS NOT NULL", hashToken(refreshToken))
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected, "token harus sudah dirotasi")
}

func (s *Suite) liveTokens(t *testing.T, sid string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM refresh_tokens WHERE family_id = ? AND revoked_at IS NULL", sid).Scan(&n).Error)
	return n
}

func (s *Suite) auditCount(t *testing.T, aksi, aktor, entitasID string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = ? AND aktor_id = ? AND entitas_id = ?", aksi, aktor, entitasID).Scan(&n).Error)
	return n
}

func (s *Suite) dataObj(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	data, ok := s.parseResponse(t, w)["data"].(map[string]any)
	require.True(t, ok, w.Body.String())
	return data
}

func (s *Suite) daftarSesi(t *testing.T, accessToken string) []map[string]any {
	t.Helper()
	w := s.panggil(t, http.MethodGet, "/v1/auth/sessions", nil, accessToken)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	raw, ok := s.parseResponse(t, w)["data"].([]any)
	require.True(t, ok, w.Body.String())
	out := make([]map[string]any, len(raw))
	for i, r := range raw {
		out[i] = r.(map[string]any)
	}
	return out
}

func (s *Suite) status(t *testing.T, accessToken string) int {
	t.Helper()
	return s.panggil(t, http.MethodGet, "/v1/auth/me", nil, accessToken).Code
}

// suiteDenganToleransi seperti setupSuite dengan jendela toleransi refresh tertentu (0 = tanpa toleransi).
func suiteDenganToleransi(t *testing.T, grace time.Duration) *Suite {
	t.Helper()
	cfg := routes.ConfigFromEnv(os.Getenv)
	cfg.RefreshGraceWindow = grace
	return newSuite(t, seedData, routes.Options{UseCaseConfig: cfg})
}

// ── Penegakan pencabutan access token ─────────────────────────────────

func TestSesi_LogoutRejectsTheAccessTokenAtOnce(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	require.Equal(t, http.StatusOK, s.status(t, a.access))

	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, a.access).Code)

	assert.Equal(t, http.StatusUnauthorized, s.status(t, a.access), "access token yang sudah logout tidak boleh dipakai lagi")
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code, "refresh token sesi itu juga mati")
}

func TestSesi_LogoutAlsoKillsEarlierAccessTokensOfTheSession(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	w := s.refreshRaw(t, a.refresh)
	require.Equal(t, http.StatusOK, w.Code)
	baru := s.dataObj(t, w)["access_token"].(string)
	require.Equal(t, http.StatusOK, s.status(t, a.access), "token lama masih berlaku sampai kedaluwarsa")

	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, baru).Code)

	assert.Equal(t, http.StatusUnauthorized, s.status(t, a.access), "sid yang sama: semua access token sesi ikut mati")
	assert.Equal(t, http.StatusUnauthorized, s.status(t, baru))
}

func TestSesi_BlacklistedJTIIsEnforcedForTokensWithoutSid(t *testing.T) {
	s := setupSuite(t)
	legacy, err := s.tokenMgr.CreateAccessToken("test-user-001", "ADMIN_SISTEM", usecase.IzinBawaan("ADMIN_SISTEM"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, s.status(t, legacy), "token tanpa sid tetap diterima selama jti tidak dicabut")

	claims, err := s.tokenMgr.VerifyToken(legacy)
	require.NoError(t, err)
	require.NoError(t, s.db.Exec("INSERT INTO jwt_blacklist (jti, expires_at) VALUES (?, now() + interval '15 minutes')", claims.ID).Error)

	assert.Equal(t, http.StatusUnauthorized, s.status(t, legacy))

	require.NoError(t, s.db.Exec("UPDATE jwt_blacklist SET expires_at = now() - interval '1 second' WHERE jti = ?", claims.ID).Error)
	assert.Equal(t, http.StatusOK, s.status(t, legacy), "entri daftar cabut yang kedaluwarsa diabaikan")
}

func TestSesi_RevokedSessionRowRejectsTokenWithoutAnyBlacklistEntry(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	require.Equal(t, http.StatusOK, s.status(t, a.access))

	require.NoError(t, s.db.Exec("UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = ?", a.sid).Error)

	assert.Equal(t, http.StatusUnauthorized, s.status(t, a.access))
}

func TestSesi_EnforcementAppliesToEveryModuleSharingTheTokenManager(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodGet, "/v1/master/komoditas", nil, a.access).Code)

	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, a.access).Code)

	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/master/komoditas", nil, a.access).Code,
		"JWTAuth modul lain memakai pemeriksa yang sama")
}

func TestSesi_CookieAccessTokenIsEnforcedToo(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	require.Equal(t, http.StatusOK, s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, map[string]string{"Cookie": "garda_access=" + a.access}).Code)

	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, a.access).Code)

	assert.Equal(t, http.StatusUnauthorized, s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, map[string]string{"Cookie": "garda_access=" + a.access}).Code)
}

func TestSesi_RevocationLookupFailureFailsClosedWith503(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	require.NoError(t, s.db.Exec("DROP TABLE jwt_blacklist").Error)

	w := s.panggil(t, http.MethodGet, "/v1/auth/me", nil, a.access)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "tidak diloloskan dan bukan 401 (klien tidak boleh menyangka sesi berakhir)")
}

// ── Login dan metadata sesi ───────────────────────────────────────────

func TestSesi_LoginRecordsClientAndRefreshUpdatesLastUsed(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)

	var row struct {
		UserAgent        string
		IP               string
		LastUsedAt       time.Time
		SessionStartedAt time.Time
	}
	require.NoError(t, s.db.Raw("SELECT user_agent, ip, last_used_at, session_started_at FROM refresh_tokens WHERE family_id = ?", a.sid).Scan(&row).Error)
	assert.Equal(t, uaMac, row.UserAgent)
	assert.Equal(t, "192.0.2.1", row.IP)
	assert.WithinDuration(t, time.Now(), row.LastUsedAt, time.Minute)

	// Mundurkan jam sesi, lalu refresh dari peramban yang sudah diperbarui.
	require.NoError(t, s.db.Exec("UPDATE refresh_tokens SET last_used_at = now() - interval '2 hours', session_started_at = now() - interval '3 hours' WHERE family_id = ?", a.sid).Error)
	var awal time.Time
	require.NoError(t, s.db.Raw("SELECT session_started_at FROM refresh_tokens WHERE family_id = ?", a.sid).Scan(&awal).Error)

	w := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]any{"refresh_token": a.refresh}, map[string]string{"User-Agent": uaMac + " Edg/121"})
	require.Equal(t, http.StatusOK, w.Code)
	rotated := s.dataObj(t, w)["refresh_token"].(string)

	var next struct {
		UserAgent        string
		LastUsedAt       time.Time
		SessionStartedAt time.Time
		Family           string
	}
	require.NoError(t, s.db.Raw("SELECT user_agent, last_used_at, session_started_at, family_id::text AS family FROM refresh_tokens WHERE token_hash = ?", hashToken(rotated)).Scan(&next).Error)
	assert.Equal(t, a.sid, next.Family, "rotasi tetap dalam sesi yang sama")
	assert.WithinDuration(t, time.Now(), next.LastUsedAt, time.Minute, "refresh memperbarui last_used_at")
	assert.WithinDuration(t, awal, next.SessionStartedAt, time.Second, "awal sesi diwariskan")
	assert.Contains(t, next.UserAgent, "Edg/121", "user agent terbaru dicatat")
}

// ── Rotasi, atomisitas, jendela toleransi, pemakaian ulang ────────────

func TestSesi_RotationRevokesOldTokenAndKeepsExactlyOneLiveToken(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.refreshRaw(t, a.refresh)
	require.Equal(t, http.StatusOK, w.Code)
	data := s.dataObj(t, w)
	assert.NotEmpty(t, data["refresh_token"])
	assert.NotEqual(t, a.refresh, data["refresh_token"])

	var old struct {
		RevokedAt *time.Time
		RotatedAt *time.Time
	}
	require.NoError(t, s.db.Raw("SELECT revoked_at, rotated_at FROM refresh_tokens WHERE token_hash = ?", hashToken(a.refresh)).Scan(&old).Error)
	assert.NotNil(t, old.RevokedAt)
	assert.NotNil(t, old.RotatedAt, "dicabut karena diganti")
	assert.EqualValues(t, 1, s.liveTokens(t, a.sid))

	claims, err := s.tokenMgr.VerifyToken(data["access_token"].(string))
	require.NoError(t, err)
	assert.Equal(t, a.sid, claims.SessionID, "access token baru tetap terikat ke sesi yang sama")
}

func TestSesi_ConcurrentRefreshWithSameTokenRotatesExactlyOnce(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)

	const n = 16
	codes := make([]int, n)
	refreshBaru := make([]string, n)
	akses := make([]string, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			w := s.refreshRaw(t, a.refresh)
			codes[i] = w.Code
			if w.Code == http.StatusOK {
				if data, ok := s.parseResponse(t, w)["data"].(map[string]any); ok {
					refreshBaru[i], _ = data["refresh_token"].(string)
					akses[i], _ = data["access_token"].(string)
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	pemenang := ""
	jumlahPemenang := 0
	for i := 0; i < n; i++ {
		assert.Equal(t, http.StatusOK, codes[i], "refresh serentak dari tab yang sah tidak boleh gagal")
		if refreshBaru[i] != "" {
			pemenang = refreshBaru[i]
			jumlahPemenang++
		}
	}
	assert.Equal(t, 1, jumlahPemenang, "tepat satu permintaan menerima refresh token pengganti")
	assert.EqualValues(t, 1, s.liveTokens(t, a.sid), "tepat satu token hidup tersisa di sesi")
	assert.EqualValues(t, 0, s.auditCount(t, "TOKEN_REUSE", "test-user-001", ""), "tidak ada deteksi pemakaian ulang palsu")
	var reuse int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'TOKEN_REUSE'").Scan(&reuse).Error)
	assert.Zero(t, reuse)

	// Sesi tetap dapat dipakai: token pemenang dan semua access token yang diterbitkan.
	for _, at := range akses {
		if at != "" {
			assert.Equal(t, http.StatusOK, s.status(t, at))
		}
	}
	require.NotEmpty(t, pemenang)
	assert.Equal(t, http.StatusOK, s.refreshRaw(t, pemenang).Code)
}

func TestSesi_ConcurrentRefreshWithoutGraceAllowsOnlyOneWinner(t *testing.T) {
	s := suiteDenganToleransi(t, 0)
	a := s.mulaiSesi(t, emailAdmin, uaMac)

	const n = 12
	codes := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			codes[i] = s.refreshRaw(t, a.refresh).Code
		}()
	}
	close(start)
	wg.Wait()

	menang := 0
	for _, c := range codes {
		if c == http.StatusOK {
			menang++
		} else {
			assert.Equal(t, http.StatusUnauthorized, c)
		}
	}
	assert.Equal(t, 1, menang, "tanpa toleransi tepat satu refresh serentak yang lolos (pengecekan-lalu-cabut atomik)")
	assert.Zero(t, s.liveTokens(t, a.sid), "yang kalah memicu deteksi reuse: seluruh family dicabut")
}

func TestSesi_ReplayWithinGraceGivesAccessTokenOnlyAndKeepsSessionUsable(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)

	pertama := s.refreshRaw(t, a.refresh)
	require.Equal(t, http.StatusOK, pertama.Code)
	pemenang := s.dataObj(t, pertama)["refresh_token"].(string)

	kedua := s.refreshRaw(t, a.refresh) // tab kedua dengan cookie lama
	require.Equal(t, http.StatusOK, kedua.Code)
	data := s.dataObj(t, kedua)
	assert.NotEmpty(t, data["access_token"])
	assert.Empty(t, data["refresh_token"], "tidak ada refresh token kedua: cookie bersama tetap milik pemenang")
	assert.Equal(t, http.StatusOK, s.status(t, data["access_token"].(string)))
	assert.EqualValues(t, 1, s.liveTokens(t, a.sid))
	assert.Equal(t, http.StatusOK, s.refreshRaw(t, pemenang).Code, "token pemenang tetap sah")
}

func TestSesi_ReuseOutsideGraceRevokesFamilyAccessTokensAndAudits(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	lain := s.mulaiSesi(t, emailAdmin, uaAndroid)

	w := s.refreshRaw(t, a.refresh)
	require.Equal(t, http.StatusOK, w.Code)
	data := s.dataObj(t, w)
	pemenang, aksesBaru := data["refresh_token"].(string), data["access_token"].(string)
	require.Equal(t, http.StatusOK, s.status(t, aksesBaru))

	s.ageRotation(t, a.refresh)
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code, "token lama dipakai ulang")

	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, pemenang).Code, "refresh token pengganti ikut mati")
	assert.Equal(t, http.StatusUnauthorized, s.status(t, aksesBaru), "access token sesi itu mati seketika")
	assert.Equal(t, http.StatusUnauthorized, s.status(t, a.access))
	assert.Zero(t, s.liveTokens(t, a.sid))
	assert.Equal(t, http.StatusOK, s.status(t, lain.access), "sesi perangkat lain tidak terpengaruh")

	var n int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'TOKEN_REUSE' AND aktor_id = 'test-user-001' AND metadata->>'family_id' = ?", a.sid).Scan(&n).Error)
	assert.EqualValues(t, 1, n, "TOKEN_REUSE tercatat")
}

func TestSesi_ReplayAfterSessionRevokedDoesNotResurrectIt(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	w := s.refreshRaw(t, a.refresh)
	require.Equal(t, http.StatusOK, w.Code)
	baru := s.dataObj(t, w)["access_token"].(string)
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, baru).Code)

	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code, "logout di dalam jendela toleransi tetap mengakhiri sesi")
}

func TestSesi_RevokedTokenReplayIsPlainInvalidNotReuse(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, a.access).Code)

	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code)

	var n int64
	require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'TOKEN_REUSE'").Scan(&n).Error)
	assert.Zero(t, n, "sesi yang sudah dicabut bukan pencurian token")
}

func TestSesi_RefreshRejectsExpiredTokenAndDeactivatedAccount(t *testing.T) {
	s := setupSuite(t)

	t.Run("token kedaluwarsa", func(t *testing.T) {
		a := s.mulaiSesi(t, emailBiasa, uaMac)
		require.NoError(t, s.db.Exec("UPDATE refresh_tokens SET expires_at = now() - interval '1 minute' WHERE family_id = ?", a.sid).Error)
		assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code)
	})
	t.Run("akun dinonaktifkan di basis data tanpa lewat API", func(t *testing.T) {
		a := s.mulaiSesi(t, emailBiasa, uaMac)
		require.NoError(t, s.db.Exec("UPDATE pengguna SET status = 'NONAKTIF' WHERE id = ?", userBiasa).Error)
		assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code)
		require.NoError(t, s.db.Exec("UPDATE pengguna SET status = 'AKTIF' WHERE id = ?", userBiasa).Error)
	})
}

// ── Daftar, pencabutan, keluar ────────────────────────────────────────

func TestSesi_ListShowsOwnActiveSessionsWithCurrentMarked(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	b := s.mulaiSesi(t, emailAdmin, uaAndroid)
	s.mulaiSesi(t, emailBiasa, uaIPhone) // sesi milik orang lain tidak boleh muncul

	items := s.daftarSesi(t, a.access)

	require.Len(t, items, 2)
	oleh := map[string]map[string]any{}
	for _, it := range items {
		oleh[it["id"].(string)] = it
	}
	require.Contains(t, oleh, a.sid)
	require.Contains(t, oleh, b.sid)
	assert.Equal(t, true, oleh[a.sid]["saat_ini"])
	assert.Equal(t, false, oleh[b.sid]["saat_ini"])
	assert.Equal(t, "Chrome di macOS", oleh[a.sid]["perangkat"])
	assert.Equal(t, "Chrome di Android", oleh[b.sid]["perangkat"])
	assert.Equal(t, uaAndroid, oleh[b.sid]["user_agent"])
	assert.Equal(t, "192.0.2.1", oleh[a.sid]["ip"])
	for _, k := range []string{"dibuat_pada", "terakhir_dipakai_pada", "kedaluwarsa_pada"} {
		assert.NotEmpty(t, oleh[a.sid][k], k)
	}

	// Setelah rotasi tetap satu entri per sesi, dengan awal sesi yang sama.
	s.refreshRaw(t, a.refresh)
	assert.Len(t, s.daftarSesi(t, a.access), 2)
}

func TestSesi_ListRequiresAuthentication(t *testing.T) {
	s := setupSuite(t)
	assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodGet, "/v1/auth/sessions", nil, "").Code)
}

func TestSesi_RevokeOneSessionKillsItsTokensAtOnceAndIsAudited(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	b := s.mulaiSesi(t, emailAdmin, uaAndroid)
	require.Equal(t, http.StatusOK, s.status(t, b.access))

	w := s.panggil(t, http.MethodDelete, "/v1/auth/sessions/"+b.sid, nil, a.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, http.StatusUnauthorized, s.status(t, b.access), "access token sesi yang dicabut langsung 401")
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, b.refresh).Code, "refresh token-nya gagal")
	assert.Equal(t, http.StatusOK, s.status(t, a.access), "sesi pemanggil tidak terpengaruh")
	assert.Len(t, s.daftarSesi(t, a.access), 1)
	assert.EqualValues(t, 1, s.auditCount(t, "REVOKE_SESSION", "test-user-001", b.sid))
}

func TestSesi_RevokeRejectsCurrentUnknownAndForeignSessions(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	orangLain := s.mulaiSesi(t, emailBiasa, uaIPhone)

	t.Run("sesi yang sedang dipakai", func(t *testing.T) {
		w := s.panggil(t, http.MethodDelete, "/v1/auth/sessions/"+a.sid, nil, a.access)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, http.StatusOK, s.status(t, a.access))
	})
	t.Run("sesi tidak ada", func(t *testing.T) {
		w := s.panggil(t, http.MethodDelete, "/v1/auth/sessions/00000000-0000-4000-8000-000000000000", nil, a.access)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
	t.Run("id bukan UUID", func(t *testing.T) {
		w := s.panggil(t, http.MethodDelete, "/v1/auth/sessions/bukan-uuid", nil, a.access)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
	t.Run("sesi milik pengguna lain tidak bisa dicabut (dan tidak bocor)", func(t *testing.T) {
		w := s.panggil(t, http.MethodDelete, "/v1/auth/sessions/"+orangLain.sid, nil, a.access)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, http.StatusOK, s.status(t, orangLain.access), "sesi orang lain tetap hidup")
	})
	t.Run("sudah dicabut", func(t *testing.T) {
		b := s.mulaiSesi(t, emailAdmin, uaAndroid)
		require.Equal(t, http.StatusOK, s.panggil(t, http.MethodDelete, "/v1/auth/sessions/"+b.sid, nil, a.access).Code)
		assert.Equal(t, http.StatusNotFound, s.panggil(t, http.MethodDelete, "/v1/auth/sessions/"+b.sid, nil, a.access).Code)
	})
}

func TestSesi_RevokeOthersKeepsOnlyTheCurrentSession(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	b := s.mulaiSesi(t, emailAdmin, uaAndroid)
	c := s.mulaiSesi(t, emailAdmin, uaIPhone)
	orangLain := s.mulaiSesi(t, emailBiasa, uaMac)

	w := s.panggil(t, http.MethodPost, "/v1/auth/sessions/revoke-others", nil, a.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 2, s.dataObj(t, w)["dicabut"])

	assert.Equal(t, http.StatusOK, s.status(t, a.access))
	for _, x := range []sesiUji{b, c} {
		assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access))
		assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, x.refresh).Code)
	}
	assert.Equal(t, http.StatusOK, s.status(t, orangLain.access), "pengguna lain tidak tersentuh")
	assert.EqualValues(t, 1, s.auditCount(t, "REVOKE_OTHER_SESSIONS", "test-user-001", "test-user-001"))
}

func TestSesi_RevokeOthersWithTokenWithoutSidAsksToLoginAgain(t *testing.T) {
	s := setupSuite(t)
	legacy, err := s.tokenMgr.CreateAccessToken("test-user-001", "ADMIN_SISTEM", usecase.IzinBawaan("ADMIN_SISTEM"))
	require.NoError(t, err)
	s.mulaiSesi(t, emailAdmin, uaMac)

	w := s.panggil(t, http.MethodPost, "/v1/auth/sessions/revoke-others", nil, legacy)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Len(t, s.daftarSesi(t, s.mulaiSesi(t, emailAdmin, uaMac).access), 2, "tidak ada sesi yang dicabut diam-diam")
}

func TestSesi_LogoutEndsOnlyTheCurrentSession(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	b := s.mulaiSesi(t, emailAdmin, uaAndroid)

	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", nil, a.access).Code)

	assert.Equal(t, http.StatusUnauthorized, s.status(t, a.access))
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code)
	assert.Equal(t, http.StatusOK, s.status(t, b.access), "perangkat lain tetap masuk")
	assert.Equal(t, http.StatusOK, s.refreshRaw(t, b.refresh).Code)
	assert.EqualValues(t, 1, s.auditCount(t, "LOGOUT", "test-user-001", "test-user-001"))
}

func TestSesi_LogoutWithRefreshTokenBodyFallsBackForTokenWithoutSid(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	b := s.mulaiSesi(t, emailAdmin, uaAndroid)
	legacy, err := s.tokenMgr.CreateAccessToken("test-user-001", "ADMIN_SISTEM", usecase.IzinBawaan("ADMIN_SISTEM"))
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodPost, "/v1/auth/logout", map[string]any{"refresh_token": a.refresh}, legacy).Code)

	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code, "family dari refresh token yang dikirim")
	assert.Equal(t, http.StatusOK, s.refreshRaw(t, b.refresh).Code, "sesi lain utuh")
	assert.Equal(t, http.StatusUnauthorized, s.status(t, legacy), "jti masuk daftar cabut")
}

func TestSesi_LogoutAllEndsEverySessionIncludingCurrent(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	b := s.mulaiSesi(t, emailAdmin, uaAndroid)
	orangLain := s.mulaiSesi(t, emailBiasa, uaMac)

	w := s.panggil(t, http.MethodPost, "/v1/auth/logout-all", nil, a.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 2, s.dataObj(t, w)["dicabut"])

	for _, x := range []sesiUji{a, b} {
		assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access))
		assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, x.refresh).Code)
	}
	assert.Equal(t, http.StatusOK, s.status(t, orangLain.access))
	assert.EqualValues(t, 1, s.auditCount(t, "LOGOUT_ALL", "test-user-001", "test-user-001"))
}

// ── Admin: paksa keluar ───────────────────────────────────────────────

func TestSesi_AdminForceLogoutEndsAllSessionsOfTheTargetAndIsAudited(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	t1 := s.mulaiSesi(t, emailBiasa, uaAndroid)
	t2 := s.mulaiSesi(t, emailBiasa, uaIPhone)

	w := s.panggil(t, http.MethodPost, "/v1/users/"+userBiasa+"/revoke-sessions", nil, admin.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.dataObj(t, w)
	assert.EqualValues(t, 2, data["dicabut"])
	assert.Equal(t, userBiasa, data["id"])

	for _, x := range []sesiUji{t1, t2} {
		assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access), "access token target mati seketika")
		assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, x.refresh).Code)
	}
	assert.Equal(t, http.StatusOK, s.status(t, admin.access), "sesi admin tidak terpengaruh")
	assert.EqualValues(t, 1, s.auditCount(t, "FORCE_LOGOUT", "test-user-001", userBiasa), "aktornya admin, sasarannya pengguna")
	// Pengguna itu bisa masuk lagi.
	s.mulaiSesi(t, emailBiasa, uaMac)
}

func TestSesi_ForceLogoutRequiresPermissionAndKnownUser(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	biasa := s.mulaiSesi(t, emailBiasa, uaAndroid)

	t.Run("tanpa izin pengguna.kelola", func(t *testing.T) {
		w := s.panggil(t, http.MethodPost, "/v1/users/test-user-001/revoke-sessions", nil, biasa.access)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, http.StatusOK, s.status(t, admin.access), "sesi admin tetap hidup")
	})
	t.Run("tanpa token", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, s.panggil(t, http.MethodPost, "/v1/users/"+userBiasa+"/revoke-sessions", nil, "").Code)
	})
	t.Run("pengguna tidak ada", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, s.panggil(t, http.MethodPost, "/v1/users/tidak-ada/revoke-sessions", nil, admin.access).Code)
		assert.Zero(t, s.auditCount(t, "FORCE_LOGOUT", "test-user-001", "tidak-ada"))
	})
}

// ── Konsistensi pencabutan di alur lain ───────────────────────────────

func TestSesi_ChangePasswordKeepsCurrentSessionAndRevokesTheOthers(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	b := s.mulaiSesi(t, emailAdmin, uaAndroid)
	var awalSebelum time.Time
	require.NoError(t, s.db.Raw("SELECT min(session_started_at) FROM refresh_tokens WHERE family_id = ?", a.sid).Scan(&awalSebelum).Error)

	w := s.panggil(t, http.MethodPost, "/v1/auth/change-password",
		map[string]any{"old_password": "Test1234!", "new_password": "NewTest5678@"}, a.access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := s.dataObj(t, w)
	baru, refreshBaru := data["access_token"].(string), data["refresh_token"].(string)

	claims, err := s.tokenMgr.VerifyToken(baru)
	require.NoError(t, err)
	assert.Equal(t, a.sid, claims.SessionID, "sesi yang sedang dipakai tetap sama")
	assert.Equal(t, http.StatusOK, s.status(t, baru))
	assert.Equal(t, http.StatusOK, s.status(t, a.access), "token lama sesi yang sama masih sah sampai kedaluwarsa")
	assert.Equal(t, http.StatusUnauthorized, s.status(t, b.access), "sesi lain dicabut")
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, b.refresh).Code)
	assert.EqualValues(t, 1, s.liveTokens(t, a.sid), "satu token hidup di sesi ini")
	assert.Equal(t, http.StatusOK, s.refreshRaw(t, refreshBaru).Code, "refresh token baru berfungsi")

	items := s.daftarSesi(t, baru)
	require.Len(t, items, 1)
	assert.Equal(t, a.sid, items[0]["id"])
	assert.Equal(t, true, items[0]["saat_ini"])
	var awalSesudah time.Time
	require.NoError(t, s.db.Raw("SELECT min(session_started_at) FROM refresh_tokens WHERE family_id = ?", a.sid).Scan(&awalSesudah).Error)
	assert.WithinDuration(t, awalSebelum, awalSesudah, time.Second, "awal sesi tidak ikut tergeser")
}

func TestSesi_AdminActionsRevokeSessionsImmediately(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	pulihkan := func(t *testing.T) {
		t.Helper()
		require.NoError(t, s.db.Exec("UPDATE pengguna SET status = 'AKTIF' WHERE id = ?", userBiasa).Error)
	}

	t.Run("nonaktifkan akun", func(t *testing.T) {
		x := s.mulaiSesi(t, emailBiasa, uaIPhone)
		w := s.panggil(t, http.MethodPatch, "/v1/users/"+userBiasa, map[string]any{"status": "NONAKTIF"}, admin.access)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access))
		assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, x.refresh).Code)
		pulihkan(t)
	})
	t.Run("nonaktifkan lewat aksi massal", func(t *testing.T) {
		x := s.mulaiSesi(t, emailBiasa, uaIPhone)
		w := s.panggil(t, http.MethodPost, "/v1/users/bulk", map[string]any{"action": "nonaktifkan", "ids": []string{userBiasa}}, admin.access)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access))
		pulihkan(t)
	})
	t.Run("akhiri sesi lewat aksi massal", func(t *testing.T) {
		x := s.mulaiSesi(t, emailBiasa, uaIPhone)
		w := s.panggil(t, http.MethodPost, "/v1/users/bulk", map[string]any{"action": "akhiri_sesi", "ids": []string{userBiasa}}, admin.access)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access))
	})
	t.Run("reset kata sandi oleh admin", func(t *testing.T) {
		x := s.mulaiSesi(t, emailBiasa, uaIPhone)
		w := s.panggil(t, http.MethodPost, "/v1/users/"+userBiasa+"/reset-password", map[string]any{"new_password": "Admin#Reset2026"}, admin.access)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access))
		assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, x.refresh).Code)
	})
	t.Run("ganti peran", func(t *testing.T) {
		require.NoError(t, s.db.Exec("UPDATE pengguna SET password_hash = (SELECT password_hash FROM pengguna WHERE id = 'test-user-001'), wajib_ganti_password = false WHERE id = ?", userBiasa).Error)
		x := s.mulaiSesi(t, emailBiasa, uaIPhone)
		w := s.panggil(t, http.MethodPut, "/v1/users/"+userBiasa+"/roles", map[string]any{"peran": "OPS_KOORDINATOR"}, admin.access)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access))
	})
	assert.Equal(t, http.StatusOK, s.status(t, admin.access), "sesi admin sendiri tidak ikut tercabut")
}

func TestSesi_PasswordResetRevokesAccessTokensToo(t *testing.T) {
	s := setupSuite(t)
	a := s.mulaiSesi(t, emailAdmin, uaMac)
	s.simpanTokenReset(t, "token-sesi", time.Now().Add(30*time.Minute))

	require.Equal(t, http.StatusOK, s.reset(t, "token-sesi", passwordBaru).Code)

	assert.Equal(t, http.StatusUnauthorized, s.status(t, a.access), "access token tidak hidup sampai 15 menit setelah reset")
	assert.Equal(t, http.StatusUnauthorized, s.refreshRaw(t, a.refresh).Code)
}

func TestSesi_DeletingTheUserEndsItsSessions(t *testing.T) {
	s := setupSuite(t)
	admin := s.mulaiSesi(t, emailAdmin, uaMac)
	require.NoError(t, s.db.Exec(`INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga, status)
		SELECT 'user-hapus', 'Dihapus', 'hapus@gardainflasi.id', password_hash, 'OPS_MAHASISWA', 'OPERASIONAL', 'BPPSDMP', 'AKTIF' FROM pengguna WHERE id = 'test-user-001'`).Error)
	x := s.mulaiSesi(t, "hapus@gardainflasi.id", uaMac)

	require.Equal(t, http.StatusOK, s.panggil(t, http.MethodDelete, "/v1/users/user-hapus", nil, admin.access).Code)

	assert.Equal(t, http.StatusUnauthorized, s.status(t, x.access))
}

// ── Kompatibilitas token lama ─────────────────────────────────────────

func TestSesi_TokenManagerWithoutAuthModuleDoesNotEnforceRevocation(t *testing.T) {
	// Router modul lain tanpa modul auth (tes modul) memakai Manager tanpa pemeriksa: token sah tetap lolos.
	m := token.NewManager("secret-tanpa-modul-auth-32-chars!!")
	tokenStr, err := m.CreateAccessToken("u", "ADMIN_SISTEM", nil, token.WithSession("3f1c8a52-0a57-4b9a-8f0e-0d1f6f3c9a11"))
	require.NoError(t, err)
	revoked, err := m.IsRevoked(t.Context(), mustClaims(t, m, tokenStr))
	require.NoError(t, err)
	assert.False(t, revoked)
}

func mustClaims(t *testing.T, m *token.Manager, tokenStr string) *token.Claims {
	t.Helper()
	claims, err := m.VerifyToken(tokenStr)
	require.NoError(t, err)
	return claims
}
