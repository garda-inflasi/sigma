package dto

import "time"

// ListHargaRequest adalah query GET /v1/harga dan GET /v1/harga/saya.
// Tanggal berformat YYYY-MM-DD dalam waktu Aceh; sampai bersifat inklusif.
type ListHargaRequest struct {
	Page    int    `form:"page" binding:"omitempty,min=1"`
	Limit   int    `form:"limit" binding:"omitempty,min=1,max=1000"`
	Status  string `form:"status"`
	Wilayah string `form:"wilayah"`
	// Komoditas adalah kode komoditas.
	Komoditas string `form:"komoditas"`
	PasarID   string `form:"pasar_id"`
	Dari      string `form:"dari" binding:"omitempty,datetime=2006-01-02"`
	Sampai    string `form:"sampai" binding:"omitempty,datetime=2006-01-02"`
}

// RingkasanHargaRequest adalah query GET /v1/harga/ringkasan.
type RingkasanHargaRequest struct {
	Hari int `form:"hari" binding:"omitempty,min=1,max=365"`
}

// TrenHargaRequest adalah query GET /v1/harga/tren; wilayah adalah kode kabupaten/kota.
type TrenHargaRequest struct {
	Komoditas string `form:"komoditas" binding:"required"`
	Wilayah   string `form:"wilayah" binding:"required"`
	Hari      int    `form:"hari" binding:"omitempty,min=1,max=365"`
	Interval  string `form:"interval" binding:"omitempty,oneof=harian mingguan"`
}

// CreateHargaRequest adalah isian POST /v1/harga.
type CreateHargaRequest struct {
	KomoditasKode string     `json:"komoditas_kode" binding:"required"`
	PasarID       string     `json:"pasar_id" binding:"required"`
	Tipe          string     `json:"tipe" binding:"required,oneof=PRODUSEN KONSUMEN"`
	Harga         int        `json:"harga" binding:"required,min=1"`
	Kualitas      string     `json:"kualitas" binding:"omitempty,oneof=PREMIUM MEDIUM ASALAN"`
	Catatan       *string    `json:"catatan" binding:"omitempty,max=1000"`
	DicatatPada   *time.Time `json:"dicatat_pada"`
}

// BulkHargaRequest adalah isian POST /v1/harga/bulk.
type BulkHargaRequest struct {
	Aksi string   `json:"aksi" binding:"required,oneof=validasi publikasi tolak kembalikan"`
	IDs  []string `json:"ids" binding:"required,min=1,max=1000"`
}
