package routes

import (
	"github.com/garda-inflasi/api/internal/audit"
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
	uc := usecase.New(repo, audit.NewWriter(deps.DB), deps.Logger)
	handler := http.New(uc)

	sumber := engine.Group("/sumber-data")
	sumber.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("sumber.lihat"))
	{
		sumber.GET("", handler.List)
		sumber.GET("/:id", handler.Get)
		sumber.POST("/bulk", middleware.RequirePermission("sumber.kelola"), handler.Bulk)
	}
}
