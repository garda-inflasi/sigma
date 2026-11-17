package middleware

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/garda-inflasi/api/pkg/idx"
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

		// pengguna.id adalah UUID (000110). Token yang terbit sebelum migrasi itu membawa slug lama sebagai sub;
		// kolom uuid menolak teks bebas (22P02 => 500 di pemeriksaan pencabutan dan izin di bawah), jadi ditolak
		// bersih di sini. Jawabannya sama dengan izin yang basi: klien web menyegarkan sesi dengan refresh token
		// (yang sudah dimigrasi) dan mendapat token ber-UUID tanpa pengguna harus masuk ulang.
		userID, ok := idx.NormalUUID(claims.UserID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    PermissionsChangedCode,
				"message": PermissionsChangedMessage,
			})
			return
		}
		claims.UserID = userID

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

		// Izin yang ditegakkan berasal dari basis data (resolver berbatas cache pendek), bukan dari klaim token:
		// izin yang dicabut, izin langsung yang kedaluwarsa, dan perubahan matriks peran harus berlaku dalam
		// hitungan detik, bukan menunggu token 15 menit habis. Tanpa resolver (router tanpa modul auth) klaim dipakai.
		//
		// Gagal-tutup: izin yang tidak dapat dipastikan menolak request dengan 503 (bukan memakai klaim yang
		// mungkin sudah dicabut). Akun yang sudah tidak ada atau tidak aktif dijawab 401 seperti sesi dicabut.
		perms, resolved, err := tokenMgr.ResolvePermissions(c.Request.Context(), claims)
		if err != nil {
			// Sesi impersonasi yang berakhir (dihentikan, kedaluwarsa, kebijakan dinonaktifkan, pelaku atau sasaran tidak
			// lagi berhak) dijawab 401 berkode khusus: klien web mencoba refresh, refresh ditolak, lalu masuk ulang.
			if errors.Is(err, token.ErrImpersonationEnded) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code":    ImpersonationEndedCode,
					"message": ImpersonationEndedMessage,
				})
				return
			}
			if errors.Is(err, token.ErrPrincipalNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code":    "401",
					"message": SessionRevokedMessage,
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    "503",
				"message": "Izin akun tidak dapat diperiksa, coba lagi",
			})
			return
		}
		if resolved {
			// Token yang izinnya sudah berbeda dari kenyataan ditolak dengan kode khusus: klien web menjawabnya dengan
			// refresh senyap (01-E) dan token baru langsung membawa izin yang benar, sehingga menu, tombol, dan
			// penjaga rute di UI ikut menyusul tanpa pengguna harus masuk ulang.
			if !token.SamePermissions(perms, claims.Permissions) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code":    PermissionsChangedCode,
					"message": PermissionsChangedMessage,
				})
				return
			}
		} else {
			perms = claims.Permissions
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("permissions", perms)
		c.Set("jti", claims.ID)
		c.Set(sessionIDKey, claims.SessionID)
		if claims.ExpiresAt != nil {
			c.Set(tokenExpiryKey, claims.ExpiresAt.Time)
		}

		if claims.PasswordChangeRequired && !terbukaBagiSesiTerbatas(c, passwordChangeExempt) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    PasswordChangeRequiredCode,
				"message": "Anda harus mengganti kata sandi sebelum melanjutkan",
			})
			return
		}
		if claims.MFASetupRequired && !terbukaBagiSesiTerbatas(c, mfaSetupExempt) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    MFASetupRequiredCode,
				"message": "Anda harus mengaktifkan autentikasi dua langkah (MFA) sebelum melanjutkan",
			})
			return
		}

		if claims.Imp != nil {
			ic := token.ImpersonationContext{Impersonation: *claims.Imp, EffectiveUserID: claims.UserID, EffectiveRole: claims.Role}
			c.Set(impersonationKey, ic)
			// Context request ikut membawa impersonasi: audit.Writer dan usecase memakainya untuk menautkan setiap aksi
			// ke pelaku asli tanpa semua modul mengubah kode mereka.
			c.Request = c.Request.WithContext(token.ContextWithImpersonation(c.Request.Context(), ic))
			if impersonasiDilarang(c.Request.Method, c.FullPath()) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"code":    ImpersonationRestrictedCode,
					"message": ImpersonationRestrictedMessage,
				})
			} else {
				c.Next()
			}
			// Jaring pengaman akuntabilitas: setiap mutasi selama impersonasi tercatat, termasuk yang ditolak, walau
			// modul tujuannya tidak menulis audit sendiri.
			// Mulai/berhenti/ganti sudah punya entri audit sendiri (IMPERSONASI_*), jadi tidak dicatat dua kali.
			if mutasi(c.Request.Method) && !strings.HasPrefix(c.FullPath(), "/v1/auth/impersonation/") {
				if guard := tokenMgr.ImpersonationGuard(); guard != nil {
					reqID, _ := c.Get("request_id")
					id, _ := reqID.(string)
					guard.RecordImpersonationMutation(c.Request.Context(), ic, token.MutationInfo{
						Method: c.Request.Method, Route: c.FullPath(), Status: c.Writer.Status(),
						IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), RequestID: id,
					})
				}
			}
			return
		}
		c.Next()
	}
}

// impersonationKey adalah kunci context Gin untuk token.ImpersonationContext (hanya ada selama impersonasi).
const impersonationKey = "impersonation"

