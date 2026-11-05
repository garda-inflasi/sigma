// Package routes mendaftarkan rute indikator kinerja (KPI).
package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/kpi/delivery/http"
	"github.com/garda-inflasi/api/internal/kpi/repository/postgres"
	"github.com/garda-inflasi/api/internal/kpi/usecase"
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
	uc := usecase.New(postgres.New(deps.DB), audit.NewWriter(deps.DB), deps.Logger)
	handler := http.New(uc, deps.Logger)

	kpi := engine.Group("/kpi")
	kpi.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("kpi.lihat"))
	{
		kpi.GET("", handler.List)
		kpi.GET("/export", handler.Export)
		kpi.GET("/:kode/riwayat", handler.Riwayat)
	}
}
