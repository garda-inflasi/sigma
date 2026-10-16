package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ── SecurityHeaders ───────────────────────────────────────────────────

func TestSecurityHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler := SecurityHeaders(false)
	handler(c)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "0", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "camera=(), microphone=(), geolocation=()", w.Header().Get("Permissions-Policy"))
	assert.Equal(t, "same-origin", w.Header().Get("Cross-Origin-Resource-Policy"))
	assert.Equal(t, "same-origin", w.Header().Get("Cross-Origin-Opener-Policy"))
	assert.Equal(t, "require-corp", w.Header().Get("Cross-Origin-Embedder-Policy"))
	// HSTS should NOT be set when hsts=false
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_WithHSTS(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler := SecurityHeaders(true)
	handler(c)

	assert.Equal(t, "max-age=63072000; includeSubDomains; preload", w.Header().Get("Strict-Transport-Security"))
}

// ── CSP ───────────────────────────────────────────────────────────────

func TestCSP(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler := CSP()
	handler(c)

	policy := w.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, policy)
	assert.Contains(t, policy, "default-src 'self'")
	assert.Contains(t, policy, "frame-ancestors 'none'")
	assert.Contains(t, policy, "form-action 'self'")
	assert.Contains(t, policy, "object-src 'none'")
}

func TestCSP_WithExtra(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler := CSP("upgrade-insecure-requests")
	handler(c)

	policy := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, policy, "upgrade-insecure-requests")
}

// ── CORS ──────────────────────────────────────────────────────────────

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Origin", "http://localhost:3000")

	handler := CORSMiddleware("http://localhost:3000")
	handler(c)

	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Origin", "http://evil.com")

	handler := CORSMiddleware("http://localhost:3000")
	handler(c)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_WildcardOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Origin", "http://anything.com")

	handler := CORSMiddleware("*")
	handler(c)

	assert.Equal(t, "http://anything.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_NoOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	// No Origin header

	handler := CORSMiddleware("http://localhost:3000")
	handler(c)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_OptionsRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodOptions, "/", nil)
	c.Request.Header.Set("Origin", "http://localhost:3000")

	handler := CORSMiddleware("http://localhost:3000")
	handler(c)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, c.IsAborted())
}

func TestCORSMiddleware_DefaultOrigins(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Origin", "http://localhost:3000")

	handler := CORSMiddleware() // no origins = defaults
	handler(c)

	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}

// ── RequestID ─────────────────────────────────────────────────────────

func TestRequestID_Generated(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler := RequestID()
	handler(c)

	id, exists := c.Get("request_id")
	assert.True(t, exists)
	assert.NotEmpty(t, id)
	assert.Contains(t, id, "req_")
	assert.Equal(t, id, w.Header().Get("X-Request-ID"))
}

func TestRequestID_FromHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("X-Request-ID", "client-provided-id")

	handler := RequestID()
	handler(c)

	id, exists := c.Get("request_id")
	assert.True(t, exists)
	assert.Equal(t, "client-provided-id", id)
	assert.Equal(t, "client-provided-id", w.Header().Get("X-Request-ID"))
}

// ── RateLimit ─────────────────────────────────────────────────────────

func TestRateLimit_Allow(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "192.168.1.1:12345"

	handler := RateLimit(5, time.Minute)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, c.IsAborted())
}

func TestRateLimit_Block(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "192.168.1.2:12345"

	rl := NewRateLimiter(2, time.Minute)
	require.True(t, rl.Allow("192.168.1.2"))
	require.True(t, rl.Allow("192.168.1.2"))
	require.False(t, rl.Allow("192.168.1.2"))
}

func TestRateLimit_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	require.True(t, rl.Allow("192.168.1.1"))
	require.False(t, rl.Allow("192.168.1.1"))
	require.True(t, rl.Allow("192.168.1.2"))
}

func TestRateLimit_WindowReset(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)

	require.True(t, rl.Allow("192.168.1.1"))
	require.False(t, rl.Allow("192.168.1.1"))

	time.Sleep(60 * time.Millisecond)

	require.True(t, rl.Allow("192.168.1.1"))
}

func TestLoginRateLimit(t *testing.T) {
	handler := LoginRateLimit()

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/login", nil)
		c.Request.RemoteAddr = "10.0.0.1:12345"

		handler(c)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should be allowed", i+1)
	}

	// 6th request should be blocked
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/login", nil)
	c.Request.RemoteAddr = "10.0.0.1:12345"

	handler(c)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.True(t, c.IsAborted())
}

// ── JWTAuth ───────────────────────────────────────────────────────────

