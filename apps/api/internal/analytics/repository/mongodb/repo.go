package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/analytics/domain"
	"github.com/garda-inflasi/api/pkg/logger"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repository struct {
	client *mongo.Client
	dbName string
	log    *logger.Logger
}

func New(client *mongo.Client, log *logger.Logger) *Repository {
	return &Repository{
		client: client,
		dbName: "garda_inflasi",
		log:    log,
	}
}

func (r *Repository) coll(name string) *mongo.Collection {
	return r.client.Database(r.dbName).Collection(name)
}

// Helper to create bson.D elements
func doc(key string, value interface{}) bson.E {
	return bson.E{Key: key, Value: value}
}

// ── Dashboard Summary ─────────────────────────────────────────────────

func (r *Repository) GetDashboardSummary(ctx context.Context) (*domain.DashboardSummary, error) {
	// Get latest date with data
	pipeline := mongo.Pipeline{
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: -1}}}},
		{bson.E{Key: "$limit", Value: 1}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "date", Value: 1},
			bson.E{Key: "commodity_id", Value: 1},
			bson.E{Key: "commodity_metadata", Value: 1},
			bson.E{Key: "provincial_metrics", Value: 1},
			bson.E{Key: "alerts", Value: 1},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate latest bucket: %w", err)
	}
	defer cursor.Close(ctx)

	// Get fiscal space summary
	fiscalPipeline := mongo.Pipeline{
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "period", Value: -1}}}},
		{bson.E{Key: "$limit", Value: 1}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "budget_summary", Value: 1},
		}}},
	}

	var fiscalResult struct {
		BudgetSummary domain.BudgetSummary `bson:"budget_summary"`
	}
	fiscalCursor, err := r.coll(domain.CollectionFiscalSpaceBuckets).Aggregate(ctx, fiscalPipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate fiscal space: %w", err)
	}
	defer fiscalCursor.Close(ctx)

	if fiscalCursor.Next(ctx) {
		if err := fiscalCursor.Decode(&fiscalResult); err != nil {
			return nil, fmt.Errorf("decode fiscal space: %w", err)
		}
	}

	// Count active alerts
	alertCount, err := r.coll(domain.CollectionCommodityDailyBuckets).CountDocuments(ctx, bson.M{
		"alerts": bson.M{"$ne": []interface{}{}},
	})
	if err != nil {
		return nil, fmt.Errorf("count active alerts: %w", err)
	}

	// Get top contributors (commodities with highest price change)
	topContribPipeline := mongo.Pipeline{
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: -1}}}},
		{bson.E{Key: "$limit", Value: 10}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "commodity_id", Value: 1},
			bson.E{Key: "commodity_metadata", Value: 1},
			bson.E{Key: "provincial_metrics.inflation_mom", Value: 1},
		}}},
	}

	type topContribRaw struct {
		CommodityID       string                 `bson:"commodity_id"`
		CommodityMetadata domain.CommodityMetadata `bson:"commodity_metadata"`
		ProvincialMetrics struct {
			InflationMom float64 `bson:"inflation_mom"`
		} `bson:"provincial_metrics"`
	}

	var topContribs []topContribRaw
	topCursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, topContribPipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate top contributors: %w", err)
	}
	defer topCursor.Close(ctx)

	if err := topCursor.All(ctx, &topContribs); err != nil {
		return nil, fmt.Errorf("decode top contributors: %w", err)
	}

	var contributors []domain.TopContributor
	for _, tc := range topContribs {
		contributors = append(contributors, domain.TopContributor{
			CommodityID:   tc.CommodityID,
			CommodityName: tc.CommodityMetadata.Name,
			PriceChangePct: tc.ProvincialMetrics.InflationMom,
		})
	}

	return &domain.DashboardSummary{
		FiscalSpace: &domain.FiscalSpaceSummary{
			TotalPagu:         fiscalResult.BudgetSummary.TotalPagu,
			RealizationPct:    fiscalResult.BudgetSummary.RealizationPct,
			InterventionScore: fiscalResult.BudgetSummary.InterventionScore,
		},
		TopContributors:   contributors,
		ActiveAlertsCount: int(alertCount),
		Period:            time.Now().Format("2006-01"),
	}, nil
}

