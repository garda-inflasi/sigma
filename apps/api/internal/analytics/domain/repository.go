package domain

import (
	"context"
	"time"
)

// ── Dashboard Summary ─────────────────────────────────────────────────

type DashboardSummary struct {
	InflationYoY      float64              `json:"inflation_yoy"`
	InflationMoM      float64              `json:"inflation_mom"`
	FiscalSpace       *FiscalSpaceSummary  `json:"fiscal_space"`
	TopContributors   []TopContributor     `json:"top_contributors"`
	ActiveAlertsCount int                  `json:"active_alerts_count"`
	Period            string               `json:"period"`
}

type FiscalSpaceSummary struct {
	TotalPagu         float64 `json:"total_pagu"`
	RealizationPct    float64 `json:"realization_pct"`
	InterventionScore float64 `json:"intervention_score"`
}

type TopContributor struct {
	CommodityID   string  `json:"commodity_id"`
	CommodityName string  `json:"commodity_name"`
	PriceChangePct float64 `json:"price_change_pct"`
	Contribution   float64 `json:"contribution"`
}

// ── Inflation Trends ─────────────────────────────────────────────────

type InflationTrend struct {
	Date         time.Time `json:"date"`
	CommodityID  string    `json:"commodity_id"`
	AvgPrice     float64   `json:"avg_price"`
	InflationMoM float64   `json:"inflation_mom"`
	InflationYoY float64   `json:"inflation_yoy"`
	Status       string    `json:"status"`
}

// ── Food Balance ──────────────────────────────────────────────────────

type FoodBalanceItem struct {
	CommodityID   string  `json:"commodity_id"`
	CommodityName string  `json:"commodity_name"`
	SurplusDeficit float64 `json:"surplus_deficit"`
	Status        string  `json:"status"`
}

// ── Heatmap ───────────────────────────────────────────────────────────

type HeatmapItem struct {
	Date           time.Time `json:"date"`
	CommodityID    string    `json:"commodity_id"`
	CommodityName  string    `json:"commodity_name"`
	RegionID       string    `json:"region_id"`
	RegionName     string    `json:"region_name"`
	Price          float64   `json:"price"`
	PriceChangePct float64   `json:"price_change_pct"`
	Status         string    `json:"status"`
}

// ── Alert ─────────────────────────────────────────────────────────────

type AlertItemView struct {
	AlertID        string    `json:"alert_id"`
	CommodityID    string    `json:"commodity_id"`
	CommodityName  string    `json:"commodity_name"`
	RegionID       string    `json:"region_id"`
	Type           string    `json:"type"`
	Severity       string    `json:"severity"`
	PriceChangePct float64   `json:"price_change_pct"`
	Timestamp      time.Time `json:"timestamp"`
}

// ── Price Trend ───────────────────────────────────────────────────────

type PriceTrendItem struct {
	Date           time.Time `json:"date"`
	Price          float64   `json:"price"`
	PriceChangePct float64   `json:"price_change_pct"`
	Avg7Day        float64   `json:"avg_7day"`
}

// ── Region Status Map ─────────────────────────────────────────────────

type RegionStatusMap struct {
	RegionID   string      `json:"region_id"`
	RegionName string      `json:"region_name"`
	Status     string      `json:"status"`
	GeoJSON    interface{} `json:"geojson"`
}

// ── Forecast ──────────────────────────────────────────────────────────

type ForecastResult struct {
	CommodityID        string               `json:"commodity_id"`
	CommodityName      string               `json:"commodity_name"`
	CurrentPrice       float64              `json:"current_price"`
	Forecast14dPrice   float64              `json:"forecast_14d_price"`
	ForecastChangePct  float64              `json:"forecast_change_pct"`
	ConfidenceInterval80 ConfidenceInterval80 `json:"confidence_interval_80"`
	Mape               float64              `json:"mape"`
	ModelVersion       string               `json:"model_version"`
}

// ── Feature Importance ────────────────────────────────────────────────

type FeatureImportanceView struct {
	CommodityID      string                 `json:"commodity_id"`
	Features         []FeatureImportanceItem `json:"features"`
}

// ── Simulation ────────────────────────────────────────────────────────

