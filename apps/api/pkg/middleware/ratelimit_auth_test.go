package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tes AuthRateLimit: pembatas per IP klien untuk endpoint auth publik di Go. Kontrak 429 sama dengan batas
// laju BFF (kode RATE_LIMITED, header Retry-After, data.sisa_detik) supaya BFF meneruskannya tanpa diubah.

func routerBatasAuth(t *testing.T, limit int, window time.Duration, proxies []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(proxies))
	r.POST("/login", AuthRateLimit(limit, window), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func kirimDari(r *gin.Engine, remote, xff string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthRateLimit_KontrakJawaban429(t *testing.T) {
	r := routerBatasAuth(t, 2, time.Minute, nil)
	for i := 1; i <= 2; i++ {
		require.Equal(t, http.StatusOK, kirimDari(r, "203.0.113.5:4000", "").Code, "permintaan %d", i)
	}

	w := kirimDari(r, "203.0.113.5:4001", "")

	require.Equal(t, http.StatusTooManyRequests, w.Code)
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    struct {
			SisaDetik int `json:"sisa_detik"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	assert.Equal(t, KodeBatasLaju, body.Code)
	assert.Equal(t, "RATE_LIMITED", body.Code, "kode yang sama dengan batas laju BFF")
	assert.NotEmpty(t, body.Message)
	assert.GreaterOrEqual(t, body.Data.SisaDetik, 59, "jendela satu menit baru berjalan")
	assert.LessOrEqual(t, body.Data.SisaDetik, 60)
	assert.Equal(t, strconv.Itoa(body.Data.SisaDetik), w.Header().Get("Retry-After"), "header dan badan memuat detik yang sama")
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestAuthRateLimit_PermintaanDitolakTidakSampaiKeHandlerDanTidakMemperpanjang(t *testing.T) {
	panggilan := 0
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", AuthRateLimit(1, time.Minute), func(c *gin.Context) { panggilan++; c.Status(http.StatusOK) })
	require.Equal(t, http.StatusOK, kirimDari(r, "203.0.113.5:1", "").Code)
	pertama := kirimDari(r, "203.0.113.5:1", "")
	require.Equal(t, http.StatusTooManyRequests, pertama.Code)
	detik1, err := strconv.Atoi(pertama.Header().Get("Retry-After"))
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		require.Equal(t, http.StatusTooManyRequests, kirimDari(r, "203.0.113.5:1", "").Code)
	}

	assert.Equal(t, 1, panggilan, "handler hanya dijalankan untuk permintaan yang lolos")
	akhir := kirimDari(r, "203.0.113.5:1", "")
	detik2, err := strconv.Atoi(akhir.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.LessOrEqual(t, detik2, detik1, "permintaan yang ditolak tidak mendorong jendela")
}

func TestAuthRateLimit_JendelaBerlalu(t *testing.T) {
	r := routerBatasAuth(t, 1, 150*time.Millisecond, nil)
	require.Equal(t, http.StatusOK, kirimDari(r, "203.0.113.5:1", "").Code)
	w := kirimDari(r, "203.0.113.5:1", "")
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "1", w.Header().Get("Retry-After"), "sisa kurang dari satu detik dibulatkan ke atas, minimal 1")

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, http.StatusOK, kirimDari(r, "203.0.113.5:1", "").Code)
}

func TestAuthRateLimit_PerIPKlien(t *testing.T) {
	r := routerBatasAuth(t, 1, time.Minute, nil)
	require.Equal(t, http.StatusOK, kirimDari(r, "203.0.113.5:1", "").Code)
	require.Equal(t, http.StatusTooManyRequests, kirimDari(r, "203.0.113.5:2", "").Code)

	assert.Equal(t, http.StatusOK, kirimDari(r, "203.0.113.6:1", "").Code, "IP lain punya ember sendiri")
}

func TestAuthRateLimit_IPKlienDariProxyTepercaya(t *testing.T) {
	// BFF/Caddy (10.0.0.9) meneruskan IP peramban di X-Forwarded-For: kuncinya IP itu, bukan alamat proxy.
	r := routerBatasAuth(t, 1, time.Minute, []string{"10.0.0.0/8"})
	require.Equal(t, http.StatusOK, kirimDari(r, "10.0.0.9:5000", "198.51.100.7").Code)

	assert.Equal(t, http.StatusTooManyRequests, kirimDari(r, "10.0.0.9:5001", "198.51.100.7").Code, "klien yang sama lewat proxy yang sama")
	assert.Equal(t, http.StatusOK, kirimDari(r, "10.0.0.9:5002", "198.51.100.8").Code, "klien lain di balik proxy yang sama tidak ikut terhitung")
}

func TestAuthRateLimit_XForwardedForPalsuDariKlienLangsungTidakMembukaEmberBaru(t *testing.T) {
	r := routerBatasAuth(t, 1, time.Minute, []string{"10.0.0.0/8"})
	require.Equal(t, http.StatusOK, kirimDari(r, "203.0.113.5:1", "").Code)

	// Klien langsung (bukan proxy tepercaya) mengarang X-Forwarded-For: diabaikan, kuncinya tetap alamat koneksi.
	for _, palsu := range []string{"1.1.1.1", "2.2.2.2", "9.9.9.9, 8.8.8.8"} {
		assert.Equal(t, http.StatusTooManyRequests, kirimDari(r, "203.0.113.5:1", palsu).Code, palsu)
	}
}

func TestAuthRateLimit_InstansBerbedaMemilikiEmberTerpisah(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	satu, dua := AuthRateLimit(1, time.Minute), AuthRateLimit(1, time.Minute)
	r.POST("/login", satu, ok)
	r.POST("/forgot-password", satu, ok)
	r.POST("/sso", dua, ok)
	panggil := func(path string) int {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "203.0.113.5:1"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	require.Equal(t, http.StatusOK, panggil("/login"))
	assert.Equal(t, http.StatusTooManyRequests, panggil("/forgot-password"), "instans yang sama dipakai bersama: satu ember")
	assert.Equal(t, http.StatusOK, panggil("/sso"), "instans lain: ember terpisah")
}

func TestRateLimiter_AllowTetapBerperilakuSama(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	assert.True(t, rl.Allow("a"))
	assert.True(t, rl.Allow("a"))
	assert.False(t, rl.Allow("a"))
	ok, tunggu := rl.allow("a")
	assert.False(t, ok)
	assert.Greater(t, tunggu, 59*time.Second)
	ok, tunggu = rl.allow("b")
	assert.True(t, ok)
	assert.Zero(t, tunggu)
}
