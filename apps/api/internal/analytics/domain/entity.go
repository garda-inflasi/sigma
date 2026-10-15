package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ── CommodityDailyBucket (Core Analytics) ─────────────────────────────
// Bucket keys: date + commodity_id
// Stores aggregated daily prices, regional breakdown, alerts, and AI predictions.

type CommodityMetadata struct {
	Name     string `json:"name" bson:"name"`
	Unit     string `json:"unit" bson:"unit"`
	Category string `json:"category" bson:"category"`
}

type ProvincialMetrics struct {
	AvgPrice     float64 `json:"avg_price" bson:"avg_price"`
	InflationMom float64 `json:"inflation_mom" bson:"inflation_mom"`
	InflationYoy float64 `json:"inflation_yoy" bson:"inflation_yoy"`
	Status       string  `json:"status" bson:"status"`
}

type RegionalBreakdownItem struct {
	RegionID       string  `json:"region_id" bson:"region_id"`
	RegionName     string  `json:"region_name" bson:"region_name"`
	Price          float64 `json:"price" bson:"price"`
	PriceChangePct float64 `json:"price_change_pct" bson:"price_change_pct"`
	Status         string  `json:"status" bson:"status"`
}

type OperationalMetrics struct {
	TotalFieldEntries          int     `json:"total_field_entries" bson:"total_field_entries"`
	ValidationRatePct          float64 `json:"validation_rate_pct" bson:"validation_rate_pct"`
	ColdStorageUtilizationPct  float64 `json:"cold_storage_utilization_pct" bson:"cold_storage_utilization_pct"`
	CommoditiesAboveNormal     int     `json:"commodities_above_normal" bson:"commodities_above_normal"`
}

type AlertItem struct {
	AlertID        string    `json:"alert_id" bson:"alert_id"`
	RegionID       string    `json:"region_id" bson:"region_id"`
	Type           string    `json:"type" bson:"type"`
	Severity       string    `json:"severity" bson:"severity"`
	PriceChangePct float64   `json:"price_change_pct" bson:"price_change_pct"`
	Timestamp      time.Time `json:"timestamp" bson:"timestamp"`
}

type ConfidenceInterval80 struct {
	Lower float64 `json:"lower" bson:"lower"`
	Upper float64 `json:"upper" bson:"upper"`
}

type FeatureImportanceItem struct {
	Feature          string  `json:"feature" bson:"feature"`
	ContributionScore float64 `json:"contribution_score" bson:"contribution_score"`
}

type AIPredictions struct {
	ModelVersion       string                `json:"model_version" bson:"model_version"`
	Forecast14dPrice   float64               `json:"forecast_14d_price" bson:"forecast_14d_price"`
	ForecastChangePct  float64               `json:"forecast_change_pct" bson:"forecast_change_pct"`
	ConfidenceInterval80 ConfidenceInterval80 `json:"confidence_interval_80" bson:"confidence_interval_80"`
	Mape               float64               `json:"mape" bson:"mape"`
	FeatureImportance  []FeatureImportanceItem `json:"feature_importance" bson:"feature_importance"`
}

type CommodityDailyBucket struct {
	ID                  bson.ObjectID          `json:"id" bson:"_id,omitempty"`
	Date                time.Time              `json:"date" bson:"date"`
	CommodityID         string                 `json:"commodity_id" bson:"commodity_id"`
	CommodityMetadata   CommodityMetadata      `json:"commodity_metadata" bson:"commodity_metadata"`
	ProvincialMetrics   ProvincialMetrics      `json:"provincial_metrics" bson:"provincial_metrics"`
	RegionalBreakdown   []RegionalBreakdownItem `json:"regional_breakdown" bson:"regional_breakdown"`
	OperationalMetrics  OperationalMetrics     `json:"operational_metrics" bson:"operational_metrics"`
	Alerts              []AlertItem            `json:"alerts" bson:"alerts"`
	AIPredictions       *AIPredictions         `json:"ai_predictions,omitempty" bson:"ai_predictions,omitempty"`
	SchemaVersion       int                    `json:"schema_version" bson:"schema_version"`
	CreatedAt           time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at" bson:"updated_at"`
}

// ── FiscalSpaceBucket (Monthly Fiscal & Intervention) ─────────────────
// Bucket keys: period (YYYY-MM) + region_id

type BudgetSummary struct {
	TotalPagu         float64 `json:"total_pagu" bson:"total_pagu"`
	RealizedAmount    float64 `json:"realized_amount" bson:"realized_amount"`
	RealizationPct    float64 `json:"realization_pct" bson:"realization_pct"`
	InterventionScore float64 `json:"intervention_score" bson:"intervention_score"`
}

