package dto

// ── Dashboard Summary ─────────────────────────────────────────────────

type DashboardSummaryRequest struct {
	Periode string `form:"periode"`
}

// ── Inflation Trends ──────────────────────────────────────────────────

type InflationTrendsRequest struct {
	Year   int    `form:"year"`
	Region string `form:"region"`
}

// ── Food Balance ──────────────────────────────────────────────────────

type FoodBalanceRequest struct {
	Period string `form:"period"`
}

// ── Heatmap ───────────────────────────────────────────────────────────

type HeatmapRequest struct {
	Date string `form:"date"`
}

// ── Alerts ────────────────────────────────────────────────────────────

type AlertsRequest struct {
	Status string `form:"status"`
}

// ── Price Trends ──────────────────────────────────────────────────────

type PriceTrendsRequest struct {
	Commodity string `form:"commodity"`
	Region    string `form:"region"`
	Days      int    `form:"days"`
}

// ── Forecast ──────────────────────────────────────────────────────────

type ForecastRequest struct {
	Commodity string `form:"commodity"`
	Days      int    `form:"days"`
}

// ── Feature Importance ────────────────────────────────────────────────

type FeatureImportanceRequest struct {
	Commodity string `form:"commodity"`
}

// ── Simulation ────────────────────────────────────────────────────────

type SimulationRequest struct {
	CommodityID        string  `json:"commodity_id" binding:"required"`
	RainfallPct        float64 `json:"rainfall_pct"`
	SupplyIncreasePct  float64 `json:"supply_increase_pct"`
	TransportSubsidyPct float64 `json:"transport_subsidy_pct"`
}

// ── Validation Queue ──────────────────────────────────────────────────

type ValidationQueueRequest struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

// ── Batch Approve ─────────────────────────────────────────────────────

type BatchApproveRequest struct {
	EntryIDs []string `json:"entry_ids" binding:"required"`
}
