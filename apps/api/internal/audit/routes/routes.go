package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/audit/delivery/http"
	"github.com/garda-inflasi/api/internal/audit/repository/postgres"
	"github.com/garda-inflasi/api/internal/audit/usecase"
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

	engine.GET("/audit", handler.List)
}
