package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/policy-brief/delivery/http"
	"github.com/garda-inflasi/api/internal/policy-brief/repository/postgres"
	"github.com/garda-inflasi/api/internal/policy-brief/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Logger *logger.Logger
	Token  *token.Manager
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, audit.NewWriter(deps.DB), deps.Logger)
	handler := http.New(uc)

	group := engine.Group("/policy-brief")
	group.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("brief.lihat"))
	{
		group.GET("", handler.List)
		group.POST("/generate", middleware.RequirePermission("brief.susun"), handler.Generate)
	}
}
