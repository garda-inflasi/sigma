package routes

import (
	"os"
	"strconv"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/delivery/http"
	"github.com/garda-inflasi/api/internal/auth/repository/postgres"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/crypto"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/sso"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Token  *token.Manager
	Logger *logger.Logger
	// AuthRateLimitPerMenit membatasi percobaan per menit per IP klien ke endpoint auth publik (login, validasi
	// OTP, lupa dan atur ulang kata sandi, mulai dan selesai SSO). 0 atau negatif berarti tanpa pembatas: itu
	// nilai tes yang membuat puluhan login per paket. cmd/api mengisinya dari AUTH_RATE_LIMIT_PER_MENIT
	// (bawaan middleware.DefaultAuthRateLimit).
	AuthRateLimitPerMenit int
}

type Options struct {
	UseCaseConfig usecase.Config
	// SSO menggantikan konfigurasi SSO Google dari lingkungan (tes); nil berarti SSO nonaktif.
	SSO *sso.Config
}

func Register(engine *gin.RouterGroup, deps *Dependencies, opts ...Options) {
	var cfg usecase.Config
	if len(opts) > 0 {
		cfg = opts[0].UseCaseConfig
	} else {
		cfg = ConfigFromEnv(os.Getenv)
	}

	// Setup email sender from environment
	emailPort, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if emailPort == 0 {
		emailPort = 587
	}
	emailCfg := email.Config{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     emailPort,
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
		FromName: "Garda Inflasi",
		BaseURL:  os.Getenv("APP_PUBLIC_URL"),
	}
	if emailCfg.From == "" {
		emailCfg.From = "noreply@gardainflasi.id"
	}
	emailSender := email.New(emailCfg)

	// Penegakan pencabutan access token (logout, sesi dicabut, paksa keluar): JWTAuth(tokenMgr) di semua modul
	// menanyakannya lewat Manager yang sama, sehingga memasang modul auth di router mana pun otomatis
	// menyalakannya tanpa mengubah tanda tangan JWTAuth.
	deps.Token.SetRevocationChecker(postgres.NewRevocationChecker(deps.DB))

	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, deps.Token, cfg, deps.Logger, emailSender, audit.NewWriter(deps.DB))
	// Izin efektif (peran + izin langsung) dibaca dari basis data lewat cache pendek dan ditegakkan JWTAuth di
	// semua modul, dengan cara yang sama seperti pemeriksa pencabutan di atas (lihat usecase/izin.go).
	deps.Token.SetPermissionResolver(uc.PermissionResolver())
	handler := http.New(uc)

	// Caddy membuka /api/v1/* langsung ke Go, melewati batas laju BFF (5/menit/IP). Endpoint publik yang
	// menerima tebakan rahasia atau memicu email/penyimpanan dibatasi per IP klien di sini juga. Login,
	// validasi OTP, lupa dan atur ulang kata sandi berbagi satu ember (seperti di BFF); SSO punya ember sendiri
	// supaya mulai/selesai SSO tidak menghabiskan jatah login. Refresh tidak dibatasi: dipanggil otomatis
	// oleh klien dan butuh refresh token yang sah.
	batasKredensial := batasAuth(deps.AuthRateLimitPerMenit)
	batasSSO := batasAuth(deps.AuthRateLimitPerMenit)

	// Public routes — no auth middleware
	public := engine.Group("/auth")
	{
		public.POST("/login", batasKredensial, handler.Login)
		public.POST("/refresh", handler.Refresh)
		public.POST("/forgot-password", batasKredensial, handler.ForgotPassword)
		public.POST("/reset-password", batasKredensial, handler.ResetPassword)
		// Menukar token tantangan dari login dan kode OTP dengan sesi; belum ada sesi, jadi publik.
		public.POST("/mfa/validate", batasKredensial, handler.ValidateMFA)
	}
	registerSSO(public, repo, uc, deps.Logger, ssoSettings(opts, os.Getenv), batasSSO)

	// Protected routes — require valid JWT
	protected := engine.Group("")
	protected.Use(middleware.JWTAuth(deps.Token))
	{
		protected.POST("/auth/logout", handler.Logout)
		protected.POST("/auth/logout-all", handler.LogoutAll)
		protected.GET("/auth/sessions", handler.ListSessions)
		protected.DELETE("/auth/sessions/:id", handler.RevokeSession)
		protected.POST("/auth/sessions/revoke-others", handler.RevokeOtherSessions)
		protected.GET("/auth/me", handler.Me)
		protected.POST("/auth/change-password", handler.ChangePassword)

		// MFA routes
		mfa := protected.Group("/auth/mfa")
		{
			mfa.POST("/setup", handler.SetupMFA)
			mfa.POST("/verify", handler.VerifyMFA)
			mfa.POST("/backup-codes", handler.RegenerateBackupCodes)
			mfa.POST("/disable", handler.DisableMFA)
		}

		// User management — requires pengguna.kelola permission
		users := protected.Group("/users")
		users.Use(middleware.RequirePermission("pengguna.kelola"))
		{
			users.POST("", handler.CreateUser)
			users.GET("", handler.ListUsers)
			users.POST("/bulk", handler.BulkUser)
			users.GET("/:id", handler.GetUser)
			users.PATCH("/:id", handler.UpdateUser)
			users.DELETE("/:id", handler.DeleteUser)
			users.POST("/:id/reset-password", handler.ResetPasswordAdmin)
			users.POST("/:id/buka-kunci", handler.BukaKunci)
			users.POST("/:id/revoke-sessions", handler.ForceLogoutUser)
			users.PUT("/:id/roles", handler.AssignRole)
			// Izin langsung per pengguna (02-C): beri dengan alasan wajib dan masa berlaku opsional, cabut, daftar.
			users.GET("/:id/permissions", handler.ListUserPermissions)
			users.POST("/:id/permissions", handler.GrantUserPermission)
			users.DELETE("/:id/permissions/:izin", handler.RevokeUserPermission)
		}

		// RBAC catalogue and role x permission matrix (02-C) — requires pengguna.kelola permission
		rbac := protected.Group("")
		rbac.Use(middleware.RequirePermission("pengguna.kelola"))
		{
			rbac.GET("/roles", handler.ListRoles)
			rbac.PUT("/roles/:id/permissions", handler.UpdateRolePermissions)
			rbac.GET("/permissions", handler.ListPermissions)
		}
	}
}

