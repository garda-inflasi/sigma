package dto

import "time"

// UpdateTahapRequest memindahkan kartu ke tahap (kode_tahap) dan posisi (indeks dari 0) baru.
type UpdateTahapRequest struct {
	Tahap  string `json:"tahap" binding:"required,max=100"`
	Urutan int    `json:"urutan" binding:"min=0"`
}

// CreateIntervensiRequest membuat kartu baru di tahap pertama siklusnya.
type CreateIntervensiRequest struct {
	Judul              string    `json:"judul" binding:"required,max=255"`
	Deskripsi          string    `json:"deskripsi"`
	Siklus             string    `json:"siklus" binding:"required,oneof=NORMAL TEKANAN SURPLUS"`
	Prioritas          string    `json:"prioritas" binding:"required,oneof=TINGGI SEDANG RENDAH"`
	KomoditasKode      string    `json:"komoditas_kode" binding:"required"`
	KabupatenKode      string    `json:"kabupaten_kode" binding:"required,max=4"`
	Tenggat            time.Time `json:"tenggat" binding:"required"`
	EwsAlertID         *string   `json:"ews_alert_id"`
	PenanggungJawabIDs []string  `json:"penanggung_jawab_ids" binding:"max=20,dive,required"`
}

// ListIntervensiRequest menyaring daftar kartu (semua opsional).
type ListIntervensiRequest struct {
	Siklus    string `form:"siklus" binding:"omitempty,oneof=NORMAL TEKANAN SURPLUS"`
	Tahap     string `form:"tahap" binding:"omitempty,max=100"`
	Prioritas string `form:"prioritas" binding:"omitempty,oneof=TINGGI SEDANG RENDAH"`
}
