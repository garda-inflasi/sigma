package routes

import (
	"github.com/garda-inflasi/api/internal/ews/delivery/http"
	"github.com/garda-inflasi/api/internal/ews/repository/postgres"
	"github.com/garda-inflasi/api/internal/ews/usecase"
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
	uc := usecase.New(repo, deps.Logger)
	handler := http.New(uc)

	group := engine.Group("/ews")
	group.Use(middleware.JWTAuth(deps.Token))
	{
		group.GET("", middleware.RequirePermission("ews.lihat"), handler.List)
		group.GET("/:id", middleware.RequirePermission("ews.lihat"), handler.GetByID)
		group.POST("/bulk", middleware.RequirePermission("ews.akui"), handler.Bulk)
	}
}
