package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
)

// JWTAuth returns a Gin middleware that validates JWT access tokens
// and sets user_id, role, and permissions in the Gin context.
//
// The token is read from the `Authorization: Bearer <token>` header first
// (service-to-service and API clients), then from the `garda_access` cookie.
// The cookie fallback is required because the browser never sees the access
// token: the Next.js BFF stores it in an HttpOnly cookie, so direct-to-Go
// calls from the browser (e.g. /api/v1/*) can only authenticate via cookie.
func JWTAuth(tokenMgr *token.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := bearerToken(c)
		if tokenStr == "" {
			tokenStr = accessTokenCookie(c)
		}
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "401",
				"message": "Unauthorized",
			})
			return
		}

		claims, err := tokenMgr.VerifyToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "401",
				"message": "Token tidak valid",
			})
			return
		}

		// Tanda tangan sah belum berarti token masih berlaku: logout memasukkan jti ke daftar cabut dan
		// pencabutan sesi (logout, "Cabut", paksa keluar, reuse refresh token, ganti kata sandi) mematikan
		// sid-nya. Satu query terindeks per request, tanpa cache, supaya pencabutan berlaku seketika
		// di semua replika.
		//
		// Gagal-tutup: bila status pencabutan tidak dapat dipastikan (galat basis data) request ditolak 503,
		// bukan diloloskan. Fail-open akan membuat token yang sudah dicabut hidup lagi tepat saat basis data
		// bermasalah, dan hampir semua handler juga butuh basis data sehingga kehilangan ketersediaannya kecil.
		// 503 (bukan 401) agar klien tidak menyangka sesinya berakhir dan memulai refresh atau logout paksa.
		revoked, err := tokenMgr.IsRevoked(c.Request.Context(), claims)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    "503",
				"message": "Status sesi tidak dapat diperiksa, coba lagi",
			})
			return
		}
		if revoked {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "401",
				"message": SessionRevokedMessage,
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("permissions", claims.Permissions)
		c.Set("jti", claims.ID)
		c.Set(sessionIDKey, claims.SessionID)
		if claims.ExpiresAt != nil {
			c.Set(tokenExpiryKey, claims.ExpiresAt.Time)
		}

		if claims.PasswordChangeRequired && !passwordChangeExempt[c.FullPath()] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    PasswordChangeRequiredCode,
				"message": "Anda harus mengganti kata sandi sebelum melanjutkan",
			})
			return
		}
		if claims.MFASetupRequired && !mfaSetupExempt[c.FullPath()] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    MFASetupRequiredCode,
				"message": "Anda harus mengaktifkan autentikasi dua langkah (MFA) sebelum melanjutkan",
			})
			return
		}
		c.Next()
	}
}

// PasswordChangeRequiredCode adalah kode galat 403 untuk akun yang wajib mengganti kata sandi.
const PasswordChangeRequiredCode = "PASSWORD_CHANGE_REQUIRED"

// passwordChangeExempt adalah satu-satunya rute yang terbuka bagi akun yang wajib
// mengganti kata sandi: mengganti, keluar, dan membaca profil sendiri.
var passwordChangeExempt = map[string]bool{
	"/v1/auth/change-password": true,
	"/v1/auth/logout":          true,
	"/v1/auth/me":              true,
}

// MFASetupRequiredCode adalah kode galat 403 untuk akun yang wajib mengaktifkan MFA tetapi belum.
const MFASetupRequiredCode = "MFA_SETUP_REQUIRED"

// mfaSetupExempt adalah satu-satunya rute yang terbuka bagi akun yang wajib setup MFA: memulai dan
// memverifikasi setup, mengganti kata sandi (bila juga diwajibkan), keluar, dan membaca profil sendiri.
var mfaSetupExempt = map[string]bool{
	"/v1/auth/mfa/setup":       true,
	"/v1/auth/mfa/verify":      true,
	"/v1/auth/change-password": true,
	"/v1/auth/logout":          true,
	"/v1/auth/me":              true,
}

// bearerToken extracts the token from an `Authorization: Bearer <token>` header.
func bearerToken(c *gin.Context) string {
	bearer := c.GetHeader("Authorization")
	if len(bearer) < 7 || !strings.EqualFold(bearer[:7], "Bearer ") {
		return ""
	}
	return bearer[7:]
}

// accessTokenCookie extracts the access token from the BFF-issued cookie.
// The cookie name mirrors the frontend (see apps/web/src/lib/bff/cookies.ts):
// `garda_access` in development, `__Host-access` in production.
func accessTokenCookie(c *gin.Context) string {
	for _, name := range []string{"garda_access", "__Host-access"} {
		if v, err := c.Cookie(name); err == nil && v != "" {
			return v
		}
	}
	return ""
}

// HasPermission reports whether the authenticated user holds the given
// permission. Must be used after JWTAuth middleware; it is false when no
// permissions were set on the context.
func HasPermission(c *gin.Context, permission string) bool {
	perms, exists := c.Get("permissions")
	if !exists {
		return false
	}

	permList, ok := perms.([]string)
	if !ok {
		return false
	}

	return slices.Contains(permList, permission)
}

// RequirePermission returns a Gin middleware that checks if the authenticated
// user has the specified permission. Must be used after JWTAuth middleware.
func RequirePermission(permission string) gin.HandlerFunc {
	return RequireAnyPermission(permission)
}

// RequireAnyPermission returns a Gin middleware that allows the request when
// the authenticated user holds at least one of the given permissions. Use it
// for routes whose finer-grained check happens in the handler (e.g. an action
// that needs a different permission per payload). Must be used after JWTAuth.
func RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, p := range permissions {
			if HasPermission(c, p) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    "403",
			"message": "Akses ditolak",
		})
	}
}

// AccessLog logs every request with its duration, status, and request ID.
func AccessLog(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Log after response
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		ip := c.ClientIP()
		reqID, _ := c.Get("request_id")

		fields := map[string]interface{}{
			"method":     method,
			"path":       path,
			"status":     status,
			"ip":         ip,
			"request_id": reqID,
		}

		if userID, exists := c.Get("user_id"); exists {
			fields["user_id"] = userID
		}

		if status >= 500 {
			log.Error("server error", fields)
		} else if status >= 400 {
			log.Warn("client error", fields)
		} else {
			log.Info("request", fields)
		}
	}
}
