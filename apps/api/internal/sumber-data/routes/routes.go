package routes

import (
	"github.com/garda-inflasi/api/internal/sumber-data/delivery/http"
	"github.com/garda-inflasi/api/internal/sumber-data/repository/postgres"
	"github.com/garda-inflasi/api/internal/sumber-data/usecase"
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

	engine.GET("/sumber-data", middleware.JWTAuth(deps.Token), middleware.RequirePermission("sumber.lihat"), handler.List)

	protected := engine.Group("/sumber-data")
	protected.Use(middleware.JWTAuth(deps.Token))
	{
		protected.GET("/:id", middleware.RequirePermission("sumber.lihat"), handler.Get)
		protected.POST("/bulk", middleware.RequirePermission("sumber.kelola"), handler.Bulk)
	}
}
