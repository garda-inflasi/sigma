package routes

import (
	"github.com/garda-inflasi/api/internal/spasial/delivery/http"
	"github.com/garda-inflasi/api/internal/spasial/repository/postgres"
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
	uc := usecase.New(repo, deps.Logger)
	handler := http.New(uc)

	spasial := engine.Group("/spasial")
	spasial.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("spasial.lihat"))
	{
		spasial.GET("/wilayah", handler.ListWilayah)
		spasial.GET("/neraca", handler.ListNeraca)
		spasial.GET("/rute", handler.ListRute)
		spasial.GET("/kandidat-cold-storage", handler.ListKandidat)
		spasial.GET("/geometri", handler.ListGeometri)
		spasial.GET("/lokasi", handler.Lokasi)
	}
}
