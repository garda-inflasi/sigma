package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/dashboard/delivery/http"
	"github.com/garda-inflasi/api/internal/dashboard/repository/postgres"
	"github.com/garda-inflasi/api/internal/dashboard/usecase"
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

	// Operasional
	operasional := engine.Group("/dashboard/operasional")
	{
		operasional.GET("/kpi", handler.GetKpiOperasional)
		operasional.GET("/heatmap", handler.GetHeatmap)
	}

	// Eksekutif
	eksekutif := engine.Group("/dashboard/eksekutif")
	{
		eksekutif.GET("", handler.GetRingkasanEksekutif)
	}
}

