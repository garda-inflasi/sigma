package usecase

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/analytics/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

type UseCase struct {
	repo domain.AnalyticsRepository
	log  *logger.Logger
}

func New(repo domain.AnalyticsRepository, log *logger.Logger) *UseCase {
	return &UseCase{
		repo: repo,
		log:  log,
	}
}

// ── Dashboard Summary ─────────────────────────────────────────────────

func (uc *UseCase) GetDashboardSummary(ctx context.Context) (*domain.DashboardSummary, error) {
	data, err := uc.repo.GetDashboardSummary(ctx)
	if err != nil {
		uc.log.Error("analytics: get dashboard summary failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get dashboard summary: %w", err)
	}
	return data, nil
}

// ── Inflation Trends ──────────────────────────────────────────────────

func (uc *UseCase) GetInflationTrends(ctx context.Context, year int, region string) ([]domain.InflationTrend, error) {
	if year == 0 {
		year = 2026
	}
	if region == "" {
		region = "aceh"
	}

	data, err := uc.repo.GetInflationTrends(ctx, year, region)
	if err != nil {
		uc.log.Error("analytics: get inflation trends failed", map[string]interface{}{"error": err.Error(), "year": year, "region": region})
		return nil, fmt.Errorf("get inflation trends: %w", err)
	}
	return data, nil
}

// ── Food Balance ──────────────────────────────────────────────────────

func (uc *UseCase) GetFoodBalance(ctx context.Context, period string) ([]domain.FoodBalanceItem, error) {
	if period == "" {
		period = "monthly"
	}

	data, err := uc.repo.GetFoodBalance(ctx, period)
	if err != nil {
		uc.log.Error("analytics: get food balance failed", map[string]interface{}{"error": err.Error(), "period": period})
		return nil, fmt.Errorf("get food balance: %w", err)
	}
	return data, nil
}

// ── Fiscal Space ──────────────────────────────────────────────────────

func (uc *UseCase) GetFiscalSpace(ctx context.Context) (*domain.FiscalSpaceSummary, error) {
	data, err := uc.repo.GetFiscalSpace(ctx)
	if err != nil {
		uc.log.Error("analytics: get fiscal space failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get fiscal space: %w", err)
	}
	return data, nil
}

// ── Heatmap ───────────────────────────────────────────────────────────

func (uc *UseCase) GetHeatmap(ctx context.Context, date string) ([]domain.HeatmapItem, error) {
	if date == "" {
		date = "2026-10-04"
	}

	data, err := uc.repo.GetHeatmap(ctx, date)
	if err != nil {
		uc.log.Error("analytics: get heatmap failed", map[string]interface{}{"error": err.Error(), "date": date})
		return nil, fmt.Errorf("get heatmap: %w", err)
	}
	return data, nil
}

// ── Active Alerts ─────────────────────────────────────────────────────

func (uc *UseCase) GetActiveAlerts(ctx context.Context) ([]domain.AlertItemView, error) {
	data, err := uc.repo.GetActiveAlerts(ctx)
	if err != nil {
		uc.log.Error("analytics: get active alerts failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get active alerts: %w", err)
	}
	return data, nil
}

// ── Price Trends ──────────────────────────────────────────────────────

func (uc *UseCase) GetPriceTrends(ctx context.Context, commodityID, regionID string, days int) ([]domain.PriceTrendItem, error) {
	if commodityID == "" {
		commodityID = "cabai_merah_keriting"
	}
	if regionID == "" {
		regionID = "banda_aceh"
	}
	if days <= 0 {
		days = 45
	}

	data, err := uc.repo.GetPriceTrends(ctx, commodityID, regionID, days)
	if err != nil {
		uc.log.Error("analytics: get price trends failed", map[string]interface{}{"error": err.Error(), "commodity": commodityID, "region": regionID, "days": days})
		return nil, fmt.Errorf("get price trends: %w", err)
	}
	return data, nil
}

// ── Region Status Map ─────────────────────────────────────────────────

func (uc *UseCase) GetRegionStatusMap(ctx context.Context) ([]domain.RegionStatusMap, error) {
	data, err := uc.repo.GetRegionStatusMap(ctx)
	if err != nil {
		uc.log.Error("analytics: get region status map failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get region status map: %w", err)
	}
	return data, nil
}

