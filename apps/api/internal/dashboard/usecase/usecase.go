package usecase

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/pkg/logger"
)

type UseCase struct {
	repo domain.Repository
	log  *logger.Logger
}

func New(repo domain.Repository, log *logger.Logger) *UseCase {
	return &UseCase{
		repo: repo,
		log:  log,
	}
}

// ── Operasional ───────────────────────────────────────────────────────

func (uc *UseCase) GetKpiOperasional(ctx context.Context, periode string) ([]domain.KpiOperasional, error) {
	if periode == "" {
		periode = "bulan_ini"
	}

	items, err := uc.repo.GetKpiOperasional(ctx, periode)
	if err != nil {
		uc.log.Error("dashboard: get kpi operasional failed", map[string]interface{}{"error": err.Error(), "periode": periode})
		return nil, fmt.Errorf("get kpi operasional: %w", err)
	}

	return items, nil
}

func (uc *UseCase) GetHeatmap(ctx context.Context, periode string) (*domain.HeatmapData, error) {
	if periode == "" {
		periode = "bulan_ini"
	}

	data, err := uc.repo.GetHeatmapData(ctx, periode)
	if err != nil {
		uc.log.Error("dashboard: get heatmap failed", map[string]interface{}{"error": err.Error(), "periode": periode})
		return nil, fmt.Errorf("get heatmap: %w", err)
	}

	return data, nil
}

// ── Eksekutif ─────────────────────────────────────────────────────────

func (uc *UseCase) GetRingkasanEksekutif(ctx context.Context, periode string) (*domain.RingkasanEksekutif, error) {
	if periode == "" {
		periode = "bulan_ini"
	}

	data, err := uc.repo.GetRingkasanEksekutif(ctx, periode)
	if err != nil {
		uc.log.Error("dashboard: get ringkasan eksekutif failed", map[string]interface{}{"error": err.Error(), "periode": periode})
		return nil, fmt.Errorf("get ringkasan eksekutif: %w", err)
	}

	return data, nil
}

