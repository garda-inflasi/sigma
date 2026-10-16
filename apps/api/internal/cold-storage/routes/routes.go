package routes

import (
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
	uc := usecase.New(repo, deps.Logger)
	handler := http.New(uc)

	engine.GET("/cold-storage", middleware.JWTAuth(deps.Token), middleware.RequirePermission("coldstorage.lihat"), handler.GetAll)
	// Izin per aksi (logistik.kelola / coldstorage.kelola) diperiksa di handler.
	engine.POST("/cold-storage/bulk", middleware.JWTAuth(deps.Token), middleware.RequireAnyPermission("coldstorage.kelola", "logistik.kelola"), handler.Bulk)
}
