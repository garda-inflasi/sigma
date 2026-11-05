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
	// Sumber: LAPANGAN (entri tervalidasi) atau EKSTERNAL (hasil ingestion, bila belum ada data lapangan).
	Sumber string `json:"sumber"`
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
		res.Sel[i] = SelHeatmapResponse{KomoditasKode: s.KomoditasKode, KabupatenKode: s.KabupatenKode, Delta: s.Delta, Harga: s.Harga, Sumber: s.Sumber}
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

// PembandingHargaItem membandingkan harga lapangan dan eksternal pada hari yang sama.
// SelisihPct dalam persen terhadap harga eksternal; Demo benar bila sisi eksternal adalah contoh.
type PembandingHargaItem struct {
	KomoditasKode  string  `json:"komoditas_kode"`
	KomoditasNama  string  `json:"komoditas_nama"`
	KabupatenKode  string  `json:"kabupaten_kode"`
	KabupatenNama  string  `json:"kabupaten_nama"`
	Tanggal        string  `json:"tanggal"`
	HargaLapangan  float64 `json:"harga_lapangan"`
	HargaEksternal float64 `json:"harga_eksternal"`
	SelisihPct     float64 `json:"selisih_pct"`
	Sepakat        bool    `json:"sepakat"`
	Demo           bool    `json:"demo"`
}

// PembandingHargaResponse: Pasangan dan Sepakat dihitung atas semua pasangan, Items hanya
// yang selisihnya terbesar. BatasPct adalah ambang sepakat (persen).
type PembandingHargaResponse struct {
	Pasangan int                   `json:"pasangan"`
	Sepakat  int                   `json:"sepakat"`
	BatasPct float64               `json:"batas_pct"`
	Items    []PembandingHargaItem `json:"items"`
}

func ToPembandingHargaResponse(p *domain.Pembanding) PembandingHargaResponse {
	res := PembandingHargaResponse{Pasangan: p.Pasangan, Sepakat: p.Sepakat, BatasPct: domain.BatasSelisihPct, Items: make([]PembandingHargaItem, len(p.Items))}
	for i, it := range p.Items {
		res.Items[i] = PembandingHargaItem{
			KomoditasKode: it.KomoditasKode, KomoditasNama: it.KomoditasNama, KabupatenKode: it.KabupatenKode, KabupatenNama: it.KabupatenNama,
			Tanggal: it.Tanggal.Format("2006-01-02"), HargaLapangan: it.HargaLapangan, HargaEksternal: it.HargaEksternal,
			SelisihPct: it.SelisihPct, Sepakat: it.Sepakat(), Demo: it.Demo,
		}
	}
	return res
}

// CuacaResponse adalah cuaca harian terbaru satu kabupaten/kota. Anomali = hujan lebat (>= 50 mm/hari).
type CuacaResponse struct {
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
	Tanggal       string  `json:"tanggal"`
	CurahHujanMm  float64 `json:"curah_hujan_mm"`
	SuhuC         float64 `json:"suhu_c"`
	Kelembapan    int     `json:"kelembapan"`
	Anomali       bool    `json:"anomali"`
	Sumber        string  `json:"sumber"`
	Demo          bool    `json:"demo"`
}

func ToCuacaResponses(items []domain.CuacaKabupaten) []CuacaResponse {
	out := make([]CuacaResponse, len(items))
	for i, c := range items {
		out[i] = CuacaResponse{
			KabupatenKode: c.KabupatenKode, KabupatenNama: c.KabupatenNama, Tanggal: c.Tanggal.Format("2006-01-02"),
			CurahHujanMm: c.CurahHujanMm, SuhuC: c.SuhuC, Kelembapan: c.Kelembapan, Anomali: c.Anomali, Sumber: c.Sumber, Demo: c.Demo,
		}
	}
	return out
}