type SimulationRequest struct {
	CommodityID        string  `json:"commodity_id" binding:"required"`
	RainfallPct        float64 `json:"rainfall_pct"`
	SupplyIncreasePct  float64 `json:"supply_increase_pct"`
	TransportSubsidyPct float64 `json:"transport_subsidy_pct"`
}

type SimulationResult struct {
	CommodityID       string  `json:"commodity_id"`
	CurrentPrice      float64 `json:"current_price"`
	ProjectedPrice    float64 `json:"projected_price"`
	ProjectedChangePct float64 `json:"projected_change_pct"`
	Scenario          SimulationRequest `json:"scenario"`
}

// ── All Commodities Forecast ──────────────────────────────────────────

type AllCommoditiesForecast struct {
	Items []ForecastResult `json:"items"`
}

// ── Validation Queue ──────────────────────────────────────────────────

type ValidationQueueItem struct {
	EntryID           string     `json:"entry_id"`
	CommodityID       string     `json:"commodity_id"`
	MarketID          string     `json:"market_id"`
	RegionID          string     `json:"region_id"`
	Price             float64    `json:"price"`
	PriceDeviationPct float64    `json:"price_deviation_pct"`
	RecordedBy        string     `json:"recorded_by"`
	RecordedAt        time.Time  `json:"recorded_at"`
	Status            string     `json:"status"`
}

type BatchApproveRequest struct {
	EntryIDs []string `json:"entry_ids" binding:"required"`
}

// ── Field Activity Progress ───────────────────────────────────────────

type FieldActivityProgress struct {
	RegionID      string `json:"region_id"`
	RegionName    string `json:"region_name"`
	TotalEntries  int    `json:"total_entries"`
	Validated     int    `json:"validated"`
	Pending       int    `json:"pending"`
	Rejected      int    `json:"rejected"`
	Date          string `json:"date"`
}

// ── Model Registry View ───────────────────────────────────────────────

type ModelRegistryView struct {
	CommodityID string       `json:"commodity_id"`
	Version     string       `json:"version"`
	Algorithm   string       `json:"algorithm"`
	Metrics     ModelMetrics `json:"metrics"`
	Status      string       `json:"status"`
	TrainedAt   time.Time    `json:"trained_at"`
	DeployedAt  *time.Time   `json:"deployed_at,omitempty"`
}

// ── Repository Interface ──────────────────────────────────────────────

type AnalyticsRepository interface {
	// Dashboard Summary
	GetDashboardSummary(ctx context.Context) (*DashboardSummary, error)

	// Inflation Trends
	GetInflationTrends(ctx context.Context, year int, region string) ([]InflationTrend, error)

	// Food Balance
	GetFoodBalance(ctx context.Context, period string) ([]FoodBalanceItem, error)

	// Fiscal Space
	GetFiscalSpace(ctx context.Context) (*FiscalSpaceSummary, error)

	// Heatmap
	GetHeatmap(ctx context.Context, date string) ([]HeatmapItem, error)

	// Alerts
	GetActiveAlerts(ctx context.Context) ([]AlertItemView, error)

	// Price Trends
	GetPriceTrends(ctx context.Context, commodityID, regionID string, days int) ([]PriceTrendItem, error)

	// Region Status Map
	GetRegionStatusMap(ctx context.Context) ([]RegionStatusMap, error)

	// Forecast
	GetForecast(ctx context.Context, commodityID string, days int) (*ForecastResult, error)

	// Feature Importance
	GetFeatureImportance(ctx context.Context, commodityID string) (*FeatureImportanceView, error)

	// Simulation
	SimulateScenario(ctx context.Context, req SimulationRequest) (*SimulationResult, error)

	// All Commodities Forecast
	GetAllCommoditiesForecast(ctx context.Context) ([]ForecastResult, error)

	// Validation Queue
	GetValidationQueue(ctx context.Context, page, limit int) ([]ValidationQueueItem, int64, error)

	// Batch Approve
	BatchApprove(ctx context.Context, entryIDs []string, validatedBy string) (int64, error)

	// Field Activity Progress
	GetFieldActivityProgress(ctx context.Context) ([]FieldActivityProgress, error)

	// Model Registry
	GetModelRegistry(ctx context.Context) ([]ModelRegistryView, error)
}
