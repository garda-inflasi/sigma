package dto

// ── Operasional Requests ──────────────────────────────────────────────

type KpiOperasionalRequest struct {
	Periode string `form:"periode"`
}

type HeatmapRequest struct {
	Periode string `form:"periode"`
}

