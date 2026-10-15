package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds security-related HTTP headers to every response.
// If hsts is true, Strict-Transport-Security is included (only in production).
func SecurityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "0")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Embedder-Policy", "require-corp")

		if hsts {
			c.Header("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}

		c.Next()
	}
}

// CSP returns a Content-Security-Policy middleware.
func CSP(extras ...string) gin.HandlerFunc {
	policy := "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob: https://*.tile.openstreetmap.org https://api.mapbox.com; " +
		"font-src 'self' data:; " +
		"connect-src 'self' https://api.mapbox.com; " +
		"frame-ancestors 'none'; " +
		"form-action 'self'; " +
		"base-uri 'self'; " +
		"object-src 'none'"

	for _, extra := range extras {
		policy += "; " + extra
	}

	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", policy)
		c.Next()
	}
}

// CORSMiddleware configures CORS for the BFF-to-API communication.
// If no origins are provided, defaults to allowing localhost:3000 (Next.js dev).
func CORSMiddleware(allowedOrigins ...string) gin.HandlerFunc {
	origins := allowedOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000", "https://localhost:3000"}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		allowed := false
		for _, o := range origins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Authorization, X-CSRF-Token, Idempotency-Key, X-Request-ID")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatusJSON(http.StatusNoContent, nil)
			return
		}

		c.Next()
	}
}

// RequestID adds a unique request ID to every request.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = generateID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func generateID() string {
	return "req_" + time.Now().Format("150405.000000")
}
