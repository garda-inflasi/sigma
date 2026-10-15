package routes

import (
	"github.com/garda-inflasi/api/internal/harga/delivery/http"
	"github.com/garda-inflasi/api/internal/harga/repository/postgres"
	"github.com/garda-inflasi/api/internal/harga/usecase"
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

	harga := engine.Group("/harga")
	harga.Use(middleware.JWTAuth(deps.Token))
	{
		harga.GET("", middleware.RequirePermission("harga.lihat"), handler.ListHarga)
		harga.GET("/tren", middleware.RequirePermission("harga.lihat"), handler.GetTren)
		harga.POST("", middleware.RequirePermission("harga.input"), handler.CreateHarga)
		harga.GET("/:id", middleware.RequirePermission("harga.lihat"), handler.GetHargaByID)
		harga.POST("/bulk", middleware.RequirePermission("harga.validasi"), handler.BulkHarga)
	}
}