// batasAuth membuat pembatas laju per IP untuk endpoint auth publik; n <= 0 berarti tanpa pembatas.
func batasAuth(n int) gin.HandlerFunc {
	if n <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	return middleware.AuthRateLimit(n, time.Minute)
}

// ConfigFromEnv membangun konfigurasi usecase dari lingkungan. MFA_ENFORCE ("true"/"false") menentukan
// apakah MFA diwajibkan bagi peran wajib; bila kosong, diwajibkan hanya di produksi (APP_ENV=production),
// sehingga lokal dan demo tetap dapat masuk tanpa setup. AES_KEY menyegel rahasia TOTP (lihat crypto.KeyringFromEnv).
// Konfigurasi yang salah menghentikan proses saat start (panic) agar tidak berjalan tanpa perlindungan.
func ConfigFromEnv(getenv func(string) string) usecase.Config {
	cfg := usecase.DefaultConfig()

	cfg.MFAEnforce = getenv("APP_ENV") == "production"
	if raw := getenv("MFA_ENFORCE"); raw != "" {
		enforce, err := strconv.ParseBool(raw)
		if err != nil {
			panic("MFA_ENFORCE harus true atau false, bukan " + raw)
		}
		cfg.MFAEnforce = enforce
	}

	ring, err := crypto.KeyringFromEnv(getenv)
	if err != nil {
		panic("konfigurasi AES_KEY: " + err.Error())
	}
	cfg.MFAKey = ring
	return cfg
}
