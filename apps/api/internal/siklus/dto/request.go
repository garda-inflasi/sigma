package dto

// SiklusAktifRequest holds query parameters for the active cycle endpoint.
type SiklusAktifRequest struct {
	WilayahKode string `form:"wilayah_kode" binding:"required"`
}