// ── Inflation Trends ──────────────────────────────────────────────────

func (r *Repository) GetInflationTrends(ctx context.Context, year int, region string) ([]domain.InflationTrend, error) {
	startDate := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(1, 0, 0)

	pipeline := mongo.Pipeline{
		{bson.E{Key: "$match", Value: bson.M{
			"date": bson.M{"$gte": startDate, "$lt": endDate},
		}}},
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: 1}}}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "date", Value: 1},
			bson.E{Key: "commodity_id", Value: 1},
			bson.E{Key: "provincial_metrics", Value: 1},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate inflation trends: %w", err)
	}
	defer cursor.Close(ctx)

	type trendRaw struct {
		Date              time.Time                `bson:"date"`
		CommodityID       string                   `bson:"commodity_id"`
		ProvincialMetrics domain.ProvincialMetrics `bson:"provincial_metrics"`
	}

	var raw []trendRaw
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode inflation trends: %w", err)
	}

	var result []domain.InflationTrend
	for _, r := range raw {
		result = append(result, domain.InflationTrend{
			Date:         r.Date,
			CommodityID:  r.CommodityID,
			AvgPrice:     r.ProvincialMetrics.AvgPrice,
			InflationMoM: r.ProvincialMetrics.InflationMom,
			InflationYoY: r.ProvincialMetrics.InflationYoy,
			Status:       r.ProvincialMetrics.Status,
		})
	}

	return result, nil
}

// ── Food Balance ──────────────────────────────────────────────────────

func (r *Repository) GetFoodBalance(ctx context.Context, period string) ([]domain.FoodBalanceItem, error) {
	pipeline := mongo.Pipeline{
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: -1}}}},
		{bson.E{Key: "$group", Value: bson.D{
			bson.E{Key: "_id", Value: "$commodity_id"},
			bson.E{Key: "commodity_metadata", Value: bson.D{bson.E{Key: "$first", Value: "$commodity_metadata"}}},
			bson.E{Key: "provincial_metrics", Value: bson.D{bson.E{Key: "$first", Value: "$provincial_metrics"}}},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate food balance: %w", err)
	}
	defer cursor.Close(ctx)

	type fbRaw struct {
		CommodityID       string                   `bson:"_id"`
		CommodityMetadata domain.CommodityMetadata  `bson:"commodity_metadata"`
		ProvincialMetrics domain.ProvincialMetrics  `bson:"provincial_metrics"`
	}

	var raw []fbRaw
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode food balance: %w", err)
	}

	var result []domain.FoodBalanceItem
	for _, r := range raw {
		var surplusDeficit float64
		switch r.ProvincialMetrics.Status {
		case "Surplus":
			surplusDeficit = 1.0
		case "Defisit":
			surplusDeficit = -1.0
		default:
			surplusDeficit = 0
		}

		result = append(result, domain.FoodBalanceItem{
			CommodityID:    r.CommodityID,
			CommodityName:  r.CommodityMetadata.Name,
			SurplusDeficit: surplusDeficit,
			Status:         r.ProvincialMetrics.Status,
		})
	}

	return result, nil
}

// ── Fiscal Space ──────────────────────────────────────────────────────

