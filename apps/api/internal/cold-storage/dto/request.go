package dto

import "time"

// ColdStorageRequest is the query params for GET /v1/cold-storage.
// Seluruhnya opsional; nilai kosong memakai bawaan di usecase.
type ColdStorageRequest struct {
	// ColdStorageID menyaring stok pada satu fasilitas.
	ColdStorageID string `form:"cold_storage_id" binding:"omitempty,max=255"`
	// Q mencari pada nama komoditas atau pemilik.
	Q     string `form:"q" binding:"omitempty,max=100"`
	Sort  string `form:"sort" binding:"omitempty,oneof=sisa_masa_simpan komoditas fasilitas volume masuk risiko pemilik"`
	Order string `form:"order" binding:"omitempty,oneof=asc desc"`
	Page  int    `form:"page" binding:"omitempty,min=1"`
	Limit int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

type BulkColdStorageRequest struct {
	Action string   `json:"action" binding:"required,oneof=rencanakan_distribusi rekomendasi_olahan tandai_prioritas_jual buang"`
	IDs    []string `json:"ids" binding:"required,min=1,max=500,dive,required,max=255"`
}

// DetailFasilitasRequest adalah query GET /v1/cold-storage/fasilitas/:id.
type DetailFasilitasRequest struct {
	// Jam adalah rentang riwayat suhu (bawaan 48, maksimum 720).
	Jam int `form:"jam" binding:"omitempty,min=1,max=720"`
}

// PembacaanRequest adalah satu pembacaan sensor.
type PembacaanRequest struct {
	FasilitasID   string    `json:"fasilitas_id" binding:"required,max=255"`
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
	Jenis             string     `json:"jenis" binding:"required,oneof=MASUK KELUAR PINDAH"`
	LotID             string     `json:"lot_id" binding:"max=255"`
	FasilitasID       string     `json:"fasilitas_id" binding:"max=255"`
	FasilitasTujuanID string     `json:"fasilitas_tujuan_id" binding:"max=255"`
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
