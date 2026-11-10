package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/siklus/delivery/http"
	"github.com/garda-inflasi/api/internal/siklus/jobs"
	"github.com/garda-inflasi/api/internal/siklus/repository/postgres"
	"github.com/garda-inflasi/api/internal/siklus/usecase"
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
	shared := jobs.Shared(deps.DB, deps.Logger)
	uc := usecase.New(repo, shared.Engine, audit.NewWriter(deps.DB), deps.Logger)
	handler := http.New(uc)

	siklus := engine.Group("/siklus")
	siklus.Use(middleware.JWTAuth(deps.Token))
	{
		// Siklus drives the ambient UI theme for every console page, so it is gated
		// on the same permission as the operational dashboard rather than a
		// dedicated one (no role is granted "siklus.lihat").
		siklus.GET("/aktif", middleware.RequirePermission("dashboard.operasional"), handler.GetAktif)
		// Aturan penentuan siklus adalah ambang turunan EWS: dibaca dengan izin yang sama dengan GET /v1/ews/aturan.
		siklus.GET("/aturan", middleware.RequirePermission("ews.lihat"), handler.ListAturan)
		// Evaluasi manual meniru POST /v1/ews/evaluasi: izin mengatur ambang, bukan izin baru.
		siklus.POST("/evaluasi", middleware.RequirePermission("ews.atur_ambang"), handler.Evaluasi)
	}
}
