package dto

import (
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

// ColdStorageRequest is the query params for GET /v1/cold-storage.
// Seluruhnya opsional; nilai kosong memakai bawaan di usecase.
type ColdStorageRequest struct {
	// ColdStorageID menyaring stok pada satu fasilitas: UUID atau kode fasilitas (CS-01). Fasilitas yang tidak
	// ada menghasilkan daftar kosong.
	ColdStorageID string `form:"cold_storage_id" binding:"omitempty,max=64"`
	// Q mencari pada nama komoditas atau pemilik.
	Q     string `form:"q" binding:"omitempty,max=100"`
	Sort  string `form:"sort" binding:"omitempty,oneof=sisa_masa_simpan komoditas fasilitas volume masuk risiko pemilik"`
	Order string `form:"order" binding:"omitempty,oneof=asc desc"`
	Page  int    `form:"page" binding:"omitempty,min=1"`
	Limit int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

type BulkColdStorageRequest struct {
	Action string `json:"action" binding:"required,oneof=rencanakan_distribusi rekomendasi_olahan tandai_prioritas_jual buang"`
	// IDs adalah ID BIGINT lot (string desimal atau angka); ID yang bukan bilangan bulat positif ditolak (400).
	IDs []idx.ID `json:"ids" binding:"required,min=1,max=500,dive,required"`
}

// DetailFasilitasRequest adalah query GET /v1/cold-storage/fasilitas/:id.
type DetailFasilitasRequest struct {
	// Jam adalah rentang riwayat suhu (bawaan 48, maksimum 720).
	Jam int `form:"jam" binding:"omitempty,min=1,max=720"`
}

// PembacaanRequest adalah satu pembacaan sensor. FasilitasID adalah UUID atau kode fasilitas (CS-01).
type PembacaanRequest struct {
	FasilitasID   string    `json:"fasilitas_id" binding:"required,max=64"`
	Waktu         time.Time `json:"waktu" binding:"required"`
	SuhuC         *float64  `json:"suhu_c" binding:"required"`
	KelembapanPct *float64  `json:"kelembapan_pct"`
	Sumber        string    `json:"sumber" binding:"omitempty,oneof=SENSOR MANUAL"`
}

// IngestTelemetriRequest adalah isi POST /v1/cold-storage/telemetri.
type IngestTelemetriRequest struct {
	Pembacaan []PembacaanRequest `json:"pembacaan" binding:"required,min=1,max=500,dive"`
}

// TransaksiRequest adalah isi POST /v1/cold-storage/transaksi. Kolom yang
// wajib bergantung pada jenis; aturannya divalidasi di usecase.
type TransaksiRequest struct {
	Jenis string `json:"jenis" binding:"required,oneof=MASUK KELUAR PINDAH"`
	// LotID adalah ID BIGINT lot (string desimal atau angka); ID yang salah bentuk ditolak (400).
	LotID idx.ID `json:"lot_id"`
	// FasilitasID dan FasilitasTujuanID adalah UUID atau kode fasilitas (CS-01).
	FasilitasID       string     `json:"fasilitas_id" binding:"max=64"`
	FasilitasTujuanID string     `json:"fasilitas_tujuan_id" binding:"max=64"`
	KomoditasKode     string     `json:"komoditas_kode" binding:"max=255"`
	Pemilik           string     `json:"pemilik" binding:"max=255"`
	Alasan            string     `json:"alasan" binding:"max=500"`
	VolumeTon         float64    `json:"volume_ton"`
	Semua             bool       `json:"semua"`
	KedaluwarsaPada   *time.Time `json:"kedaluwarsa_pada"`
}

// UbahFasilitasRequest adalah isi PATCH /v1/cold-storage/fasilitas/:id; field yang tidak dikirim tidak diubah.
type UbahFasilitasRequest struct {
	KapasitasTon *int     `json:"kapasitas_ton" binding:"omitempty,min=1"`
	SuhuMinC     *float64 `json:"suhu_min_c" binding:"omitempty,min=-80,max=60"`
	SuhuMaksC    *float64 `json:"suhu_maks_c" binding:"omitempty,min=-80,max=60"`
	Status       *string  `json:"status" binding:"omitempty,oneof=OPERASIONAL PERAWATAN RENCANA"`
}
