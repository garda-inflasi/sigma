package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/cold-storage/delivery/http"
	"github.com/garda-inflasi/api/internal/cold-storage/repository/postgres"
	"github.com/garda-inflasi/api/internal/cold-storage/usecase"
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

type Options struct {
	// Module-specific options
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := postgres.New(deps.DB)
	auditWriter := audit.NewWriter(deps.DB)
	uc := usecase.New(repo, deps.Logger, auditWriter)
	ops := usecase.NewOperasi(repo, deps.Logger, auditWriter)
	handler := http.New(uc, ops)

	auth := middleware.JWTAuth(deps.Token)

	engine.GET("/cold-storage", auth, middleware.RequirePermission("coldstorage.lihat"), handler.GetAll)
	// Izin per aksi (logistik.kelola / coldstorage.kelola) diperiksa di handler.
	engine.POST("/cold-storage/bulk", auth, middleware.RequireAnyPermission("coldstorage.kelola", "logistik.kelola"), handler.Bulk)

	engine.GET("/cold-storage/fasilitas/:id", auth, middleware.RequirePermission("coldstorage.lihat"), handler.Detail)
	engine.PATCH("/cold-storage/fasilitas/:id", auth, middleware.RequirePermission("coldstorage.kelola"), handler.UbahFasilitas)
	engine.POST("/cold-storage/telemetri", auth, middleware.RequirePermission("coldstorage.kelola"), handler.IngestTelemetri)
	// Pindah stok boleh dilakukan pemegang logistik.kelola; jenis lain diperiksa di handler.
	engine.POST("/cold-storage/transaksi", auth, middleware.RequireAnyPermission("coldstorage.kelola", "logistik.kelola"), handler.Transaksi)
}
