package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/prediksi/delivery/http"
	"github.com/garda-inflasi/api/internal/prediksi/repository/postgres"
	"github.com/garda-inflasi/api/internal/prediksi/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Logger *logger.Logger
}

type Options struct {
	// Module-specific options
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, deps.Logger)
	handler := http.New(uc)

	prediksi := engine.Group("/prediksi")
	{
		prediksi.GET("", handler.List)
		prediksi.GET("/:kode", handler.GetByKode)
		prediksi.GET("/:kode/export", handler.Export)
	}
}
