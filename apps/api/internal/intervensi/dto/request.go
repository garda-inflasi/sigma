package dto

type UpdateTahapRequest struct {
	Tahap  string `json:"tahap" binding:"required,oneof=usulan kaji setuju laksana selesai batal"`
	Urutan int    `json:"urutan" binding:"min=0"`
}

type CreateIntervensiRequest struct {
	Judul         string `json:"judul" binding:"required"`
	Deskripsi     string `json:"deskripsi"`
	Prioritas     string `json:"prioritas" binding:"required,oneof=RENDAH SEDANG TINGGI KRITIS"`
	PIC           string `json:"pic" binding:"required"`
	KomoditasKode string `json:"komoditas_kode" binding:"required"`
	WilayahKode   string `json:"wilayah_kode" binding:"required"`
	EWSAlertID    *int64 `json:"ews_alert_id"`
}

type BulkIntervensiRequest struct {
	Action string  `json:"action" binding:"required,oneof=arsip hapus"`
	IDs    []int64 `json:"ids" binding:"required,min=1"`
}

type ListIntervensiRequest struct {
	Tahap     string `form:"tahap"`
	Prioritas string `form:"prioritas"`
}
