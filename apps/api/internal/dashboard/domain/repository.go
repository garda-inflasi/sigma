package domain

import "context"

type Repository interface {
	// Operasional
	GetKpiOperasional(ctx context.Context, periode string) ([]KpiOperasional, error)
	GetHeatmapData(ctx context.Context, periode string) (*HeatmapData, error)

	// Eksekutif
	GetRingkasanEksekutif(ctx context.Context, periode string) (*RingkasanEksekutif, error)
}

