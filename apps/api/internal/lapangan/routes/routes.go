package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	hargahttp "github.com/garda-inflasi/api/internal/harga/delivery/http"
	hargapostgres "github.com/garda-inflasi/api/internal/harga/repository/postgres"
	hargausecase "github.com/garda-inflasi/api/internal/harga/usecase"
	"github.com/garda-inflasi/api/internal/lapangan/delivery/http"
	"github.com/garda-inflasi/api/internal/lapangan/repository/postgres"
	"github.com/garda-inflasi/api/internal/lapangan/usecase"
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
	auditWriter := audit.NewWriter(deps.DB)
	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, auditWriter, deps.Logger)
	handler := http.New(uc)

	hargaUC := hargausecase.New(hargapostgres.New(deps.DB), auditWriter, refresher.Shared(deps.DB, deps.Logger), deps.Logger)
	hargaHandler := hargahttp.New(hargaUC)

	lapangan := engine.Group("/lapangan")
	lapangan.Use(middleware.JWTAuth(deps.Token))
	{
		tugas := lapangan.Group("/tugas")
		tugas.Use(middleware.RequireAnyPermission("tugas.lihat_sendiri", "tugas.kelola"))
		{
			tugas.GET("/saya", handler.ListSaya)
			tugas.GET("/:id", handler.Get)
			// Petugas menutup tugasnya sendiri; tugas.kelola membuka tugas petugas lain.
			tugas.POST("/bulk", handler.Bulk)
		}

		// Kiriman harga dan data acuan offline untuk sync engine PWA.
		lapangan.POST("/harga", middleware.RequirePermission("harga.input"), hargaHandler.KirimLapangan)
		lapangan.GET("/acuan", middleware.RequirePermission("harga.input"), handler.Acuan)
	}
}
