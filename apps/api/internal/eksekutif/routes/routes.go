// Package routes mendaftarkan rute dashboard eksekutif.
package routes

import (
	"github.com/garda-inflasi/api/internal/eksekutif/delivery/http"
	"github.com/garda-inflasi/api/internal/eksekutif/repository/postgres"
	"github.com/garda-inflasi/api/internal/eksekutif/usecase"
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
	handler := http.New(usecase.New(postgres.New(deps.DB)), deps.Logger)

	engine.GET("/dashboard/eksekutif", middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.eksekutif"), handler.Ringkasan)
}
