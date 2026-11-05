package dto

import "time"

// UpdateTahapRequest memindahkan kartu ke tahap (kode_tahap) dan posisi (indeks dari 0) baru.
// Versi adalah versi kartu yang dibaca klien (optimistic locking): bila kartu sudah berubah, 409.
type UpdateTahapRequest struct {
	Tahap  string `json:"tahap" binding:"required,max=100"`
	Urutan int    `json:"urutan" binding:"min=0"`
	Versi  int    `json:"versi" binding:"required,min=1"`
}

// BulkIntervensiRequest menerapkan satu aksi pada beberapa kartu: "tugaskan" menambahkan
// pemanggil sebagai penanggung jawab; "eskalasi" mengeskalasi ke Dewan Pengarah.
type BulkIntervensiRequest struct {
	Action string   `json:"action" binding:"required,oneof=tugaskan eskalasi"`
	IDs    []string `json:"ids" binding:"required,min=1,max=100,dive,required"`
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