func (r *Repository) GetFiscalSpace(ctx context.Context) (*domain.FiscalSpaceSummary, error) {
	pipeline := mongo.Pipeline{
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "period", Value: -1}}}},
		{bson.E{Key: "$limit", Value: 1}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "budget_summary", Value: 1},
		}}},
	}

	cursor, err := r.coll(domain.CollectionFiscalSpaceBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate fiscal space: %w", err)
	}
	defer cursor.Close(ctx)

	var result struct {
		BudgetSummary domain.BudgetSummary `bson:"budget_summary"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("decode fiscal space: %w", err)
		}
	}

	return &domain.FiscalSpaceSummary{
		TotalPagu:         result.BudgetSummary.TotalPagu,
		RealizationPct:    result.BudgetSummary.RealizationPct,
		InterventionScore: result.BudgetSummary.InterventionScore,
	}, nil
}

// ── Heatmap ───────────────────────────────────────────────────────────

func (r *Repository) GetHeatmap(ctx context.Context, date string) ([]domain.HeatmapItem, error) {
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("parse date: %w", err)
	}

	pipeline := mongo.Pipeline{
		{bson.E{Key: "$match", Value: bson.M{"date": parsedDate}}},
		{bson.E{Key: "$unwind", Value: "$regional_breakdown"}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "date", Value: 1},
			bson.E{Key: "commodity_id", Value: 1},
			bson.E{Key: "commodity_metadata.name", Value: 1},
			bson.E{Key: "provincial_metrics.avg_price", Value: 1},
			bson.E{Key: "provincial_metrics.inflation_mom", Value: 1},
			bson.E{Key: "provincial_metrics.inflation_yoy", Value: 1},
			bson.E{Key: "region_id", Value: "$regional_breakdown.region_id"},
			bson.E{Key: "region_name", Value: "$regional_breakdown.region_name"},
			bson.E{Key: "price", Value: "$regional_breakdown.price"},
			bson.E{Key: "price_change_pct", Value: "$regional_breakdown.price_change_pct"},
			bson.E{Key: "status", Value: "$regional_breakdown.status"},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate heatmap: %w", err)
	}
	defer cursor.Close(ctx)

	type heatmapRaw struct {
		Date           time.Time `bson:"date"`
		CommodityID    string    `bson:"commodity_id"`
		CommodityName  string    `bson:"name"`
		AvgPrice       float64   `bson:"avg_price"`
		InflationMom   float64   `bson:"inflation_mom"`
		InflationYoy   float64   `bson:"inflation_yoy"`
		RegionID       string    `bson:"region_id"`
		RegionName     string    `bson:"region_name"`
		Price          float64   `bson:"price"`
		PriceChangePct float64   `bson:"price_change_pct"`
		Status         string    `bson:"status"`
	}

	var raw []heatmapRaw
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode heatmap: %w", err)
	}

	var result []domain.HeatmapItem
	for _, r := range raw {
		result = append(result, domain.HeatmapItem{
			Date:           r.Date,
			CommodityID:    r.CommodityID,
			CommodityName:  r.CommodityName,
			RegionID:       r.RegionID,
			RegionName:     r.RegionName,
			Price:          r.Price,
			PriceChangePct: r.PriceChangePct,
			Status:         r.Status,
		})
	}

	return result, nil
}

// ── Active Alerts ─────────────────────────────────────────────────────

func (r *Repository) GetActiveAlerts(ctx context.Context) ([]domain.AlertItemView, error) {
	pipeline := mongo.Pipeline{
		{bson.E{Key: "$match", Value: bson.M{
			"alerts": bson.M{"$ne": []interface{}{}},
		}}},
		{bson.E{Key: "$unwind", Value: "$alerts"}},
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "alerts.timestamp", Value: -1}}}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "commodity_id", Value: 1},
			bson.E{Key: "commodity_metadata.name", Value: 1},
			bson.E{Key: "alert_id", Value: "$alerts.alert_id"},
			bson.E{Key: "region_id", Value: "$alerts.region_id"},
			bson.E{Key: "type", Value: "$alerts.type"},
			bson.E{Key: "severity", Value: "$alerts.severity"},
			bson.E{Key: "price_change_pct", Value: "$alerts.price_change_pct"},
			bson.E{Key: "timestamp", Value: "$alerts.timestamp"},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate alerts: %w", err)
	}
	defer cursor.Close(ctx)

	type alertRaw struct {
		CommodityID    string    `bson:"commodity_id"`
		CommodityName  string    `bson:"name"`
		AlertID        string    `bson:"alert_id"`
		RegionID       string    `bson:"region_id"`
		Type           string    `bson:"type"`
		Severity       string    `bson:"severity"`
		PriceChangePct float64   `bson:"price_change_pct"`
		Timestamp      time.Time `bson:"timestamp"`
	}

	var raw []alertRaw
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode alerts: %w", err)
	}

	var result []domain.AlertItemView
	for _, r := range raw {
		result = append(result, domain.AlertItemView{
			AlertID:        r.AlertID,
			CommodityID:    r.CommodityID,
			CommodityName:  r.CommodityName,
			RegionID:       r.RegionID,
			Type:           r.Type,
			Severity:       r.Severity,
			PriceChangePct: r.PriceChangePct,
			Timestamp:      r.Timestamp,
		})
	}

	return result, nil
}

// ── Price Trends ──────────────────────────────────────────────────────

func (r *Repository) GetPriceTrends(ctx context.Context, commodityID, regionID string, days int) ([]domain.PriceTrendItem, error) {
	startDate := time.Now().AddDate(0, 0, -days)

	pipeline := mongo.Pipeline{
		{bson.E{Key: "$match", Value: bson.M{
			"commodity_id": commodityID,
			"date":         bson.M{"$gte": startDate},
		}}},
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: 1}}}},
		{bson.E{Key: "$unwind", Value: "$regional_breakdown"}},
		{bson.E{Key: "$match", Value: bson.M{
			"regional_breakdown.region_id": regionID,
		}}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "date", Value: 1},
			bson.E{Key: "price", Value: "$regional_breakdown.price"},
			bson.E{Key: "price_change_pct", Value: "$regional_breakdown.price_change_pct"},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate price trends: %w", err)
	}
	defer cursor.Close(ctx)

	type trendRaw struct {
		Date           time.Time `bson:"date"`
		Price          float64   `bson:"price"`
		PriceChangePct float64   `bson:"price_change_pct"`
	}

	var raw []trendRaw
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode price trends: %w", err)
	}

	// Calculate 7-day moving average
	var result []domain.PriceTrendItem
	for i, r := range raw {
		avg7Day := 0.0
		count := 0
		for j := i - 6; j <= i; j++ {
			if j >= 0 {
				avg7Day += raw[j].Price
				count++
			}
		}
		if count > 0 {
			avg7Day /= float64(count)
		}

		result = append(result, domain.PriceTrendItem{
			Date:           r.Date,
			Price:          r.Price,
			PriceChangePct: r.PriceChangePct,
			Avg7Day:        avg7Day,
		})
	}

	return result, nil
}

// ── Region Status Map ─────────────────────────────────────────────────

func (r *Repository) GetRegionStatusMap(ctx context.Context) ([]domain.RegionStatusMap, error) {
	// Get latest date's regional breakdown
	pipeline := mongo.Pipeline{
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: -1}}}},
		{bson.E{Key: "$limit", Value: 1}},
		{bson.E{Key: "$unwind", Value: "$regional_breakdown"}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "region_id", Value: "$regional_breakdown.region_id"},
			bson.E{Key: "region_name", Value: "$regional_breakdown.region_name"},
			bson.E{Key: "status", Value: "$regional_breakdown.status"},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate region status: %w", err)
	}
	defer cursor.Close(ctx)

	type regionStatusRaw struct {
		RegionID   string `bson:"region_id"`
		RegionName string `bson:"region_name"`
		Status     string `bson:"status"`
	}

	var raw []regionStatusRaw
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode region status: %w", err)
	}

	// Enrich with GeoJSON from region_master
	var result []domain.RegionStatusMap
	for _, item := range raw {
		var region domain.RegionMaster
		err := r.coll(domain.CollectionRegionMaster).FindOne(ctx, bson.M{"region_id": item.RegionID}).Decode(&region)
		if err != nil {
			continue
		}

		result = append(result, domain.RegionStatusMap{
			RegionID:   item.RegionID,
			RegionName: item.RegionName,
			Status:     item.Status,
			GeoJSON:    region.GeoJSON,
		})
	}

	return result, nil
}

// ── Forecast ──────────────────────────────────────────────────────────

func (r *Repository) GetForecast(ctx context.Context, commodityID string, days int) (*domain.ForecastResult, error) {
	pipeline := mongo.Pipeline{
		{bson.E{Key: "$match", Value: bson.M{"commodity_id": commodityID}}},
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: -1}}}},
		{bson.E{Key: "$limit", Value: 1}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "commodity_id", Value: 1},
			bson.E{Key: "commodity_metadata", Value: 1},
			bson.E{Key: "provincial_metrics.avg_price", Value: 1},
			bson.E{Key: "ai_predictions", Value: 1},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate forecast: %w", err)
	}
	defer cursor.Close(ctx)

	type forecastRaw struct {
		CommodityID       string                 `bson:"commodity_id"`
		CommodityMetadata domain.CommodityMetadata `bson:"commodity_metadata"`
		AvgPrice          float64                `bson:"avg_price"`
		AIPredictions     *domain.AIPredictions  `bson:"ai_predictions"`
	}

	if !cursor.Next(ctx) {
		return nil, fmt.Errorf("no forecast data for commodity: %s", commodityID)
	}

	var raw forecastRaw
	if err := cursor.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode forecast: %w", err)
	}

	if raw.AIPredictions == nil {
		return nil, fmt.Errorf("no AI predictions for commodity: %s", commodityID)
	}

	return &domain.ForecastResult{
		CommodityID:        raw.CommodityID,
		CommodityName:      raw.CommodityMetadata.Name,
		CurrentPrice:       raw.AvgPrice,
		Forecast14dPrice:   raw.AIPredictions.Forecast14dPrice,
		ForecastChangePct:  raw.AIPredictions.ForecastChangePct,
		ConfidenceInterval80: raw.AIPredictions.ConfidenceInterval80,
		Mape:               raw.AIPredictions.Mape,
		ModelVersion:       raw.AIPredictions.ModelVersion,
	}, nil
}

// ── Feature Importance ────────────────────────────────────────────────

func (r *Repository) GetFeatureImportance(ctx context.Context, commodityID string) (*domain.FeatureImportanceView, error) {
	pipeline := mongo.Pipeline{
		{bson.E{Key: "$match", Value: bson.M{"commodity_id": commodityID}}},
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: -1}}}},
		{bson.E{Key: "$limit", Value: 1}},
		{bson.E{Key: "$project", Value: bson.D{
			bson.E{Key: "_id", Value: 0},
			bson.E{Key: "commodity_id", Value: 1},
			bson.E{Key: "ai_predictions.feature_importance", Value: 1},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate feature importance: %w", err)
	}
	defer cursor.Close(ctx)

	type fiRaw struct {
		CommodityID string `bson:"commodity_id"`
		AIPredictions struct {
			FeatureImportance []domain.FeatureImportanceItem `bson:"feature_importance"`
		} `bson:"ai_predictions"`
	}

	if !cursor.Next(ctx) {
		return nil, fmt.Errorf("no feature importance for commodity: %s", commodityID)
	}

	var raw fiRaw
	if err := cursor.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode feature importance: %w", err)
	}

	return &domain.FeatureImportanceView{
		CommodityID: raw.CommodityID,
		Features:    raw.AIPredictions.FeatureImportance,
	}, nil
}

// ── Simulation ────────────────────────────────────────────────────────

func (r *Repository) SimulateScenario(ctx context.Context, req domain.SimulationRequest) (*domain.SimulationResult, error) {
	// Get current price and prediction
	forecast, err := r.GetForecast(ctx, req.CommodityID, 14)
	if err != nil {
		return nil, fmt.Errorf("get forecast for simulation: %w", err)
	}

	// Simple simulation model: adjust forecast based on scenario inputs
	adjustedChange := forecast.ForecastChangePct
	adjustedChange += req.RainfallPct * 0.3       // Rainfall increases price
	adjustedChange -= req.SupplyIncreasePct * 0.5  // Supply decreases price
	adjustedChange -= req.TransportSubsidyPct * 0.2 // Subsidy decreases price

	projectedPrice := forecast.CurrentPrice * (1 + adjustedChange/100)

	return &domain.SimulationResult{
		CommodityID:        req.CommodityID,
		CurrentPrice:       forecast.CurrentPrice,
		ProjectedPrice:     projectedPrice,
		ProjectedChangePct: adjustedChange,
		Scenario:           req,
	}, nil
}

// ── All Commodities Forecast ──────────────────────────────────────────

func (r *Repository) GetAllCommoditiesForecast(ctx context.Context) ([]domain.ForecastResult, error) {
	pipeline := mongo.Pipeline{
		{bson.E{Key: "$sort", Value: bson.D{bson.E{Key: "date", Value: -1}}}},
		{bson.E{Key: "$group", Value: bson.D{
			bson.E{Key: "_id", Value: "$commodity_id"},
			bson.E{Key: "commodity_metadata", Value: bson.D{bson.E{Key: "$first", Value: "$commodity_metadata"}}},
			bson.E{Key: "provincial_metrics", Value: bson.D{bson.E{Key: "$first", Value: "$provincial_metrics"}}},
			bson.E{Key: "ai_predictions", Value: bson.D{bson.E{Key: "$first", Value: "$ai_predictions"}}},
		}}},
	}

	cursor, err := r.coll(domain.CollectionCommodityDailyBuckets).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate all forecasts: %w", err)
	}
	defer cursor.Close(ctx)

	type allForecastRaw struct {
		CommodityID       string                   `bson:"_id"`
		CommodityMetadata domain.CommodityMetadata `bson:"commodity_metadata"`
		ProvincialMetrics domain.ProvincialMetrics `bson:"provincial_metrics"`
		AIPredictions     *domain.AIPredictions    `bson:"ai_predictions"`
	}

	var raw []allForecastRaw
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode all forecasts: %w", err)
	}

	var result []domain.ForecastResult
	for _, r := range raw {
		if r.AIPredictions == nil {
			continue
		}
		result = append(result, domain.ForecastResult{
			CommodityID:        r.CommodityID,
			CommodityName:      r.CommodityMetadata.Name,
			CurrentPrice:       r.ProvincialMetrics.AvgPrice,
			Forecast14dPrice:   r.AIPredictions.Forecast14dPrice,
			ForecastChangePct:  r.AIPredictions.ForecastChangePct,
			ConfidenceInterval80: r.AIPredictions.ConfidenceInterval80,
			Mape:               r.AIPredictions.Mape,
			ModelVersion:       r.AIPredictions.ModelVersion,
		})
	}

	return result, nil
}

// ── Validation Queue ──────────────────────────────────────────────────

func (r *Repository) GetValidationQueue(ctx context.Context, page, limit int) ([]domain.ValidationQueueItem, int64, error) {
	skip := (page - 1) * limit

	filter := bson.M{"status": domain.FieldEntryPending}

	total, err := r.coll(domain.CollectionFieldEntries).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count validation queue: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{bson.E{Key: "recorded_at", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.coll(domain.CollectionFieldEntries).Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("find validation queue: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []domain.FieldEntry
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, 0, fmt.Errorf("decode validation queue: %w", err)
	}

	var result []domain.ValidationQueueItem
	for _, e := range entries {
		result = append(result, domain.ValidationQueueItem{
			EntryID:           e.EntryID,
			CommodityID:       e.CommodityID,
			MarketID:          e.MarketID,
			RegionID:          e.RegionID,
			Price:             e.Price,
			PriceDeviationPct: e.PriceDeviationPct,
			RecordedBy:        e.RecordedBy,
			RecordedAt:        e.RecordedAt,
			Status:            string(e.Status),
		})
	}

	return result, total, nil
}

// ── Batch Approve ─────────────────────────────────────────────────────

func (r *Repository) BatchApprove(ctx context.Context, entryIDs []string, validatedBy string) (int64, error) {
	now := time.Now()

	result, err := r.coll(domain.CollectionFieldEntries).UpdateMany(
		ctx,
		bson.M{
			"entry_id": bson.M{"$in": entryIDs},
			"status":   domain.FieldEntryPending,
		},
		bson.M{
			"$set": bson.M{
				"status":       domain.FieldEntryApproved,
				"validated_by": validatedBy,
				"validated_at": now,
				"updated_at":   now,
			},
		},
	)
	if err != nil {
		return 0, fmt.Errorf("batch approve: %w", err)
	}

	return result.ModifiedCount, nil
}

// ── Field Activity Progress ───────────────────────────────────────────

func (r *Repository) GetFieldActivityProgress(ctx context.Context) ([]domain.FieldActivityProgress, error) {
	today := time.Now().Truncate(24 * time.Hour)

	pipeline := mongo.Pipeline{
		{bson.E{Key: "$match", Value: bson.M{
			"recorded_at": bson.M{"$gte": today},
		}}},
		{bson.E{Key: "$group", Value: bson.D{
			bson.E{Key: "_id", Value: "$region_id"},
			bson.E{Key: "total_entries", Value: bson.D{bson.E{Key: "$sum", Value: 1}}},
			bson.E{Key: "validated", Value: bson.D{bson.E{Key: "$sum", Value: bson.M{"$cond": []interface{}{
				bson.M{"$eq": []interface{}{"$status", domain.FieldEntryApproved}},
				1,
				0,
			}}}}},
			bson.E{Key: "pending", Value: bson.D{bson.E{Key: "$sum", Value: bson.M{"$cond": []interface{}{
				bson.M{"$eq": []interface{}{"$status", domain.FieldEntryPending}},
				1,
				0,
			}}}}},
			bson.E{Key: "rejected", Value: bson.D{bson.E{Key: "$sum", Value: bson.M{"$cond": []interface{}{
				bson.M{"$eq": []interface{}{"$status", domain.FieldEntryRejected}},
				1,
				0,
		}}}}},
		}}},
	}

	cursor, err := r.coll(domain.CollectionFieldEntries).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate field progress: %w", err)
	}
	defer cursor.Close(ctx)

	type progressRaw struct {
		RegionID     string `bson:"_id"`
		TotalEntries int    `bson:"total_entries"`
		Validated    int    `bson:"validated"`
		Pending      int    `bson:"pending"`
		Rejected     int    `bson:"rejected"`
	}

	var raw []progressRaw
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode field progress: %w", err)
	}

	// Enrich with region names
	var result []domain.FieldActivityProgress
	for _, item := range raw {
		var region domain.RegionMaster
		err := r.coll(domain.CollectionRegionMaster).FindOne(ctx, bson.M{"region_id": item.RegionID}).Decode(&region)
		regionName := item.RegionID
		if err == nil {
			regionName = region.Name
		}

		result = append(result, domain.FieldActivityProgress{
			RegionID:     item.RegionID,
			RegionName:   regionName,
			TotalEntries: item.TotalEntries,
			Validated:    item.Validated,
			Pending:      item.Pending,
			Rejected:     item.Rejected,
			Date:         today.Format("2006-01-02"),
		})
	}

	return result, nil
}

// ── Model Registry ────────────────────────────────────────────────────

func (r *Repository) GetModelRegistry(ctx context.Context) ([]domain.ModelRegistryView, error) {
	opts := options.Find().SetSort(bson.D{bson.E{Key: "trained_at", Value: -1}})

	cursor, err := r.coll(domain.CollectionModelRegistry).Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find model registry: %w", err)
	}
	defer cursor.Close(ctx)

	var models []domain.ModelRegistry
	if err := cursor.All(ctx, &models); err != nil {
		return nil, fmt.Errorf("decode model registry: %w", err)
	}

	var result []domain.ModelRegistryView
	for _, m := range models {
		result = append(result, domain.ModelRegistryView{
			CommodityID: m.CommodityID,
			Version:     m.Version,
			Algorithm:   m.Algorithm,
			Metrics:     m.Metrics,
			Status:      string(m.Status),
			TrainedAt:   m.TrainedAt,
			DeployedAt:  m.DeployedAt,
		})
	}

	return result, nil
}
