// Package routes provides route registration for the master module.
package routes

import (
	"github.com/garda-inflasi/api/internal/master/delivery/http"
	"github.com/garda-inflasi/api/internal/master/repository/postgres"
	"github.com/garda-inflasi/api/internal/master/usecase"
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
	uc := usecase.New(repo)
	handler := http.New(uc)

	master := engine.Group("/master")
	master.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("master.lihat"))
	{
		master.GET("/komoditas", handler.ListKomoditas)
		master.GET("/wilayah", handler.ListWilayah)
		master.GET("/pasar", handler.ListPasar)
		master.POST("/komoditas/bulk", middleware.RequirePermission("master.ajukan"), handler.BulkKomoditas)
	}
}
