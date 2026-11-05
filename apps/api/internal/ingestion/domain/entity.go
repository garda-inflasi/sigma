// Package domain memuat model ingestion: run per sumber (eksekusi_sumber), lapisan
// bronze (data_mentah), pemetaan alias sumber ke kode master, dan lapisan silver
// (harga_eksternal, cuaca_harian). Gold adalah materialized view gold_harga_harian.
package domain

import "time"

// Status sebuah eksekusi.
const (
	StatusBerjalan = "BERJALAN"
	StatusSukses   = "SUKSES"
	// StatusSebagian: sebagian permintaan berhasil, sebagian gagal.
	StatusSebagian = "SEBAGIAN"
	StatusGagal    = "GAGAL"
)

// Pemicu sebuah eksekusi.
const (
	PemicuJadwal = "JADWAL"
	PemicuManual = "MANUAL"
	PemicuCLI    = "CLI"
	PemicuDemo   = "DEMO"
)

// Jenis pemetaan alias.
const (
	JenisKomoditas = "KOMODITAS"
	JenisWilayah   = "WILAYAH"
)

// TipeHargaKonsumen adalah tipe_harga yang ditulis ingestion: sumber eksternal
// melaporkan harga di tingkat konsumen/pasar.
const TipeHargaKonsumen = "KONSUMEN"

// EksekusiSumber memetakan tabel eksekusi_sumber. BarisDitolak adalah kolom
// turunan (tak terpetakan + tidak valid), hanya dibaca.
type EksekusiSumber struct {
	ID                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	SumberID           string     `gorm:"column:sumber_id"`
	Pemicu             string     `gorm:"column:pemicu"`
	AktorID            *string    `gorm:"column:aktor_id"`
	Status             string     `gorm:"column:status"`
	Mulai              time.Time  `gorm:"column:mulai"`
	Selesai            *time.Time `gorm:"column:selesai"`
	DurasiMs           *int       `gorm:"column:durasi_ms"`
	LatensiMs          *int       `gorm:"column:latensi_ms"`
	Dokumen            int        `gorm:"column:dokumen"`
	BarisDibaca        int        `gorm:"column:baris_dibaca"`
	BarisDiambil       int        `gorm:"column:baris_diambil"`
	BarisTakTerpetakan int        `gorm:"column:baris_tak_terpetakan"`
	BarisTidakValid    int        `gorm:"column:baris_tidak_valid"`
	BarisDitolak       int        `gorm:"column:baris_ditolak;->"`
	Galat              *string    `gorm:"column:galat"`
	Ringkasan          string     `gorm:"column:ringkasan;type:jsonb"`
}

func (EksekusiSumber) TableName() string { return "eksekusi_sumber" }

// DataMentah memetakan tabel data_mentah (bronze).
type DataMentah struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SumberID        string    `gorm:"column:sumber_id"`
	EksekusiID      int64     `gorm:"column:eksekusi_id"`
	KunciPermintaan string    `gorm:"column:kunci_permintaan"`
	TipeKonten      string    `gorm:"column:tipe_konten"`
	DiambilPada     time.Time `gorm:"column:diambil_pada"`
	Payload         string    `gorm:"column:payload;type:jsonb"`
	Checksum        string    `gorm:"column:checksum"`
}

func (DataMentah) TableName() string { return "data_mentah" }

// PemetaanSumber memetakan tabel pemetaan_sumber. KodeMaster nil bila Diabaikan.
type PemetaanSumber struct {
	SumberID   string  `gorm:"column:sumber_id;primaryKey"`
	Jenis      string  `gorm:"column:jenis;primaryKey"`
	Alias      string  `gorm:"column:alias;primaryKey"`
	KodeMaster *string `gorm:"column:kode_master"`
	Diabaikan  bool    `gorm:"column:diabaikan"`
	Catatan    *string `gorm:"column:catatan"`
}

func (PemetaanSumber) TableName() string { return "pemetaan_sumber" }

// HargaEksternal memetakan tabel harga_eksternal (silver).
type HargaEksternal struct {
	SumberID       string    `gorm:"column:sumber_id;primaryKey"`
	KomoditasKode  string    `gorm:"column:komoditas_kode;primaryKey"`
	KabupatenKode  string    `gorm:"column:kabupaten_kode;primaryKey"`
	Tanggal        time.Time `gorm:"column:tanggal;primaryKey;type:date"`
	Tipe           string    `gorm:"column:tipe;primaryKey"`
	Harga          float64   `gorm:"column:harga;type:numeric(14,2)"`
	Satuan         string    `gorm:"column:satuan"`
	DataMentahID   *int64    `gorm:"column:data_mentah_id"`
	EksekusiID     *int64    `gorm:"column:eksekusi_id"`
	DiperbaruiPada time.Time `gorm:"column:diperbarui_pada;default:now()"`
}

func (HargaEksternal) TableName() string { return "harga_eksternal" }

// CuacaHarian memetakan tabel cuaca_harian (silver cuaca). ID deterministik dari
// kunci alaminya (lihat IDCuaca).
type CuacaHarian struct {
	ID            string    `gorm:"column:id;primaryKey"`
	KabupatenKode string    `gorm:"column:kabupaten_kode"`
	Tanggal       time.Time `gorm:"column:tanggal;type:date"`
	CurahHujanMm  float64   `gorm:"column:curah_hujan_mm"`
	SuhuC         float64   `gorm:"column:suhu_c"`
	Kelembapan    int       `gorm:"column:kelembapan"`
	Anomali       bool      `gorm:"column:anomali"`
	Sumber        string    `gorm:"column:sumber"`
	EksekusiID    *int64    `gorm:"column:eksekusi_id"`
	DataMentahID  *int64    `gorm:"column:data_mentah_id"`
}

func (CuacaHarian) TableName() string { return "cuaca_harian" }

// IDCuaca membentuk id baris cuaca dari kunci alaminya (kabupaten, tanggal, sumber).
func IDCuaca(kabupaten string, tanggal time.Time, sumber string) string {
	return "CUA-" + kabupaten + "-" + tanggal.Format("20060102") + "-" + sumber
}