type ProgramItem struct {
	ProgramName string  `json:"program_name" bson:"program_name"`
	Allocated   float64 `json:"allocated" bson:"allocated"`
	Used        float64 `json:"used" bson:"used"`
}

type FiscalSpaceBucket struct {
	ID             bson.ObjectID  `json:"id" bson:"_id,omitempty"`
	Period         string         `json:"period" bson:"period"`
	RegionID       string         `json:"region_id" bson:"region_id"`
	BudgetSummary  BudgetSummary  `json:"budget_summary" bson:"budget_summary"`
	Programs       []ProgramItem  `json:"programs" bson:"programs"`
	SchemaVersion  int            `json:"schema_version" bson:"schema_version"`
	CreatedAt      time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" bson:"updated_at"`
}

// ── FieldEntry (Transactional - Validation Queue) ────────────────────

type FieldEntryStatus string

const (
	FieldEntryPending    FieldEntryStatus = "MENUNGGU_VALIDASI"
	FieldEntryApproved   FieldEntryStatus = "DIVALIDASI"
	FieldEntryRejected   FieldEntryStatus = "DITOLAK"
)

type FieldEntry struct {
	ID                bson.ObjectID    `json:"id" bson:"_id,omitempty"`
	EntryID           string           `json:"entry_id" bson:"entry_id"`
	CommodityID       string           `json:"commodity_id" bson:"commodity_id"`
	MarketID          string           `json:"market_id" bson:"market_id"`
	RegionID          string           `json:"region_id" bson:"region_id"`
	Price             float64          `json:"price" bson:"price"`
	PriceDeviationPct float64          `json:"price_deviation_pct" bson:"price_deviation_pct"`
	RecordedBy        string           `json:"recorded_by" bson:"recorded_by"`
	RecordedAt        time.Time        `json:"recorded_at" bson:"recorded_at"`
	Status            FieldEntryStatus `json:"status" bson:"status"`
	ValidatedBy       *string          `json:"validated_by,omitempty" bson:"validated_by,omitempty"`
	ValidatedAt       *time.Time       `json:"validated_at,omitempty" bson:"validated_at,omitempty"`
	RejectionReason   *string          `json:"rejection_reason,omitempty" bson:"rejection_reason,omitempty"`
	CreatedAt         time.Time        `json:"created_at" bson:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at" bson:"updated_at"`
}

// ── RegionMaster (Geospatial) ─────────────────────────────────────────

type RegionMaster struct {
	ID        bson.ObjectID `json:"id" bson:"_id,omitempty"`
	RegionID  string        `json:"region_id" bson:"region_id"`
	Name      string        `json:"name" bson:"name"`
	Type      string        `json:"type" bson:"type"`
	GeoJSON   interface{}   `json:"geojson" bson:"geojson"`
	Neighbors []string      `json:"neighbors" bson:"neighbors"`
	CreatedAt time.Time     `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time     `json:"updated_at" bson:"updated_at"`
}

// ── ModelRegistry (ML Ops) ───────────────────────────────────────────

type ModelStatus string

const (
	ModelStatusProduction  ModelStatus = "Produksi"
	ModelStatusCandidate   ModelStatus = "Kandidat"
	ModelStatusArchived    ModelStatus = "Arsip"
)

type ModelMetrics struct {
	Mape    float64 `json:"mape" bson:"mape"`
	DriftPSI float64 `json:"drift_psi" bson:"drift_psi"`
}

type ModelRegistry struct {
	ID               bson.ObjectID `json:"id" bson:"_id,omitempty"`
	CommodityID      string        `json:"commodity_id" bson:"commodity_id"`
	Version          string        `json:"version" bson:"version"`
	Algorithm        string        `json:"algorithm" bson:"algorithm"`
	Metrics          ModelMetrics  `json:"metrics" bson:"metrics"`
	Status           ModelStatus   `json:"status" bson:"status"`
	TrainedAt        time.Time     `json:"trained_at" bson:"trained_at"`
	DeployedAt       *time.Time    `json:"deployed_at,omitempty" bson:"deployed_at,omitempty"`
	FeatureImportance []FeatureImportanceItem `json:"feature_importance,omitempty" bson:"feature_importance,omitempty"`
	CreatedAt        time.Time     `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at" bson:"updated_at"`
}

// ── Collection Names ──────────────────────────────────────────────────

const (
	CollectionCommodityDailyBuckets = "commodity_daily_buckets"
	CollectionFiscalSpaceBuckets    = "fiscal_space_buckets"
	CollectionFieldEntries          = "field_entries"
	CollectionRegionMaster          = "region_master"
	CollectionModelRegistry         = "model_registry"
)
