package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/auth/delivery/http"
	"github.com/garda-inflasi/api/internal/auth/repository/postgres"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
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

	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, deps.Token, cfg, deps.Logger)
	handler := http.New(uc)

	// Public routes
	auth := engine.Group("/auth")
	{
		auth.POST("/login", handler.Login)
		auth.POST("/refresh", handler.Refresh)
	}

	// Protected routes
	protected := engine.Group("")
	protected.Use(authMiddleware(deps.Token))
	{
		protected.POST("/auth/logout", handler.Logout)
		protected.GET("/auth/me", handler.Me)

		// User management
		users := protected.Group("/users")
		{
			users.POST("", handler.CreateUser)
			users.GET("", handler.ListUsers)
			users.GET("/:id", handler.GetUser)
		}
	}
}

func authMiddleware(tokenMgr *token.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := c.GetHeader("Authorization")
		if bearer == "" || len(bearer) < 7 || bearer[:7] != "Bearer " {
			c.AbortWithStatusJSON(401, gin.H{"code": "401", "message": "Unauthorized"})
			return
		}

		tokenStr := bearer[7:]
		claims, err := tokenMgr.VerifyToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"code": "401", "message": "Token tidak valid"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

