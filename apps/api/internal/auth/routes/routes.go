package routes

import (
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/auth/delivery/http"
	"github.com/garda-inflasi/api/internal/auth/repository/postgres"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/email"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/token"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Token  *token.Manager
	Logger *logger.Logger
}

type Options struct {
	UseCaseConfig usecase.Config
}

func Register(engine *gin.RouterGroup, deps *Dependencies, opts ...Options) {
	cfg := usecase.DefaultConfig()
	if len(opts) > 0 {
		cfg = opts[0].UseCaseConfig
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
	}
	if emailCfg.From == "" {
		emailCfg.From = "noreply@garda-inflasi.id"
	}
	emailSender := email.New(emailCfg)

	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, deps.Token, cfg, deps.Logger, emailSender)
	handler := http.New(uc)

	// Public routes — no auth middleware
	public := engine.Group("/auth")
	{
		public.POST("/login", handler.Login)
		public.POST("/refresh", handler.Refresh)
		public.POST("/forgot-password", handler.ForgotPassword)
		public.POST("/reset-password", handler.ResetPassword)
	}

	// Protected routes — require valid JWT
	protected := engine.Group("")
	protected.Use(middleware.JWTAuth(deps.Token))
	{
		protected.POST("/auth/logout", handler.Logout)
		protected.GET("/auth/me", handler.Me)
		protected.POST("/auth/change-password", handler.ChangePassword)

		// MFA routes
		mfa := protected.Group("/auth/mfa")
		{
			mfa.POST("/setup", handler.SetupMFA)
			mfa.POST("/verify", handler.VerifyMFA)
			mfa.POST("/validate", handler.ValidateMFA)
			mfa.POST("/disable", handler.DisableMFA)
		}

		// User management — requires pengguna.kelola permission
		users := protected.Group("/users")
		users.Use(middleware.RequirePermission("pengguna.kelola"))
		{
			users.POST("", handler.CreateUser)
			users.GET("", handler.ListUsers)
			users.GET("/:id", handler.GetUser)
		}
	}
}

