package middleware

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

const csrfHeader = "X-CSRF-Token"

// csrfCookieNames mengikuti apps/web/src/lib/bff/csrf.ts: nama sederhana saat
// cookie tidak Secure, berawalan __Host- saat Secure.
var csrfCookieNames = []string{"garda_csrf", "__Host-csrf"}

// ParseOrigins memecah daftar origin berpisah koma (APP_ALLOWED_ORIGINS) dan
// membuang entri kosong. Hasil kosong diganti fallback.
func ParseOrigins(csv string, fallback ...string) []string {
	var origins []string
	for _, entry := range strings.Split(csv, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			origins = append(origins, entry)
		}
	}
	if len(origins) == 0 {
		return fallback
	}
	return origins
}

// CSRFProtection menolak request pengubah data yang diautentikasi lewat cookie
// bila tidak terbukti berasal dari aplikasi sendiri.
//
// Cookie dikirim browser otomatis, sehingga situs lain dapat memicu request
// atas nama pengguna. Pembuktiannya: header Origin (atau Referer) berasal dari
// allowedOrigins atau host yang sama dengan request, atau header X-CSRF-Token
// sama dengan cookie CSRF (double-submit). Request ber-Bearer tidak terdampak
// karena token tidak dikirim otomatis oleh browser.
func CSRFProtection(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[normalizeOrigin(origin)] = true
	}

	return func(c *gin.Context) {
		if !needsCSRFCheck(c) || originTrusted(c, allowed) || csrfTokenValid(c) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    "403",
			"message": "Permintaan lintas situs ditolak",
		})
	}
}

// needsCSRFCheck: hanya method pengubah data yang diautentikasi dengan cookie.
func needsCSRFCheck(c *gin.Context) bool {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	}
	return bearerToken(c) == "" && accessTokenCookie(c) != ""
}

func originTrusted(c *gin.Context, allowed map[string]bool) bool {
	source := c.GetHeader("Origin")
	if source == "" {
		source = c.GetHeader("Referer")
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}

	if allowed[normalizeOrigin(parsed.Scheme+"://"+parsed.Host)] {
		return true
	}
	// Sama host dengan yang dituju browser (Caddy meneruskan Host asli).
	return parsed.Host == c.Request.Host || parsed.Host == c.GetHeader("X-Forwarded-Host")
}

func csrfTokenValid(c *gin.Context) bool {
	header := c.GetHeader(csrfHeader)
	if header == "" {
		return false
	}
	for _, name := range csrfCookieNames {
		cookie, err := c.Cookie(name)
		if err == nil && cookie != "" && subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) == 1 {
			return true
		}
	}
	return false
}

// normalizeOrigin menyeragamkan huruf dan garis miring akhir.
func normalizeOrigin(origin string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(origin)), "/")
}
