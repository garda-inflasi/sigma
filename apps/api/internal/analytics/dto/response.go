package dto

import "github.com/garda-inflasi/api/internal/analytics/domain"

// ── Dashboard Summary ─────────────────────────────────────────────────

type DashboardSummaryResponse struct {
	Data *domain.DashboardSummary `json:"data"`
}

// ── Inflation Trends ──────────────────────────────────────────────────

type InflationTrendsResponse struct {
	Items []domain.InflationTrend `json:"items"`
}

// ── Food Balance ──────────────────────────────────────────────────────

type FoodBalanceResponse struct {
	Items []domain.FoodBalanceItem `json:"items"`
}

// ── Fiscal Space ──────────────────────────────────────────────────────

type FiscalSpaceResponse struct {
	Data *domain.FiscalSpaceSummary `json:"data"`
}

// ── Heatmap ───────────────────────────────────────────────────────────

type HeatmapResponse struct {
	Items []domain.HeatmapItem `json:"items"`
}

// ── Alerts ────────────────────────────────────────────────────────────

type AlertsResponse struct {
	Items []domain.AlertItemView `json:"items"`
}

// ── Price Trends ──────────────────────────────────────────────────────

type PriceTrendsResponse struct {
	Items []domain.PriceTrendItem `json:"items"`
}

// ── Region Status Map ─────────────────────────────────────────────────

type RegionStatusMapResponse struct {
	Items []domain.RegionStatusMap `json:"items"`
}

// ── Forecast ──────────────────────────────────────────────────────────

type ForecastResponse struct {
	Data *domain.ForecastResult `json:"data"`
}

// ── Feature Importance ────────────────────────────────────────────────

type FeatureImportanceResponse struct {
	Data *domain.FeatureImportanceView `json:"data"`
}

// ── Simulation ────────────────────────────────────────────────────────

type SimulationResponse struct {
	Data *domain.SimulationResult `json:"data"`
}

// ── All Commodities Forecast ──────────────────────────────────────────

type AllCommoditiesForecastResponse struct {
	Items []domain.ForecastResult `json:"items"`
}

// ── Validation Queue ──────────────────────────────────────────────────

type ValidationQueueResponse struct {
	Items      []domain.ValidationQueueItem `json:"items"`
	Total      int64                        `json:"total"`
	Page       int                          `json:"page"`
	Limit      int                          `json:"limit"`
}

// ── Batch Approve ─────────────────────────────────────────────────────

type BatchApproveResponse struct {
	Approved int64 `json:"approved"`
}

// ── Field Activity Progress ───────────────────────────────────────────

type FieldActivityProgressResponse struct {
	Items []domain.FieldActivityProgress `json:"items"`
}

// ── Model Registry ────────────────────────────────────────────────────

type ModelRegistryResponse struct {
	Items []domain.ModelRegistryView `json:"items"`
}
