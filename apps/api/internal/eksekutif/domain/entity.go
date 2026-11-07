// Package domain memuat model dashboard eksekutif: inflasi bulanan, andil
// komoditas, neraca pangan, target inflasi, dan ruang fiskal.
//
// Setiap angka membawa Sumber. Angka resmi hanya masuk lewat impor (09-B);
// SumberDemo menandai data contoh yang wajib diberi label "demo" di respons dan UI.
package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// Sumber angka.
const (
	SumberBPS     = "BPS"
	SumberBI      = "BI"
	SumberDJPB    = "DJPB"
	SumberBapanas = "BAPANAS"
	SumberDemo    = "DEMO"
)

// WilayahProvinsi adalah kode BPS Provinsi Aceh, wilayah yang ditampilkan dashboard eksekutif.
const WilayahProvinsi = "11"

// InflasiBulanan memetakan tabel inflasi_bulanan. Periode selalu tanggal 1. PK-nya ID BIGINT identity;
// kunci alami (wilayah_kode, periode) adalah UNIQUE dan target upsert impor.
type InflasiBulanan struct {
	ID          idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	WilayahKode string    `gorm:"column:wilayah_kode"`
	Periode     time.Time `gorm:"column:periode;type:date"`
	IHK         float64   `gorm:"column:ihk;type:numeric(8,3)"`
	InflasiMtm  float64   `gorm:"column:inflasi_mtm;type:numeric(6,2)"`
	InflasiYoy  float64   `gorm:"column:inflasi_yoy;type:numeric(6,2)"`
	InflasiYtd  float64   `gorm:"column:inflasi_ytd;type:numeric(6,2)"`
	Sumber      string    `gorm:"column:sumber"`
	DiimporPada time.Time `gorm:"column:diimpor_pada;default:now()"`
}

func (InflasiBulanan) TableName() string { return "inflasi_bulanan" }

// AndilInflasi memetakan tabel andil_inflasi: sumbangan satu komoditas (poin persen). PK-nya ID BIGINT
// identity; kunci alami (periode, wilayah_kode, komoditas_kode) adalah UNIQUE dan target upsert impor.
type AndilInflasi struct {
	ID            idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	Periode       time.Time `gorm:"column:periode;type:date"`
	WilayahKode   string    `gorm:"column:wilayah_kode"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	Andil         float64   `gorm:"column:andil;type:numeric(6,3)"`
	InflasiPct    float64   `gorm:"column:inflasi_pct;type:numeric(6,2)"`
	Sumber        string    `gorm:"column:sumber"`
	DiimporPada   time.Time `gorm:"column:diimpor_pada;default:now()"`
}

func (AndilInflasi) TableName() string { return "andil_inflasi" }

// NeracaPangan memetakan tabel neraca_pangan (ton per bulan). SurplusTon adalah
// kolom turunan di basis data (produksi − konsumsi), hanya dibaca. PK-nya ID BIGINT identity; kunci alami
// (komoditas_kode, wilayah_kode, periode) adalah UNIQUE dan target upsert impor.
type NeracaPangan struct {
	ID            idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	WilayahKode   string    `gorm:"column:wilayah_kode"`
	Periode       time.Time `gorm:"column:periode;type:date"`
	ProduksiTon   float64   `gorm:"column:produksi_ton;type:numeric(12,1)"`
	KonsumsiTon   float64   `gorm:"column:konsumsi_ton;type:numeric(12,1)"`
	StokTon       *float64  `gorm:"column:stok_ton;type:numeric(12,1)"`
	SurplusTon    *float64  `gorm:"column:surplus_ton;type:numeric(12,1);->"`
	Sumber        string    `gorm:"column:sumber"`
	DiimporPada   time.Time `gorm:"column:diimpor_pada;default:now()"`
}

func (NeracaPangan) TableName() string { return "neraca_pangan" }

// TargetInflasi memetakan tabel target_inflasi: koridor sasaran per tahun (%).
type TargetInflasi struct {
	Tahun      int     `gorm:"column:tahun;primaryKey"`
	BatasBawah float64 `gorm:"column:batas_bawah;type:numeric(4,2)"`
	BatasAtas  float64 `gorm:"column:batas_atas;type:numeric(4,2)"`
}

func (TargetInflasi) TableName() string { return "target_inflasi" }

// RuangFiskal memetakan tabel ruang_fiskal: pagu dan realisasi satu instrumen (rupiah). PK-nya ID BIGINT
// identity; kunci alami (tahun, instrumen, per_tanggal) adalah UNIQUE. Kode memuat ID lama (mis. DEMO-RF-202609-1).
type RuangFiskal struct {
	ID          idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	Kode        *string   `gorm:"column:kode"`
	Tahun       int       `gorm:"column:tahun"`
	Instrumen   string    `gorm:"column:instrumen"`
	Pagu        int64     `gorm:"column:pagu"`
	Realisasi   int64     `gorm:"column:realisasi"`
	PerTanggal  time.Time `gorm:"column:per_tanggal;type:date"`
	Sumber      string    `gorm:"column:sumber"`
	DiimporPada time.Time `gorm:"column:diimpor_pada;default:now()"`
}

func (RuangFiskal) TableName() string { return "ruang_fiskal" }

// Penyumbang adalah satu komoditas penyumbang inflasi beserta namanya.
type Penyumbang struct {
	Kode  string  `gorm:"column:kode"`
	Nama  string  `gorm:"column:nama"`
	Andil float64 `gorm:"column:andil"`
}

// NeracaItem adalah satu baris neraca pangan beserta nama komoditasnya.
type NeracaItem struct {
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	Nama          string    `gorm:"column:nama"`
	Periode       time.Time `gorm:"column:periode"`
	ProduksiTon   float64   `gorm:"column:produksi_ton"`
	KonsumsiTon   float64   `gorm:"column:konsumsi_ton"`
	StokTon       *float64  `gorm:"column:stok_ton"`
	Sumber        string    `gorm:"column:sumber"`
}

// HitungIntervensi adalah jumlah kartu intervensi satu siklus.
type HitungIntervensi struct {
	Siklus  string `gorm:"column:siklus"`
	Aktif   int    `gorm:"column:aktif"`
	Selesai int    `gorm:"column:selesai"`
}

// Status ruang fiskal.
const (
	FiskalMemadai  = "MEMADAI"
	FiskalTerbatas = "TERBATAS"
	FiskalKritis   = "KRITIS"
)

// SkorFiskal menilai ruang fiskal secara indikatif dari porsi pagu yang belum
// terealisasi (0–100), bukan penilaian resmi DJPb. Status: ≥60 memadai, ≥30 terbatas.
func SkorFiskal(pagu, realisasi int64) (skor int, status string) {
	if pagu <= 0 {
		return 0, FiskalKritis
	}
	sisa := max(pagu-realisasi, 0)
	skor = int((sisa*100 + pagu/2) / pagu)
	switch {
	case skor >= 60:
		return skor, FiskalMemadai
	case skor >= 30:
		return skor, FiskalTerbatas
	default:
		return skor, FiskalKritis
	}
}
