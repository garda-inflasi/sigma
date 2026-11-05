// Package domain memuat model indikator kinerja (KPI): definisi, target, dan
// nilai periodik yang dihitung job dari data yang sudah ada.
package domain

import "time"

// Kelompok KPI (enum kelompok_kpi).
const (
	KelompokDataSistem    = "DATA_SISTEM"
	KelompokLogistik      = "LOGISTIK"
	KelompokDampakEkonomi = "DAMPAK_EKONOMI"
	KelompokTindakLanjut  = "TINDAK_LANJUT"
)

// Arah baik: NAIK = makin tinggi makin baik, TURUN = makin rendah makin baik.
const (
	ArahNaik  = "NAIK"
	ArahTurun = "TURUN"
)

// Status capaian terhadap target.
const (
	StatusTercapai      = "TERCAPAI"
	StatusWaspada       = "WASPADA"
	StatusTidakTercapai = "TIDAK_TERCAPAI"
	// StatusTanpaTarget: nilai ada tetapi target belum ditetapkan.
	StatusTanpaTarget = "TANPA_TARGET"
	// StatusMenungguData: belum ada nilai (butuh model atau data yang belum ada).
	StatusMenungguData = "MENUNGGU_DATA"
)

// Penghitung otomatis (kolom sumber_query). KPI tanpa penghitung bernilai
// kosong sampai datanya tersedia (KPI berbasis model dan dampak: 16-B).
const (
	QProdusenTerverifikasi = "produsen_terverifikasi"
	QKetepatanData         = "ketepatan_data"
	QKelengkapanHarian     = "kelengkapan_harian"
	QUtilisasiColdStorage  = "utilisasi_cold_storage"
	QRisikoFoodLoss        = "risiko_food_loss"
	QRekomendasiAI         = "rekomendasi_ai_dieksekusi"
)

// Definisi memetakan tabel kpi_definisi.
type Definisi struct {
	Kode        string  `gorm:"column:kode;primaryKey"`
	Kelompok    string  `gorm:"column:kelompok"`
	Nama        string  `gorm:"column:nama"`
	Deskripsi   string  `gorm:"column:deskripsi"`
	Rumus       string  `gorm:"column:rumus"`
	Satuan      string  `gorm:"column:satuan"`
	ArahBaik    string  `gorm:"column:arah_baik"`
	Frekuensi   string  `gorm:"column:frekuensi"`
	SumberQuery *string `gorm:"column:sumber_query"`
	Urutan      int     `gorm:"column:urutan"`
}

func (Definisi) TableName() string { return "kpi_definisi" }

// Target memetakan tabel kpi_target. WilayahKode nil = berlaku untuk semua wilayah.
// Baseline kosong sampai ditetapkan pemangku kepentingan.
type Target struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`
	KpiKode       string    `gorm:"column:kpi_kode"`
	WilayahKode   *string   `gorm:"column:wilayah_kode"`
	PeriodeMulai  time.Time `gorm:"column:periode_mulai;type:date"`
	Target        float64   `gorm:"column:target;type:numeric(14,4)"`
	AmbangWaspada *float64  `gorm:"column:ambang_waspada;type:numeric(14,4)"`
	Baseline      *float64  `gorm:"column:baseline;type:numeric(14,4)"`
}

func (Target) TableName() string { return "kpi_target" }

// Nilai memetakan tabel kpi_nilai. Revisi = baris baru dengan Versi lebih besar.
type Nilai struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	KpiKode      string    `gorm:"column:kpi_kode"`
	WilayahKode  *string   `gorm:"column:wilayah_kode"`
	Periode      time.Time `gorm:"column:periode;type:date"`
	Versi        int       `gorm:"column:versi"`
	Nilai        float64   `gorm:"column:nilai;type:numeric(14,4)"`
	DihitungPada time.Time `gorm:"column:dihitung_pada;default:now()"`
}

func (Nilai) TableName() string { return "kpi_nilai" }

// Jendela adalah rentang waktu satu periode bulanan. Mulai inklusif dan Akhir
// eksklusif (batas bulan menurut WIB); AsOf = min(Akhir, sekarang) untuk ukuran
// berbasis keadaan seperti stok.
type Jendela struct {
	Mulai, Akhir, AsOf time.Time
}

// Indikator adalah satu KPI siap tampil: definisi, target berlaku, nilai terbaru, dan tren.
type Indikator struct {
	Definisi Definisi
	Target   *Target
	// Nilai nil = menunggu data. Periode adalah periode nilai itu.
	Nilai    *Nilai
	Tren     []Nilai
	Status   string
	Otomatis bool
}

// StatusCapaian menilai nilai terhadap target. Ambang waspada (bila ada) adalah
// batas longgar: NAIK ≥ ambang, TURUN ≤ ambang.
func StatusCapaian(arah string, nilai *float64, target *Target) string {
	switch {
	case nilai == nil:
		return StatusMenungguData
	case target == nil:
		return StatusTanpaTarget
	}
	var tercapai, waspada bool
	if arah == ArahTurun {
		tercapai = *nilai <= target.Target
		waspada = target.AmbangWaspada != nil && *nilai <= *target.AmbangWaspada
	} else {
		tercapai = *nilai >= target.Target
		waspada = target.AmbangWaspada != nil && *nilai >= *target.AmbangWaspada
	}
	switch {
	case tercapai:
		return StatusTercapai
	case waspada:
		return StatusWaspada
	default:
		return StatusTidakTercapai
	}
}
