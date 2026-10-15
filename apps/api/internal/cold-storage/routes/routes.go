package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/cold-storage/delivery/http"
	"github.com/garda-inflasi/api/internal/cold-storage/repository/postgres"
	"github.com/garda-inflasi/api/internal/cold-storage/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Logger *logger.Logger
}

type Options struct {
	// Module-specific options
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := postgres.New(deps.DB)
	uc := usecase.New(repo, deps.Logger)
	handler := http.New(uc)

	engine.GET("/cold-storage", handler.GetAll)
}