// Impersonation mengembalikan keadaan impersonasi request ini; ok false bila bukan sesi impersonasi. Hanya terisi
// setelah JWTAuth.
func Impersonation(c *gin.Context) (token.ImpersonationContext, bool) {
	v, ok := c.Get(impersonationKey)
	if !ok {
		return token.ImpersonationContext{}, false
	}
	ic, ok := v.(token.ImpersonationContext)
	return ic, ok
}

// ImpersonationEndedCode adalah kode galat 401 untuk token impersonasi yang sesinya sudah tidak sah.
const ImpersonationEndedCode = "IMPERSONATION_ENDED"

// ImpersonationEndedMessage dipakai klien sebagai pesan saat sesi impersonasi berakhir.
const ImpersonationEndedMessage = "Sesi impersonasi telah berakhir, silakan masuk kembali"

// ImpersonationRestrictedCode adalah kode galat 403 untuk endpoint yang dilarang selama impersonasi.
const ImpersonationRestrictedCode = "IMPERSONATION_RESTRICTED"

// ImpersonationRestrictedMessage menjelaskan larangan itu.
const ImpersonationRestrictedMessage = "Aksi ini tidak tersedia selama impersonasi. Kembali ke akun asli terlebih dahulu"

// mutasi true untuk metode HTTP yang mengubah data.
func mutasi(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// impersonasiDilarang melaporkan apakah request ini dilarang selama impersonasi: pelaku tidak boleh mengubah kredensial
// (kata sandi, MFA, sesi, profil) maupun hak (pengguna, peran, matriks izin, pengaturan impersonasi) akun sasaran atau
// siapa pun lewat sesi pinjaman. Membaca tetap boleh, dan `/auth/impersonation/*` (berhenti atau ganti sasaran) bukan
// bagian larangan ini.
func impersonasiDilarang(method, fullPath string) bool {
	switch fullPath {
	case "/v1/auth/change-password", "/v1/auth/logout-all", "/v1/auth/sessions/revoke-others", "/v1/auth/sessions/:id":
		return true
	case "/v1/auth/me":
		return method == http.MethodPatch
	}
	if strings.HasPrefix(fullPath, "/v1/auth/mfa/") || fullPath == "/v1/auth/mfa" {
		return true
	}
	if strings.HasPrefix(fullPath, "/v1/admin/impersonation") {
		return true
	}
	if strings.HasPrefix(fullPath, "/v1/users") || strings.HasPrefix(fullPath, "/v1/roles") {
		return mutasi(method)
	}
	return false
}

// RequireRole menolak (403) request yang peran tokennya tidak termasuk roles. Dipakai berdampingan dengan
// RequirePermission untuk izin yang eksklusif milik peran tertentu (mis. impersonasi.kelola milik ADMIN_SISTEM).
// Harus dipasang setelah JWTAuth.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if slices.Contains(roles, c.GetString("role")) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    "403",
			"message": "Akses ditolak",
		})
	}
}

// PermissionsChangedCode adalah kode galat 401 untuk access token yang salinan izinnya sudah tidak sama dengan
// izin efektif akunnya (izin langsung diberi/dicabut/kedaluwarsa, atau matriks peran diubah).
const PermissionsChangedCode = "PERMISSIONS_CHANGED"

// PermissionsChangedMessage sengaja tidak memuat kata "password"/"kata sandi": klien web memperlakukan 401
// semacam itu sebagai salah kredensial dan tidak mencoba refresh.
const PermissionsChangedMessage = "Izin akun berubah, sesi perlu disegarkan"

// PasswordChangeRequiredCode adalah kode galat 403 untuk akun yang wajib mengganti kata sandi.
const PasswordChangeRequiredCode = "PASSWORD_CHANGE_REQUIRED"

// passwordChangeExempt adalah satu-satunya rute yang terbuka bagi akun yang wajib
// mengganti kata sandi: mengganti, keluar, dan membaca (bukan mengubah) profil sendiri.
var passwordChangeExempt = map[string]bool{
	"/v1/auth/change-password": true,
	"/v1/auth/logout":          true,
	"/v1/auth/me":              true,
}

// MFASetupRequiredCode adalah kode galat 403 untuk akun yang wajib mengaktifkan MFA tetapi belum.
const MFASetupRequiredCode = "MFA_SETUP_REQUIRED"

// mfaSetupExempt adalah satu-satunya rute yang terbuka bagi akun yang wajib setup MFA: memulai dan
// memverifikasi setup, mengganti kata sandi (bila juga diwajibkan), keluar, dan membaca (bukan mengubah) profil sendiri.
var mfaSetupExempt = map[string]bool{
	"/v1/auth/mfa/setup":       true,
	"/v1/auth/mfa/verify":      true,
	"/v1/auth/change-password": true,
	"/v1/auth/logout":          true,
	"/v1/auth/me":              true,
}

// profilSendiriPath adalah profil pemilik akun. Sesi terbatas (wajib ganti kata sandi atau setup MFA) hanya boleh
// MEMBACA-nya (GET) untuk mengetahui keadaan akun; mengubahnya (PATCH, 20-A) menunggu sampai kewajiban itu selesai,
// karena sesi terbatas hanya boleh menuntaskan kewajibannya, bukan mengubah data akun.
const profilSendiriPath = "/v1/auth/me"

// terbukaBagiSesiTerbatas melaporkan apakah request ini boleh lewat dengan token yang masih terbatas: rute harus ada
// di daftar terbuka, dan untuk profil sendiri hanya metode GET.
func terbukaBagiSesiTerbatas(c *gin.Context, terbuka map[string]bool) bool {
	jalur := c.FullPath()
	if !terbuka[jalur] {
		return false
	}
	return jalur != profilSendiriPath || c.Request.Method == http.MethodGet
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
