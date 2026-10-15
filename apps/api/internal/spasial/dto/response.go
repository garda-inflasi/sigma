package dto

type WilayahMarkerResponse struct {
	Kode  string  `json:"kode"`
	Nama  string  `json:"nama"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Level string  `json:"level"`
}
