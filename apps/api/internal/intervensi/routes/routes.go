package routes

import (
	"github.com/garda-inflasi/api/internal/intervensi/delivery/http"
	"github.com/garda-inflasi/api/internal/intervensi/repository/postgres"
	"github.com/garda-inflasi/api/internal/intervensi/usecase"
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

	group := engine.Group("/intervensi")
	group.Use(middleware.JWTAuth(deps.Token))
	{
		group.GET("", middleware.RequirePermission("intervensi.lihat"), handler.List)
		group.POST("", middleware.RequirePermission("intervensi.kelola"), handler.Create)
		group.PATCH("/:id/tahap", middleware.RequirePermission("intervensi.kelola"), handler.UpdateTahap)
		group.POST("/bulk", middleware.RequirePermission("intervensi.kelola"), handler.Bulk)
	}
}
