package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/intervensi/delivery/http"
	"github.com/garda-inflasi/api/internal/intervensi/repository/postgres"
	"github.com/garda-inflasi/api/internal/intervensi/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/token"
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
	group.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("intervensi.lihat"))
	{
		group.GET("", handler.List)
		group.PATCH("/:id/tahap", handler.UpdateTahap)
	}
}
