package dto

// SiklusAktifRequest holds query parameters for the active cycle endpoint.
// Tanpa wilayah_kode, yang dipakai adalah provinsi Aceh.
type SiklusAktifRequest struct {
	WilayahKode string `form:"wilayah_kode"`
}
