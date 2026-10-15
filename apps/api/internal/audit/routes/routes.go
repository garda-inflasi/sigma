package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/audit/delivery/http"
	"github.com/garda-inflasi/api/internal/audit/repository/postgres"
	"github.com/garda-inflasi/api/internal/audit/usecase"
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
	uc := usecase.New(repo)
	handler := http.New(uc)

	engine.GET("/audit", middleware.JWTAuth(deps.Token), middleware.RequirePermission("audit.lihat"), handler.List)
}
