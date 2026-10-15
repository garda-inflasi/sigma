package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/siklus/delivery/http"
	"github.com/garda-inflasi/api/internal/siklus/repository/postgres"
	"github.com/garda-inflasi/api/internal/siklus/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/token"
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

	siklus := engine.Group("/siklus")
	siklus.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("siklus.lihat"))
	{
		siklus.GET("/aktif", handler.GetAktif)
	}
}
