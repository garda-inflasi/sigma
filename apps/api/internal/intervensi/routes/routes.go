package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/intervensi/delivery/http"
	"github.com/garda-inflasi/api/internal/intervensi/repository/postgres"
	"github.com/garda-inflasi/api/internal/intervensi/usecase"
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
	uc := usecase.New(repo, audit.NewWriter(deps.DB), deps.Logger)
	handler := http.New(uc)

	group := engine.Group("/intervensi")
	group.Use(middleware.JWTAuth(deps.Token))
	{
		group.GET("", middleware.RequirePermission("intervensi.lihat"), handler.List)
		group.GET("/tahap", middleware.RequirePermission("intervensi.lihat"), handler.ListTahap)
		group.POST("", middleware.RequirePermission("intervensi.kelola"), handler.Create)
		// Peran penyetuju memindahkan kartu ke tahap persetujuan tanpa izin kelola;
		// izin yang tepat menurut tahap tujuan diperiksa di usecase.
		group.PATCH("/:id/tahap",
			middleware.RequireAnyPermission("intervensi.kelola", "intervensi.setujui", "intervensi.setujui_fiskal"),
			handler.UpdateTahap)
	}
}