func TestJWTAuth_ValidToken(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenStr, err := tokenMgr.CreateAccessToken("user-123", "ADMIN_SISTEM", []string{"pengguna.kelola"})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+tokenStr)

	handler := JWTAuth(tokenMgr)
	handler(c)

	assert.False(t, c.IsAborted())
	assert.Equal(t, http.StatusOK, w.Code)

	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	perms, _ := c.Get("permissions")
	jti, _ := c.Get("jti")

	assert.Equal(t, "user-123", userID)
	assert.Equal(t, "ADMIN_SISTEM", role)
	assert.Equal(t, []string{"pengguna.kelola"}, perms)
	assert.NotEmpty(t, jti)
}

func TestJWTAuth_NoToken(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	// No Authorization header

	handler := JWTAuth(tokenMgr)
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_InvalidBearerFormat(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Basic somecreds")

	handler := JWTAuth(tokenMgr)
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!", token.WithAccessTTL(-1*time.Hour))
	tokenStr, err := tokenMgr.CreateAccessToken("user-123", "ADMIN_SISTEM", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+tokenStr)

	handler := JWTAuth(tokenMgr)
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer invalid.jwt.token")

	handler := JWTAuth(tokenMgr)
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_ShortBearer(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bea")

	handler := JWTAuth(tokenMgr)
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ── RequirePermission ────────────────────────────────────────────────

func TestRequirePermission_HasPermission(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", []string{"pengguna.kelola", "master.lihat"})

	handler := RequirePermission("pengguna.kelola")
	handler(c)

	assert.False(t, c.IsAborted())
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_MissingPermission(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", []string{"master.lihat"})

	handler := RequirePermission("pengguna.kelola")
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_NoPermissions(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No permissions set

	handler := RequirePermission("pengguna.kelola")
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_WrongType(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", "not-a-slice") // wrong type

	handler := RequirePermission("pengguna.kelola")
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_EmptyPermissions(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", []string{})

	handler := RequirePermission("pengguna.kelola")
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_MultiplePermissions(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", []string{"harga.lihat", "harga.input", "harga.validasi"})

	handler := RequirePermission("harga.validasi")
	handler(c)

	assert.False(t, c.IsAborted())
	assert.Equal(t, http.StatusOK, w.Code)
}

// ── HasPermission / RequireAnyPermission ──────────────────────────────

func TestHasPermission(t *testing.T) {
	cases := []struct {
		name  string
		perms any // nil = tidak diset di konteks
		want  bool
	}{
		{"izin dimiliki", []string{"harga.lihat", "coldstorage.kelola"}, true},
		{"izin tidak dimiliki", []string{"harga.lihat"}, false},
		{"daftar kosong", []string{}, false},
		{"tidak diset", nil, false},
		{"tipe salah", "coldstorage.kelola", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tc.perms != nil {
				c.Set("permissions", tc.perms)
			}

			assert.Equal(t, tc.want, HasPermission(c, "coldstorage.kelola"))
		})
	}
}

func TestRequireAnyPermission_HoldsOne(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", []string{"logistik.kelola"})

	RequireAnyPermission("coldstorage.kelola", "logistik.kelola")(c)

	assert.False(t, c.IsAborted())
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAnyPermission_HoldsNone(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", []string{"harga.lihat"})

	RequireAnyPermission("coldstorage.kelola", "logistik.kelola")(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Akses ditolak")
}

func TestRequireAnyPermission_NoPermissionsSet(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	RequireAnyPermission("coldstorage.kelola")(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireAnyPermission_NoneRequiredDenies(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("permissions", []string{"harga.lihat"})

	// Tanpa izin yang disyaratkan, middleware gagal tertutup (tolak), bukan lolos.
	RequireAnyPermission()(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ── AccessLog ─────────────────────────────────────────────────────────

func TestAccessLog(t *testing.T) {
	log := logger.NewDefault()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Set("request_id", "req_test")
	c.Status(http.StatusOK)

	handler := AccessLog(log)
	handler(c)

	// Should not panic
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAccessLog_WithUserID(t *testing.T) {
	log := logger.NewDefault()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Set("request_id", "req_test")
	c.Set("user_id", "user-123")
	c.Status(http.StatusOK)

	handler := AccessLog(log)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAccessLog_ServerError(t *testing.T) {
	log := logger.NewDefault()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AccessLog(log))
	router.GET("/error", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	req.Header.Set("X-Request-ID", "req_test")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAccessLog_ClientError(t *testing.T) {
	log := logger.NewDefault()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AccessLog(log))
	router.GET("/notfound", func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	req.Header.Set("X-Request-ID", "req_test")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── Helper: JWT for testing ───────────────────────────────────────────

// createTestToken creates a JWT token for testing purposes
func createTestToken(secret string, claims map[string]interface{}) string {
	jwtClaims := jwt.MapClaims{}
	for k, v := range claims {
		jwtClaims[k] = v
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	s, _ := t.SignedString([]byte(secret))
	return s
}
