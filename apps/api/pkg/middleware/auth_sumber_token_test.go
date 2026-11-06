package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/pkg/token"
)

// Tes JWTAuth untuk sumber token (header atau cookie BFF), urutan pemeriksaan pencabutan terhadap penegakan
// kewajiban ganti kata sandi / setup MFA, rute sesi yang baru, dan batas laju di belakang proxy.

func tokenSah(t *testing.T, m *token.Manager, opts ...token.ClaimOption) string {
	t.Helper()
	s, err := m.CreateAccessToken(uid("user-1"), "ADMIN_SISTEM", []string{"pengguna.kelola"}, opts...)
	require.NoError(t, err)
	return s
}

func rute(m *token.Manager, paths ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/v1")
	g.Use(JWTAuth(m))
	for _, p := range paths {
		g.Any(p, func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"user": c.GetString("user_id"), "sid": SessionID(c)})
		})
	}
	return r
}

func kirim(r *gin.Engine, method, path string, headers map[string]string, cookies map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJWTAuth_TokenDariCookieBFF(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	r := rute(m, "/data")
	tk := tokenSah(t, m)

	for _, nama := range []string{"garda_access", "__Host-access"} {
		w := kirim(r, http.MethodGet, "/v1/data", nil, map[string]string{nama: tk})
		assert.Equal(t, http.StatusOK, w.Code, nama)
	}
	assert.Equal(t, http.StatusUnauthorized, kirim(r, http.MethodGet, "/v1/data", nil, map[string]string{"garda_access": ""}).Code,
		"cookie kosong sama dengan tanpa token")
	assert.Equal(t, http.StatusUnauthorized, kirim(r, http.MethodGet, "/v1/data", nil, map[string]string{"access": tk}).Code,
		"hanya nama cookie yang dikenal BFF yang dibaca")
}

func TestJWTAuth_HeaderBearerMengalahkanCookieDanTidakJatuhKeCookie(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	r := rute(m, "/data")
	tk := tokenSah(t, m)

	// Bearer sah, cookie sampah: bearer yang dipakai.
	w := kirim(r, http.MethodGet, "/v1/data", map[string]string{"Authorization": "Bearer " + tk}, map[string]string{"garda_access": "sampah"})
	assert.Equal(t, http.StatusOK, w.Code)

	// Bearer sampah, cookie sah: ditolak. Jatuh ke cookie akan membiarkan klien yang salah memilih sumber
	// token tanpa sadar dan menyamarkan token bearer yang rusak.
	w = kirim(r, http.MethodGet, "/v1/data", map[string]string{"Authorization": "Bearer sampah"}, map[string]string{"garda_access": tk})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Skema selain Bearer diabaikan, lalu cookie dipakai.
	w = kirim(r, http.MethodGet, "/v1/data", map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}, map[string]string{"garda_access": tk})
	assert.Equal(t, http.StatusOK, w.Code)

	// Skema Bearer tidak peka huruf besar-kecil (RFC 7235).
	w = kirim(r, http.MethodGet, "/v1/data", map[string]string{"Authorization": "bearer " + tk}, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJWTAuth_PencabutanDiperiksaSebelumKewajibanGantiKataSandi(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	checker := &fakeChecker{revokedSID: map[string]bool{sidUji: true}}
	m.SetRevocationChecker(checker)
	r := rute(m, "/data", "/auth/me")
	// Token dicabut yang juga membawa kewajiban: harus 401 (sesi berakhir), bukan 403 yang menyiratkan masih sah.
	tk := tokenSah(t, m, token.WithSession(sidUji), token.WithPasswordChangeRequired(true), token.WithMFASetupRequired(true))

	for _, path := range []string{"/v1/data", "/v1/auth/me"} {
		w := kirim(r, http.MethodGet, path, map[string]string{"Authorization": "Bearer " + tk}, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code, path)
		assert.Contains(t, w.Body.String(), SessionRevokedMessage)
	}
}

func TestJWTAuth_PencabutanBerlakuJugaUntukTokenDariCookie(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	checker := &fakeChecker{revokedSID: map[string]bool{sidUji: true}}
	m.SetRevocationChecker(checker)
	r := rute(m, "/data")
	tk := tokenSah(t, m, token.WithSession(sidUji))

	w := kirim(r, http.MethodGet, "/v1/data", nil, map[string]string{"garda_access": tk})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.EqualValues(t, 1, checker.calls.Load())
}

// Hanya me, logout, dan change-password (serta setup MFA) yang terbuka bagi sesi yang wajib memperbaiki akunnya.
// Rute sesi dari 01-E tidak boleh ikut terbuka: sesi setengah-jadi tidak boleh melihat atau mencabut sesi lain.
func TestJWTAuth_RuteSesiBaruTertutupBagiAkunYangWajibMemperbaikiAkun(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	r := rute(m, "/auth/sessions", "/auth/sessions/:id", "/auth/sessions/revoke-others", "/auth/logout-all", "/users/:id/revoke-sessions", "/auth/logout", "/auth/me")

	kasus := map[string][]token.ClaimOption{
		"wajib ganti kata sandi": {token.WithPasswordChangeRequired(true)},
		"wajib setup MFA":        {token.WithMFASetupRequired(true)},
	}
	for nama, opts := range kasus {
		t.Run(nama, func(t *testing.T) {
			tk := tokenSah(t, m, opts...)
			bearer := map[string]string{"Authorization": "Bearer " + tk}
			for _, tertutup := range []struct{ method, path string }{
				{http.MethodGet, "/v1/auth/sessions"},
				{http.MethodDelete, "/v1/auth/sessions/3f1c8a52-0a57-4b9a-8f0e-0d1f6f3c9a11"},
				{http.MethodPost, "/v1/auth/sessions/revoke-others"},
				{http.MethodPost, "/v1/auth/logout-all"},
				{http.MethodPost, "/v1/users/" + uid("u1") + "/revoke-sessions"},
			} {
				w := kirim(r, tertutup.method, tertutup.path, bearer, nil)
				assert.Equal(t, http.StatusForbidden, w.Code, tertutup.method+" "+tertutup.path)
			}
			for _, terbuka := range []struct{ method, path string }{
				{http.MethodPost, "/v1/auth/logout"},
				{http.MethodGet, "/v1/auth/me"},
			} {
				assert.Equal(t, http.StatusOK, kirim(r, terbuka.method, terbuka.path, bearer, nil).Code, terbuka.path)
			}
		})
	}
}

// Pengecualian dicocokkan dengan pola rute, bukan URL mentah: query atau segmen tambahan tidak membukanya.
func TestJWTAuth_PengecualianDicocokkanDenganPolaRuteBukanUrlMentah(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	r := rute(m, "/data", "/auth/me")
	bearer := map[string]string{"Authorization": "Bearer " + tokenSah(t, m, token.WithPasswordChangeRequired(true))}

	assert.Equal(t, http.StatusForbidden, kirim(r, http.MethodGet, "/v1/data?x=/v1/auth/me", bearer, nil).Code)
	assert.Equal(t, http.StatusNotFound, kirim(r, http.MethodGet, "/v1/data;/v1/auth/me", bearer, nil).Code,
		"segmen tambahan bukan rute /v1/auth/me")
	assert.Equal(t, http.StatusOK, kirim(r, http.MethodGet, "/v1/auth/me?x=1", bearer, nil).Code)
}

// ── Batas laju ────────────────────────────────────────────────────────

func TestRateLimiter_SerentakTidakMelewatiBatas(t *testing.T) {
	rl := NewRateLimiter(50, time.Minute)
	var diizinkan atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 400; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow("203.0.113.9") {
				diizinkan.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.EqualValues(t, 50, diizinkan.Load(), "tepat sebanyak batas, tidak lebih walau serentak")
}

func TestRateLimiter_PermintaanDitolakTidakMemperpanjangJendela(t *testing.T) {
	rl := NewRateLimiter(2, 150*time.Millisecond)
	require.True(t, rl.Allow("ip"))
	require.True(t, rl.Allow("ip"))
	for i := 0; i < 20; i++ {
		require.False(t, rl.Allow("ip"))
	}

	time.Sleep(200 * time.Millisecond)

	assert.True(t, rl.Allow("ip"), "penolakan beruntun tidak dicatat, jadi jendela habis tepat waktu")
}

func rateRouter(t *testing.T, limit int, trusted []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(trusted))
	r.Use(RateLimit(limit, time.Minute))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return r
}

func hit(r *gin.Engine, remote, xff string) int {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// Klien langsung (bukan lewat proxy tepercaya) tidak boleh mengelak dari batas dengan mengganti X-Forwarded-For.
func TestRateLimit_XForwardedForPalsuDariKlienLangsungDiabaikan(t *testing.T) {
	r := rateRouter(t, 3, DefaultTrustedProxies)

	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusNoContent, hit(r, "198.51.100.7:4000", "10.9.9."+string(rune('1'+i))))
	}

	assert.Equal(t, http.StatusTooManyRequests, hit(r, "198.51.100.7:4001", "1.2.3.4"),
		"IP palsu di header tidak membuka ember baru")
}

// Di belakang proxy tepercaya setiap klien asli punya ember sendiri, dan entri palsu di kiri rantai tidak dipercaya.
func TestRateLimit_DiBelakangProxyEmberPerKlienAsli(t *testing.T) {
	r := rateRouter(t, 2, DefaultTrustedProxies)
	proxy := "172.18.0.5:5000"

	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusNoContent, hit(r, proxy, "203.0.113.10"))
	}
	assert.Equal(t, http.StatusTooManyRequests, hit(r, proxy, "203.0.113.10"))
	assert.Equal(t, http.StatusNoContent, hit(r, proxy, "203.0.113.11"), "klien lain di belakang proxy yang sama tidak ikut terbatasi")

	// Klien menambahkan entri palsu di kiri; proxy menambahkan IP aslinya di kanan.
	assert.Equal(t, http.StatusTooManyRequests, hit(r, proxy, "9.9.9.9, 203.0.113.10"),
		"yang dihitung adalah IP yang dilihat proxy, bukan klaim klien")
}

func TestRateLimit_ResponsMenyebutKodeDanPesan(t *testing.T) {
	r := rateRouter(t, 1, DefaultTrustedProxies)
	require.Equal(t, http.StatusNoContent, hit(r, "198.51.100.8:1", ""))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "198.51.100.8:2"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.JSONEq(t, `{"code":"429","message":"Terlalu banyak permintaan. Coba lagi nanti."}`, w.Body.String())
}
