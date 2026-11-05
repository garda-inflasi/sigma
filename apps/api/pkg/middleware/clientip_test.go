package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitFromEnv(t *testing.T) {
	env := func(v string) func(string) string { return func(string) string { return v } }

	n, err := RateLimitFromEnv(env(""), "K", 600)
	require.NoError(t, err)
	assert.Equal(t, 600, n)

	n, err = RateLimitFromEnv(env(" 1200 "), "K", 600)
	require.NoError(t, err)
	assert.Equal(t, 1200, n)

	for _, buruk := range []string{"0", "-5", "banyak", "1.5"} {
		n, err = RateLimitFromEnv(env(buruk), "K", 600)
		assert.Error(t, err, buruk)
		assert.Equal(t, 600, n, "nilai tidak valid memakai bawaan")
	}
}

func TestTrustedProxiesFromEnv(t *testing.T) {
	assert.Equal(t, DefaultTrustedProxies, TrustedProxiesFromEnv(""))
	assert.Nil(t, TrustedProxiesFromEnv("none"))
	assert.Equal(t, []string{"10.1.0.0/16", "203.0.113.7"}, TrustedProxiesFromEnv(" 10.1.0.0/16, ,203.0.113.7 "))
}

// IP klien diambil dari X-Forwarded-For hanya bila permintaan datang dari proxy tepercaya,
// dan header yang dipalsukan klien tidak menggeser IP yang ditambahkan proxy.
func TestClientIPWithTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(DefaultTrustedProxies))
	r.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	ip := func(remote, xff string) string {
		req := httptest.NewRequest(http.MethodGet, "/ip", nil)
		req.RemoteAddr = remote
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Body.String()
	}

	assert.Equal(t, "198.51.100.9", ip("172.24.0.5:443", "198.51.100.9"), "lewat Caddy: IP klien dari header")
	assert.Equal(t, "198.51.100.9", ip("172.24.0.5:443", "1.2.3.4, 198.51.100.9"), "header palsu dari klien diabaikan")
	assert.Equal(t, "203.0.113.50", ip("203.0.113.50:5000", "1.2.3.4"), "koneksi langsung tak tepercaya: header diabaikan")
}
