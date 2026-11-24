package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/spasial/delivery/http"
	"github.com/garda-inflasi/api/internal/spasial/repository/postgres"
	"github.com/garda-inflasi/api/internal/spasial/routing"
	"github.com/garda-inflasi/api/internal/spasial/usecase"
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
	auditWriter := audit.NewWriter(deps.DB)
	uc := usecase.New(repo, deps.Logger)
	opt := usecase.NewOptimasi(repo, deps.Logger, auditWriter)
	handler := http.New(uc, opt)

	routerClient := routing.NewClientFromEnv()
	routeCache := routing.NewCacheFromEnv()
	titik := http.NewTitikWithRouting(deps.DB, routerClient, routeCache)
	gangguan := http.NewGangguan(deps.DB, routeCache, auditWriter)

	spasial := engine.Group("/spasial")
	spasial.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("spasial.lihat"))
	{
		spasial.GET("/wilayah", handler.ListWilayah)
		spasial.GET("/neraca", handler.ListNeraca)
		spasial.GET("/rute", handler.ListRute)
		spasial.GET("/kandidat-cold-storage", handler.ListKandidat)
		spasial.GET("/geometri", handler.ListGeometri)
		spasial.GET("/lokasi", handler.Lokasi)
		spasial.GET("/titik-desa", titik.ListTitikDesa)
		spasial.GET("/jarak", titik.HitungJarak)
		spasial.GET("/gangguan-jalan", gangguan.ListGangguanJalan)
		spasial.POST("/gangguan-jalan", middleware.RequirePermission("logistik.kelola"), gangguan.CreateGangguanJalan)
		spasial.PATCH("/gangguan-jalan/:id", middleware.RequirePermission("logistik.kelola"), gangguan.UpdateGangguanJalan)
		spasial.DELETE("/gangguan-jalan/:id", middleware.RequirePermission("logistik.kelola"), gangguan.DeleteGangguanJalan)
		spasial.POST("/optimasi-rute", handler.Optimasi)
		spasial.GET("/optimasi-rute", handler.RiwayatOptimasi)
	}
}
