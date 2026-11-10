package domain

import (
	"time"

	ingestion "github.com/garda-inflasi/api/internal/ingestion/domain"
)

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

// TitikKpi adalah pembilang dan penyebut satu indikator pada satu hari (waktu Aceh), sudah dijumlahkan atas semua
// kabupaten dan komoditas yang lolos cakupan dan filter. Nilai kartu SELALU dihitung dari keduanya (lihat
// usecase.RakitKpi), tidak pernah dengan merata-ratakan nilai jadi, supaya persentase gabungan tepat:
//   - entri:    Pembilang jumlah entri hari itu; Penyebut tak dipakai.
//   - validasi: Pembilang entri tervalidasi, Penyebut seluruh entri, harian (jendela 7 hari disusun usecase).
//   - waspada:  Pembilang jumlah komoditas berbeda yang melampaui ambang; Penyebut tak dipakai.
//   - cs:       Pembilang volume stok (ton), Penyebut kapasitas fasilitas operasional (ton). Di view, kapasitas
//     adalah baris tersendiri per kabupaten dengan komoditas_kode kosong (kolom itu ikut kunci unik, jadi tidak boleh NULL).
//
// Hari tanpa baris berarti nol (tak ada entri, stok, atau kapasitas); usecase mengisinya.
type TitikKpi struct {
	Indikator string    `gorm:"column:indikator"`
	Tanggal   time.Time `gorm:"column:tanggal"`
	Pembilang float64   `gorm:"column:pembilang"`
	Penyebut  float64   `gorm:"column:penyebut"`
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

// Sumber sel heatmap.
const (
	// SumberLapangan: harga entri lapangan yang sudah tervalidasi (mv_heatmap_harga).
	SumberLapangan = "LAPANGAN"
	// SumberEksternal: harga hasil ingestion (mis. PIHPS) untuk sel yang belum punya data lapangan.
	SumberEksternal = "EKSTERNAL"
)

// SelHeatmap adalah harga konsumen terakhir dan perubahannya (persen) terhadap rata-rata 7 hari sebelumnya.
type SelHeatmap struct {
	KomoditasKode string  `gorm:"column:komoditas_kode"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	Delta         float64 `gorm:"column:delta"`
	Harga         float64 `gorm:"column:harga"`
	Sumber        string  `gorm:"column:sumber"`
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

// ── Validasi silang harga ─────────────────────────────────────────────

// BatasSelisihPct adalah selisih harga lapangan vs eksternal yang masih dianggap sepakat
// (sama dengan ukuran akurasi di pipeline sumber data).
const BatasSelisihPct = ingestion.BatasSelisihPct

// PembandingHarga membandingkan harga konsumen lapangan dengan harga eksternal pada hari yang
// sama untuk satu komoditas di satu kabupaten/kota (gold_harga_harian). Selisih dalam persen
// terhadap harga eksternal. Demo benar bila sisi eksternalnya berasal dari eksekusi contoh.
type PembandingHarga struct {
	KomoditasKode  string    `gorm:"column:komoditas_kode"`
	KomoditasNama  string    `gorm:"column:komoditas_nama"`
	KabupatenKode  string    `gorm:"column:kabupaten_kode"`
	KabupatenNama  string    `gorm:"column:kabupaten_nama"`
	Tanggal        time.Time `gorm:"column:tanggal"`
	HargaLapangan  float64   `gorm:"column:harga_lapangan"`
	HargaEksternal float64   `gorm:"column:harga_eksternal"`
	SelisihPct     float64   `gorm:"column:selisih_pct"`
	Demo           bool      `gorm:"column:demo"`
}

// Sepakat benar bila selisihnya dalam BatasSelisihPct.
func (p PembandingHarga) Sepakat() bool {
	return p.SelisihPct <= BatasSelisihPct && p.SelisihPct >= -BatasSelisihPct
}

// Pembanding adalah hasil validasi silang: Pasangan dan Sepakat dihitung atas semua pasangan,
// Items hanya yang selisihnya terbesar.
type Pembanding struct {
	Items    []PembandingHarga
	Pasangan int
	Sepakat  int
}

// ── Cuaca ─────────────────────────────────────────────────────────────

// CuacaKabupaten adalah cuaca harian terbaru satu kabupaten/kota (cuaca_harian, hasil ingestion).
type CuacaKabupaten struct {
	KabupatenKode string    `gorm:"column:kabupaten_kode"`
	KabupatenNama string    `gorm:"column:kabupaten_nama"`
	Tanggal       time.Time `gorm:"column:tanggal"`
	CurahHujanMm  float64   `gorm:"column:curah_hujan_mm"`
	SuhuC         float64   `gorm:"column:suhu_c"`
	Kelembapan    int       `gorm:"column:kelembapan"`
	Anomali       bool      `gorm:"column:anomali"`
	Sumber        string    `gorm:"column:sumber"`
	Demo          bool      `gorm:"column:demo"`
}
