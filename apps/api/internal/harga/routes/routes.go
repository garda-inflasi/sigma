package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/harga/delivery/http"
	"github.com/garda-inflasi/api/internal/harga/repository/postgres"
	"github.com/garda-inflasi/api/internal/harga/usecase"
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

	harga := engine.Group("/harga")
	harga.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("harga.lihat"))
	{
		harga.GET("", handler.ListHarga)
		harga.GET("/tren", handler.GetTren)
	}
}
