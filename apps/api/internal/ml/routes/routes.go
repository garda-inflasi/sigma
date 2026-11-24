// Package routes mendaftarkan rute /v1/ml/* dan pengaya lampiran `ml` untuk endpoint lama.
package routes

import (
	"github.com/garda-inflasi/api/internal/ml"
	mlhttp "github.com/garda-inflasi/api/internal/ml/delivery/http"
	"github.com/garda-inflasi/api/internal/ml/orchestrator"
	"github.com/garda-inflasi/api/internal/ml/repository/postgres"
	"github.com/garda-inflasi/api/internal/ml/usecase"
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
	// Service pembaca pipeline; nil memakai ml.Shared (dari env). Tes menyuntikkan Service ber-Mongo uji.
	Service      *usecase.Service
	Orchestrator *orchestrator.Orchestrator
}

// Register memasang pengaya pada grup v1 lalu rute /v1/ml/*. Wajib dipanggil sebelum modul lain mendaftarkan rute:
// gin hanya menerapkan middleware grup pada rute yang didaftarkan sesudahnya.
func Register(v1 *gin.RouterGroup, deps *Dependencies) {
	svc := deps.Service
	if svc == nil {
		svc = ml.Shared(deps.Logger)
	}
	orch := deps.Orchestrator
	if orch == nil {
		orch = orchestrator.Shared(deps.Logger)
	}
	pg := postgres.New(deps.DB)
	v1.Use(mlhttp.NewPengaya(svc, pg).Middleware())

	handler := mlhttp.New(svc, pg, orch)
	group := v1.Group("/ml")
	group.Use(middleware.JWTAuth(deps.Token))
	{
		group.GET("/cycles", middleware.RequirePermission(mlhttp.IzinPrediksi), handler.ListSiklus)
		group.GET("/cycles/:cycle_id", middleware.RequirePermission(mlhttp.IzinPrediksi), handler.GetSiklus)
		group.GET("/freshness", middleware.RequirePermission(mlhttp.IzinSumber), handler.Freshness)
		group.GET("/jobs", middleware.RequirePermission(mlhttp.IzinSumber), handler.ListJobs)
		group.GET("/jobs/stream", middleware.RequirePermission(mlhttp.IzinSumber), handler.StreamJobs)
		group.GET("/jobs/:id", middleware.RequirePermission(mlhttp.IzinSumber), handler.GetJob)
		group.POST("/crawler/run", middleware.RequireAnyPermission("sumber.kelola", "model.kelola"), handler.TriggerCrawler)
		group.POST("/pipeline/run", middleware.RequireAnyPermission("model.kelola", "sumber.kelola"), handler.TriggerPipeline)
	}
}
