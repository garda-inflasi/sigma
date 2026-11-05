package dto

import "github.com/garda-inflasi/api/internal/dashboard/domain"

// ── Operasional Responses ─────────────────────────────────────────────

// KpiResponse adalah satu kartu KPI operasional.
type KpiResponse struct {
	ID              string    `json:"id"`
	Label           string    `json:"label"`
	Nilai           float64   `json:"nilai"`
	Format          string    `json:"format"`
	Delta           float64   `json:"delta"`
	KeteranganDelta string    `json:"keterangan_delta"`
	Tren            []float64 `json:"tren"`
	Tone            string    `json:"tone"`
	Href            string    `json:"href"`
}

type KabupatenResponse struct {
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

type KomoditasResponse struct {
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
	Pilot bool   `json:"pilot"`
}

// SelHeatmapResponse: Delta adalah perubahan harga konsumen (persen) terhadap rata-rata 7 hari sebelumnya.
type SelHeatmapResponse struct {
	KomoditasKode string  `json:"komoditas_kode"`
	KabupatenKode string  `json:"kabupaten_kode"`
	Delta         float64 `json:"delta"`
	Harga         float64 `json:"harga"`
}

type HeatmapResponse struct {
	Kabupaten []KabupatenResponse  `json:"kabupaten"`
	Komoditas []KomoditasResponse  `json:"komoditas"`
	Sel       []SelHeatmapResponse `json:"sel"`
}

type AktivitasLapanganResponse struct {
	KabupatenKode string `json:"kabupaten_kode"`
	KabupatenNama string `json:"kabupaten_nama"`
	JumlahPetugas int    `json:"jumlah_petugas"`
	Entri         int    `json:"entri"`
	Target        int    `json:"target"`
}

func ToKpiResponses(kpis []domain.Kpi) []KpiResponse {
	out := make([]KpiResponse, len(kpis))
	for i, k := range kpis {
		out[i] = KpiResponse{
			ID: k.ID, Label: k.Label, Nilai: k.Nilai, Format: k.Format, Delta: k.Delta,
			KeteranganDelta: k.KeteranganDelta, Tren: k.Tren, Tone: k.Tone, Href: k.Href,
		}
	}
	return out
}

func ToHeatmapResponse(h *domain.HeatmapData) HeatmapResponse {
	res := HeatmapResponse{
		Kabupaten: make([]KabupatenResponse, len(h.Kabupaten)),
		Komoditas: make([]KomoditasResponse, len(h.Komoditas)),
		Sel:       make([]SelHeatmapResponse, len(h.Sel)),
	}
	for i, k := range h.Kabupaten {
		res.Kabupaten[i] = KabupatenResponse{Kode: k.Kode, Nama: k.Nama}
	}
	for i, k := range h.Komoditas {
		res.Komoditas[i] = KomoditasResponse{Kode: k.Kode, Nama: k.Nama, Pilot: k.Pilot}
	}
	for i, s := range h.Sel {
		res.Sel[i] = SelHeatmapResponse{KomoditasKode: s.KomoditasKode, KabupatenKode: s.KabupatenKode, Delta: s.Delta, Harga: s.Harga}
	}
	return res
}

func ToAktivitasLapanganResponses(items []domain.AktivitasKabupaten) []AktivitasLapanganResponse {
	out := make([]AktivitasLapanganResponse, len(items))
	for i, a := range items {
		out[i] = AktivitasLapanganResponse{
			KabupatenKode: a.KabupatenKode, KabupatenNama: a.KabupatenNama,
			JumlahPetugas: a.JumlahPetugas, Entri: a.Entri, Target: a.Target,
		}
	}
	return out
}
