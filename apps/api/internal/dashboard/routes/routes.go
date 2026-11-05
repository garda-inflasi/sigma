package routes

import (
	"github.com/garda-inflasi/api/internal/dashboard/delivery/http"
	"github.com/garda-inflasi/api/internal/dashboard/repository/postgres"
	"github.com/garda-inflasi/api/internal/dashboard/usecase"
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

	// Operasional — requires dashboard.operasional permission
	operasional := engine.Group("/dashboard/operasional")
	operasional.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.operasional"))
	{
		operasional.GET("/kpi", handler.GetKpiOperasional)
		operasional.GET("/heatmap", handler.GetHeatmap)
		operasional.GET("/aktivitas-lapangan", handler.GetAktivitasLapangan)
		operasional.GET("/pembanding-harga", handler.GetPembandingHarga)
		operasional.GET("/cuaca", handler.GetCuaca)
	}

	// Eksekutif: modul internal/eksekutif (GET /dashboard/eksekutif).
}
