package dto

// ── Operasional Requests ──────────────────────────────────────────────

type KpiOperasionalRequest struct {
	Periode string `form:"periode"`
}

type HeatmapRequest struct {
	Periode string `form:"periode"`
}

// ── Eksekutif Requests ────────────────────────────────────────────────

type RingkasanEksekutifRequest struct {
	Periode string `form:"periode"`
}

