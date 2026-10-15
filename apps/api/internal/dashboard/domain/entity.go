package domain

// ── KPI Operasional ───────────────────────────────────────────────────

type KpiOperasional struct {
	ID          string  `json:"id"`
	Indikator   string  `json:"indikator"`
	Nilai       float64 `json:"nilai"`
	Target      float64 `json:"target"`
	CapaianPct  float64 `json:"capaian_pct"`
	Status      string  `json:"status"`
	Periode     string  `json:"periode"`
	KabupatenKode string `json:"kabupaten_kode,omitempty"`
	KabupatenNama string `json:"kabupaten_nama,omitempty"`
}

// ── Heatmap ───────────────────────────────────────────────────────────

type Kabupaten struct {
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

type Komoditas struct {
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
	Pilot bool   `json:"pilot"`
}

type SelHeatmap struct {
	KabupatenKode string  `json:"kabupaten_kode"`
	KomoditasKode string  `json:"komoditas_kode"`
	Nilai         float64 `json:"nilai"`
	Kategori      string  `json:"kategori"`
}

type HeatmapData struct {
	Kabupaten []Kabupaten `json:"kabupaten"`
	Komoditas []Komoditas `json:"komoditas"`
	Sel       []SelHeatmap `json:"sel"`
}

// ── Ringkasan Eksekutif ───────────────────────────────────────────────

type RingkasanEksekutif struct {
	InflasiTahunan      float64              `json:"inflasi_tahunan"`
	InflasiBulanan      float64              `json:"inflasi_bulanan"`
	IHK                 float64              `json:"ihk"`
	NTP                 *float64             `json:"ntp,omitempty"`
	Periode             string               `json:"periode"`
	KomoditasPenyumbang []KomoditasSumbangan `json:"komoditas_penyumbang"`
	DaerahTertinggi     []DaerahInflasi      `json:"daerah_tertinggi"`
	DaerahTerendah      []DaerahInflasi      `json:"daerah_terendah"`
	Tren                []TrenBulanan        `json:"tren"`
}

type KomoditasSumbangan struct {
	Kode       string  `json:"kode"`
	Nama       string  `json:"nama"`
	Andil      float64 `json:"andil"`
	InflasiPct float64 `json:"inflasi_pct"`
}

type DaerahInflasi struct {
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
	InflasiPct    float64 `json:"inflasi_pct"`
}

type TrenBulanan struct {
	Bulan     string  `json:"bulan"`
	InflasiPct float64 `json:"inflasi_pct"`
	IHK       float64 `json:"ihk"`
}

