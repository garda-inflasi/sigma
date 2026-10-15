package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/intervensi/delivery/http"
	"github.com/garda-inflasi/api/internal/intervensi/repository/postgres"
	"github.com/garda-inflasi/api/internal/intervensi/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Logger *logger.Logger
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, deps.Logger)
	handler := http.New(uc)

	group := engine.Group("/intervensi")
	{
		group.GET("", handler.List)
		group.PATCH("/:id/tahap", handler.UpdateTahap)
	}
}
