package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/prediksi/delivery/http"
	"github.com/garda-inflasi/api/internal/prediksi/mlclient"
	"github.com/garda-inflasi/api/internal/prediksi/repository/postgres"
	"github.com/garda-inflasi/api/internal/prediksi/usecase"
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
	// ML adalah klien layanan ML; nil memakai ML_SERVICE_URL, dan tanpa itu pelatihan menjawab 503.
	ML mlclient.Client
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := postgres.New(deps.DB)
	auditWriter := audit.NewWriter(deps.DB)
	handler := http.New(usecase.New(repo), auditWriter)

	ml := deps.ML
	if ml == nil {
		ml = mlclient.FromEnv()
	}
	mlops := http.NewMLOps(usecase.NewMLOps(repo, ml, auditWriter, deps.Logger))
	simulasi := http.NewSimulasi(usecase.NewSimulasi(repo, auditWriter, deps.Logger))

	prediksi := engine.Group("/prediksi")
	prediksi.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("prediksi.lihat"))
	{
		prediksi.GET("", handler.List)
		prediksi.GET("/registri", handler.Registri)
		prediksi.GET("/ml", mlops.Status)
		prediksi.GET("/:kode", handler.GetByKode)
		prediksi.GET("/:kode/export", handler.Export)

		kelola := middleware.RequirePermission("model.kelola")
		prediksi.POST("/latih", kelola, mlops.Latih)
		prediksi.POST("/segarkan", kelola, mlops.Segarkan)
		prediksi.POST("/evaluasi", kelola, mlops.Evaluasi)
		prediksi.POST("/registri/bulk", kelola, mlops.Bulk)
		prediksi.POST("/simulasi", simulasi.Hitung)
	}
}
