package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/master/delivery/http"
	"github.com/garda-inflasi/api/internal/master/repository/postgres"
	"github.com/garda-inflasi/api/internal/master/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Logger *logger.Logger
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := postgres.New(deps.DB)
	uc := usecase.New(repo)
	handler := http.New(uc)

	master := engine.Group("/master")
	{
		master.GET("/komoditas", handler.ListKomoditas)
	}
}
