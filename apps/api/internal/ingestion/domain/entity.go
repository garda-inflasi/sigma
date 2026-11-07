// Package domain memuat model ingestion: run per sumber (eksekusi_sumber), lapisan
// bronze (data_mentah), pemetaan alias sumber ke kode master, dan lapisan silver
// (harga_eksternal, cuaca_harian). Gold adalah materialized view gold_harga_harian.
package domain

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

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
	ID                 idx.ID     `gorm:"column:id;primaryKey;autoIncrement"`
	SumberID           string     `gorm:"column:sumber_id;type:uuid"`
	Pemicu             string     `gorm:"column:pemicu"`
	AktorID            *string    `gorm:"column:aktor_id;type:uuid"`
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
	ID              idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	SumberID        string    `gorm:"column:sumber_id;type:uuid"`
	EksekusiID      idx.ID    `gorm:"column:eksekusi_id"`
	KunciPermintaan string    `gorm:"column:kunci_permintaan"`
	TipeKonten      string    `gorm:"column:tipe_konten"`
	DiambilPada     time.Time `gorm:"column:diambil_pada"`
	Payload         string    `gorm:"column:payload;type:jsonb"`
	Checksum        string    `gorm:"column:checksum"`
}

func (DataMentah) TableName() string { return "data_mentah" }

// PemetaanSumber memetakan tabel pemetaan_sumber (PK komposit sumber_id, jenis, alias). KodeMaster nil bila Diabaikan.
type PemetaanSumber struct {
	SumberID   string  `gorm:"column:sumber_id;type:uuid;primaryKey"`
	Jenis      string  `gorm:"column:jenis;primaryKey"`
	Alias      string  `gorm:"column:alias;primaryKey"`
	KodeMaster *string `gorm:"column:kode_master"`
	Diabaikan  bool    `gorm:"column:diabaikan"`
	Catatan    *string `gorm:"column:catatan"`
}

func (PemetaanSumber) TableName() string { return "pemetaan_sumber" }

// HargaEksternal memetakan tabel harga_eksternal (silver). PK-nya ID BIGINT identity; kunci alami
// (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe) adalah UNIQUE dan target upsert ingestion.
type HargaEksternal struct {
	ID             idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	SumberID       string    `gorm:"column:sumber_id;type:uuid"`
	KomoditasKode  string    `gorm:"column:komoditas_kode"`
	KabupatenKode  string    `gorm:"column:kabupaten_kode"`
	Tanggal        time.Time `gorm:"column:tanggal;type:date"`
	Tipe           string    `gorm:"column:tipe"`
	Harga          float64   `gorm:"column:harga;type:numeric(14,2)"`
	Satuan         string    `gorm:"column:satuan"`
	DataMentahID   *idx.ID   `gorm:"column:data_mentah_id"`
	EksekusiID     *idx.ID   `gorm:"column:eksekusi_id"`
	DiperbaruiPada time.Time `gorm:"column:diperbarui_pada;default:now()"`
}

func (HargaEksternal) TableName() string { return "harga_eksternal" }

// CuacaHarian memetakan tabel cuaca_harian (silver cuaca). PK-nya ID BIGINT identity; kunci alami
// (kabupaten_kode, tanggal, sumber) adalah UNIQUE dan target upsert ingestion. Kode memuat ID lama
// (CUA-...) hanya pada baris yang ada sebelum migrasi 000115.
type CuacaHarian struct {
	ID            idx.ID    `gorm:"column:id;primaryKey;autoIncrement"`
	Kode          *string   `gorm:"column:kode"`
	KabupatenKode string    `gorm:"column:kabupaten_kode"`
	Tanggal       time.Time `gorm:"column:tanggal;type:date"`
	CurahHujanMm  float64   `gorm:"column:curah_hujan_mm"`
	SuhuC         float64   `gorm:"column:suhu_c"`
	Kelembapan    int       `gorm:"column:kelembapan"`
	Anomali       bool      `gorm:"column:anomali"`
	Sumber        string    `gorm:"column:sumber"`
	EksekusiID    *idx.ID   `gorm:"column:eksekusi_id"`
	DataMentahID  *idx.ID   `gorm:"column:data_mentah_id"`
}

func (CuacaHarian) TableName() string { return "cuaca_harian" }
