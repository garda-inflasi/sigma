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
