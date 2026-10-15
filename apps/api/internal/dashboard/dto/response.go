package dto

import "github.com/garda-inflasi/api/internal/dashboard/domain"

// ── Operasional Responses ─────────────────────────────────────────────

type KpiOperasionalResponse struct {
	Items []domain.KpiOperasional `json:"items"`
}

type HeatmapResponse struct {
	Kabupaten []domain.Kabupaten `json:"kabupaten"`
	Komoditas []domain.Komoditas `json:"komoditas"`
	Sel       []domain.SelHeatmap `json:"sel"`
}

// ── Eksekutif Responses ───────────────────────────────────────────────

type RingkasanEksekutifResponse struct {
	Data *domain.RingkasanEksekutif `json:"data"`
}

