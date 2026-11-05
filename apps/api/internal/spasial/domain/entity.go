// Package domain berisi model dan kontrak repository modul spasial.
package domain

import "time"

// Status marker peta menurut deviasi harga (persen terhadap rata-rata 7 hari).
const (
	StatusDefisit = "defisit"
	StatusWaspada = "waspada"
	StatusSurplus = "surplus"
	StatusNormal  = "normal"
)

// Batas deviasi harga untuk status marker.
const (
	batasDefisit = 10.0
	batasWaspada = 5.0
	batasSurplus = -6.0
)

// StatusDariDelta menurunkan status marker dari deviasi harga.
func StatusDariDelta(delta float64) string {
	switch {
	case delta > batasDefisit:
		return StatusDefisit
	case delta > batasWaspada:
		return StatusWaspada
	case delta < batasSurplus:
		return StatusSurplus
	default:
		return StatusNormal
	}
}

// Wilayah memetakan view wilayah (kabupaten/kota sebagai titik peta).
type Wilayah struct {
	Kode  string  `gorm:"column:kode;primaryKey"`
	Nama  string  `gorm:"column:nama"`
	Lat   float64 `gorm:"column:lat"`
	Lng   float64 `gorm:"column:lng"`
	Level string  `gorm:"column:level"`
}

func (Wilayah) TableName() string { return "wilayah" }

// NeracaWilayah memetakan tabel neraca_wilayah: surplus (+) atau defisit (-) pasokan
// dalam ton dan deviasi harga untuk satu komoditas, wilayah, dan periode.
type NeracaWilayah struct {
	KomoditasKode     string    `gorm:"column:komoditas_kode;primaryKey"`
	KabupatenKode     string    `gorm:"column:kabupaten_kode;primaryKey"`
	Periode           time.Time `gorm:"column:periode;primaryKey"`
	SurplusDefisitTon float64   `gorm:"column:surplus_defisit_ton"`
	DeltaHargaPct     float64   `gorm:"column:delta_harga_pct"`
	Sumber            string    `gorm:"column:sumber"`
}

func (NeracaWilayah) TableName() string { return "neraca_wilayah" }

// RuteDistribusi memetakan tabel rute_distribusi.
type RuteDistribusi struct {
	ID            string    `gorm:"column:id;primaryKey"`
	AsalKode      string    `gorm:"column:asal_kode"`
	TujuanKode    string    `gorm:"column:tujuan_kode"`
	KomoditasKode string    `gorm:"column:komoditas_kode"`
	VolumeTon     float64   `gorm:"column:volume_ton"`
	JarakKm       int       `gorm:"column:jarak_km"`
	JamTempuh     float64   `gorm:"column:jam_tempuh"`
	Biaya         int64     `gorm:"column:biaya"`
	Status        string    `gorm:"column:status"`
	Sumber        string    `gorm:"column:sumber"`
	DibuatPada    time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (RuteDistribusi) TableName() string { return "rute_distribusi" }

// KandidatColdStorage memetakan tabel kandidat_cold_storage. KomponenSkor adalah
// dokumen JSON {nama komponen: nilai 0-100}.
type KandidatColdStorage struct {
	ID            string    `gorm:"column:id;primaryKey"`
	KabupatenKode string    `gorm:"column:kabupaten_kode"`
	Lokasi        string    `gorm:"column:lokasi"`
	Skor          int       `gorm:"column:skor"`
	KomponenSkor  string    `gorm:"column:komponen_skor;type:jsonb"`
	Alasan        string    `gorm:"column:alasan"`
	Status        string    `gorm:"column:status"`
	Sumber        string    `gorm:"column:sumber"`
	DibuatPada    time.Time `gorm:"column:dibuat_pada;default:now()"`
}

func (KandidatColdStorage) TableName() string { return "kandidat_cold_storage" }

// Marker adalah satu wilayah di peta beserta komoditas dengan deviasi harga
// terbesar pada periode terbaru. Komoditas kosong dan Delta 0 bila wilayah belum
// punya neraca.
type Marker struct {
	Wilayah
	Delta      float64
	Komoditas  string
	PunyaPasar bool
}

// Rute adalah rute beserta nama asal, tujuan, dan komoditasnya.
type Rute struct {
	RuteDistribusi
	AsalNama, TujuanNama, KomoditasNama string
}

// Neraca adalah baris neraca beserta nama komoditas dan wilayahnya.
type Neraca struct {
	NeracaWilayah
	KomoditasNama, KabupatenNama string
}

// Kandidat adalah kandidat beserta nama kabupatennya.
type Kandidat struct {
	KandidatColdStorage
	KabupatenNama string
}

// NeracaQuery menyaring neraca. Periode nol berarti periode terbaru.
type NeracaQuery struct {
	KomoditasKode string
	Periode       time.Time
}

// WilayahGeometri memetakan tabel wilayah_geometri: batas satu kabupaten/kota
// sebagai GeoJSON. Sumber DEMO berarti poligon perkiraan, bukan batas resmi.
type WilayahGeometri struct {
	KabupatenKode  string    `gorm:"column:kabupaten_kode;primaryKey"`
	GeoJSON        string    `gorm:"column:geojson;type:jsonb"`
	MinLat         float64   `gorm:"column:min_lat"`
	MinLng         float64   `gorm:"column:min_lng"`
	MaxLat         float64   `gorm:"column:max_lat"`
	MaxLng         float64   `gorm:"column:max_lng"`
	Sumber         string    `gorm:"column:sumber"`
	Keterangan     string    `gorm:"column:keterangan"`
	DiperbaruiPada time.Time `gorm:"column:diperbarui_pada;default:now()"`
}

func (WilayahGeometri) TableName() string { return "wilayah_geometri" }

// SumberResmi menandai geometri hasil impor batas administratif resmi.
const SumberResmi = "RESMI"

// GeometriWilayah adalah geometri beserta nama dan tingkat wilayahnya.
type GeometriWilayah struct {
	WilayahGeometri
	Nama  string
	Level string
}

// LokasiWilayah adalah hasil pencarian wilayah untuk satu titik.
type LokasiWilayah struct {
	Kode, Nama string
	// DiDalam benar bila titik berada di dalam poligon wilayah; bila tidak,
	// wilayah adalah yang pusatnya terdekat.
	DiDalam bool
	// JarakPusatKm adalah jarak garis lurus titik ke titik pusat wilayah.
	JarakPusatKm float64
}
