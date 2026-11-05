package dto

import "time"

// NeracaRequest menyaring neraca. Tanpa periode dipakai periode terbaru.
type NeracaRequest struct {
	Komoditas string    `form:"komoditas" binding:"omitempty,max=255"`
	Periode   time.Time `form:"periode" time_format:"2006-01-02"`
}

// GeometriRequest adalah query GET /v1/spasial/geometri. Zoom 0-22 menentukan
// penyederhanaan (bawaan 8); BBox berformat "minLng,minLat,maxLng,maxLat".
type GeometriRequest struct {
	Zoom *int   `form:"zoom" binding:"omitempty,min=0,max=22"`
	BBox string `form:"bbox"`
}

// LokasiRequest adalah query GET /v1/spasial/lokasi.
type LokasiRequest struct {
	Lat *float64 `form:"lat" binding:"required,min=-90,max=90"`
	Lng *float64 `form:"lng" binding:"required,min=-180,max=180"`
}

// PenyesuaianRequest mengubah neraca satu wilayah pada simulasi: ton positif
// menambah surplus (atau mengurangi defisit), negatif sebaliknya.
type PenyesuaianRequest struct {
	KabupatenKode string  `json:"kabupaten_kode" binding:"required,max=4"`
	Ton           float64 `json:"ton"`
}

// OptimasiRequest adalah isi POST /v1/spasial/optimasi-rute. Parameter yang
// tidak diisi memakai bawaan (tarif Rp4.200/ton-km, faktor jalan 1,35, tanpa batas waktu).
type OptimasiRequest struct {
	KomoditasKode string               `json:"komoditas_kode" binding:"required,max=255"`
	Periode       string               `json:"periode" binding:"omitempty,datetime=2006-01-02"`
	Penyesuaian   []PenyesuaianRequest `json:"penyesuaian" binding:"omitempty,max=50,dive"`
	TarifPerTonKm float64              `json:"tarif_per_ton_km" binding:"omitempty,min=0"`
	FaktorJalan   float64              `json:"faktor_jalan" binding:"omitempty,min=1,max=3"`
	MaksJamTempuh float64              `json:"maks_jam_tempuh" binding:"omitempty,min=0,max=240"`
	// Simpan menulis rencana sebagai usulan rute (izin logistik.kelola); tidak boleh bersama penyesuaian.
	Simpan bool `json:"simpan"`
}
