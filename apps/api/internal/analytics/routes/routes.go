package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/analytics/delivery/http"
	"github.com/garda-inflasi/api/internal/analytics/repository/mongodb"
	"github.com/garda-inflasi/api/internal/analytics/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/token"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB     *gorm.DB
	Mongo  *mongo.Client
	Logger *logger.Logger
	Token  *token.Manager
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	repo := mongodb.New(deps.Mongo, deps.Logger)
	uc := usecase.New(repo, deps.Logger)
	handler := http.New(uc)

	// ── Domain 1: Executive Summary & Dashboard ────────────────────────
	// Requires dashboard.eksekutif permission
	dashboard := engine.Group("/dashboard")
	dashboard.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.eksekutif"))
	{
		dashboard.GET("/summary", handler.GetDashboardSummary)
	}

	// ── Domain 1: Inflation Trends ─────────────────────────────────────
	inflation := engine.Group("/inflation")
	inflation.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.eksekutif"))
	{
		inflation.GET("/trends", handler.GetInflationTrends)
	}

	// ── Domain 1: Food Balance ─────────────────────────────────────────
	foodBalance := engine.Group("/food-balance")
	foodBalance.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.eksekutif"))
	{
		foodBalance.GET("", handler.GetFoodBalance)
	}

	// ── Domain 1: Fiscal Space ─────────────────────────────────────────
	fiscalSpace := engine.Group("/fiscal-space")
	fiscalSpace.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.eksekutif"))
	{
		fiscalSpace.GET("", handler.GetFiscalSpace)
	}

	// ── Domain 2: Operations Monitoring ────────────────────────────────
	// Requires dashboard.operasional permission
	operations := engine.Group("/operations")
	operations.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.operasional"))
	{
		operations.GET("/heatmap", handler.GetHeatmap)
		operations.GET("/alerts", handler.GetActiveAlerts)
		operations.GET("/trends", handler.GetPriceTrends)
	}

	// ── Domain 2: Regions Status Map ───────────────────────────────────
	regions := engine.Group("/regions")
	regions.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("dashboard.operasional"))
	{
		regions.GET("/status-map", handler.GetRegionStatusMap)
	}

	// ── Domain 3: Predictions & XAI ────────────────────────────────────
	// Requires prediksi.baca permission
	predictions := engine.Group("/predictions")
	predictions.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("prediksi.baca"))
	{
		predictions.GET("/forecast", handler.GetForecast)
		predictions.GET("/features", handler.GetFeatureImportance)
		predictions.POST("/simulate", handler.SimulateScenario)
		predictions.GET("/all-commodities", handler.GetAllCommoditiesForecast)
	}

	// ── Domain 4: Validations ──────────────────────────────────────────
	// Requires lapangan.validasi permission
	validations := engine.Group("/validations")
	validations.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("lapangan.validasi"))
	{
		validations.GET("/queue", handler.GetValidationQueue)
		validations.POST("/batch-approve", handler.BatchApprove)
	}

	// ── Domain 4: Field Activities ─────────────────────────────────────
	fieldActivities := engine.Group("/field-activities")
	fieldActivities.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("lapangan.baca"))
	{
		fieldActivities.GET("/progress", handler.GetFieldActivityProgress)
	}

	// ── Domain 5: Model Management ─────────────────────────────────────
	models := engine.Group("/models")
	models.Use(middleware.JWTAuth(deps.Token), middleware.RequirePermission("prediksi.baca"))
	{
		models.GET("/registry", handler.GetModelRegistry)
	}
}
