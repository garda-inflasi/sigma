package routes

import (
	"github.com/garda-inflasi/api/internal/lapangan/delivery/http"
	"github.com/garda-inflasi/api/internal/lapangan/repository/postgres"
	"github.com/garda-inflasi/api/internal/lapangan/usecase"
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

	lapangan := engine.Group("/lapangan")
	lapangan.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("lapangan.lihat"))
	{
		tugas := lapangan.Group("/tugas")
		{
			tugas.GET("/saya", handler.ListSaya)
			tugas.GET("/:id", handler.Get)
			tugas.POST("/bulk", middleware.RequirePermission("tugas.kelola"), handler.Bulk)
		}
	}
}
