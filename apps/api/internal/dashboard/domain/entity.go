package domain

import "time"

// ── KPI Operasional ───────────────────────────────────────────────────

// Indikator KPI operasional (kolom indikator pada mv_kpi_operasional dan target_kpi).
const (
	IndikatorEntri    = "entri"
	IndikatorValidasi = "validasi"
	IndikatorWaspada  = "waspada"
	IndikatorCS       = "cs"
)

// Arah target KPI.
const (
	ArahNaik  = "NAIK"
	ArahTurun = "TURUN"
)

// TitikKpi adalah nilai satu indikator pada satu hari (waktu Aceh).
type TitikKpi struct {
	Indikator string    `gorm:"column:indikator"`
	Tanggal   time.Time `gorm:"column:tanggal"`
	Nilai     float64   `gorm:"column:nilai"`
}

// TargetKpi adalah target indikator yang berlaku.
type TargetKpi struct {
	Indikator string  `gorm:"column:indikator"`
	Target    float64 `gorm:"column:target"`
	Arah      string  `gorm:"column:arah"`
}

// Kpi adalah satu kartu KPI operasional.
type Kpi struct {
	ID              string
	Label           string
	Nilai           float64
	Format          string // angka | persen
	Delta           float64
	KeteranganDelta string
	Tren            []float64
	Tone            string // data | normal | waspada | defisit
	Href            string
}

// ── Heatmap ───────────────────────────────────────────────────────────

type Kabupaten struct {
	Kode string `gorm:"column:kode"`
	Nama string `gorm:"column:nama"`
}

type Komoditas struct {
	Kode  string `gorm:"column:kode"`
	Nama  string `gorm:"column:nama"`
	Pilot bool   `gorm:"column:pilot"`
}

// SelHeatmap adalah harga konsumen terakhir dan perubahannya (persen) terhadap rata-rata 7 hari sebelumnya.
type SelHeatmap struct {
	KomoditasKode string  `gorm:"column:komoditas_kode"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	Delta         float64 `gorm:"column:delta"`
	Harga         float64 `gorm:"column:harga"`
}

type HeatmapData struct {
	Kabupaten []Kabupaten
	Komoditas []Komoditas
	Sel       []SelHeatmap
}

// ── Aktivitas lapangan ────────────────────────────────────────────────

// AktivitasKabupaten membandingkan entri harga konsumen hari ini dengan target
// harian: tiap petugas diharapkan mencatat tiap komoditas aktif sekali sehari.
type AktivitasKabupaten struct {
	KabupatenKode string `gorm:"column:kabupaten_kode"`
	KabupatenNama string `gorm:"column:kabupaten_nama"`
	JumlahPetugas int    `gorm:"column:jumlah_petugas"`
	Entri         int    `gorm:"column:entri"`
	Target        int    `gorm:"column:target"`
}