// ── Forecast ──────────────────────────────────────────────────────────

func (uc *UseCase) GetForecast(ctx context.Context, commodityID string, days int) (*domain.ForecastResult, error) {
	if commodityID == "" {
		commodityID = "cabai_merah_keriting"
	}
	if days <= 0 {
		days = 14
	}

	data, err := uc.repo.GetForecast(ctx, commodityID, days)
	if err != nil {
		uc.log.Error("analytics: get forecast failed", map[string]interface{}{"error": err.Error(), "commodity": commodityID, "days": days})
		return nil, fmt.Errorf("get forecast: %w", err)
	}
	return data, nil
}

// ── Feature Importance ────────────────────────────────────────────────

func (uc *UseCase) GetFeatureImportance(ctx context.Context, commodityID string) (*domain.FeatureImportanceView, error) {
	if commodityID == "" {
		commodityID = "cabai_merah_keriting"
	}

	data, err := uc.repo.GetFeatureImportance(ctx, commodityID)
	if err != nil {
		uc.log.Error("analytics: get feature importance failed", map[string]interface{}{"error": err.Error(), "commodity": commodityID})
		return nil, fmt.Errorf("get feature importance: %w", err)
	}
	return data, nil
}

// ── Simulation ────────────────────────────────────────────────────────

func (uc *UseCase) SimulateScenario(ctx context.Context, req domain.SimulationRequest) (*domain.SimulationResult, error) {
	data, err := uc.repo.SimulateScenario(ctx, req)
	if err != nil {
		uc.log.Error("analytics: simulate scenario failed", map[string]interface{}{"error": err.Error(), "commodity": req.CommodityID})
		return nil, fmt.Errorf("simulate scenario: %w", err)
	}
	return data, nil
}

// ── All Commodities Forecast ──────────────────────────────────────────

func (uc *UseCase) GetAllCommoditiesForecast(ctx context.Context) ([]domain.ForecastResult, error) {
	data, err := uc.repo.GetAllCommoditiesForecast(ctx)
	if err != nil {
		uc.log.Error("analytics: get all commodities forecast failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get all commodities forecast: %w", err)
	}
	return data, nil
}

// ── Validation Queue ──────────────────────────────────────────────────

func (uc *UseCase) GetValidationQueue(ctx context.Context, page, limit int) ([]domain.ValidationQueueItem, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 8
	}

	items, total, err := uc.repo.GetValidationQueue(ctx, page, limit)
	if err != nil {
		uc.log.Error("analytics: get validation queue failed", map[string]interface{}{"error": err.Error(), "page": page, "limit": limit})
		return nil, 0, fmt.Errorf("get validation queue: %w", err)
	}
	return items, total, nil
}

// ── Batch Approve ─────────────────────────────────────────────────────

func (uc *UseCase) BatchApprove(ctx context.Context, entryIDs []string, validatedBy string) (int64, error) {
	if len(entryIDs) == 0 {
		return 0, fmt.Errorf("entry_ids is required")
	}

	count, err := uc.repo.BatchApprove(ctx, entryIDs, validatedBy)
	if err != nil {
		uc.log.Error("analytics: batch approve failed", map[string]interface{}{"error": err.Error(), "count": len(entryIDs)})
		return 0, fmt.Errorf("batch approve: %w", err)
	}
	return count, nil
}

// ── Field Activity Progress ───────────────────────────────────────────

func (uc *UseCase) GetFieldActivityProgress(ctx context.Context) ([]domain.FieldActivityProgress, error) {
	data, err := uc.repo.GetFieldActivityProgress(ctx)
	if err != nil {
		uc.log.Error("analytics: get field activity progress failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get field activity progress: %w", err)
	}
	return data, nil
}

// ── Model Registry ────────────────────────────────────────────────────

func (uc *UseCase) GetModelRegistry(ctx context.Context) ([]domain.ModelRegistryView, error) {
	data, err := uc.repo.GetModelRegistry(ctx)
	if err != nil {
		uc.log.Error("analytics: get model registry failed", map[string]interface{}{"error": err.Error()})
		return nil, fmt.Errorf("get model registry: %w", err)
	}
	return data, nil
}
