package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/siklus/delivery/http"
	"github.com/garda-inflasi/api/internal/siklus/repository/postgres"
	"github.com/garda-inflasi/api/internal/siklus/usecase"
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

	siklus := engine.Group("/siklus")
	{
		siklus.GET("/aktif", handler.GetAktif)
	}
}
