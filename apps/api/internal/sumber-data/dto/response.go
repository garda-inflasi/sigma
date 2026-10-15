package dto

type SumberDataResponse struct {
	ID              int64  `json:"id"`
	Nama            string `json:"nama"`
	Tipe            string `json:"tipe"`
	URL             string `json:"url"`
	Status          string `json:"status"`
	FrekuensiUpdate string `json:"frekuensi_update"`
	DibuatPada      string `json:"dibuat_pada"`
}

type BulkSumberDataResponse struct {
	Berhasil int `json:"berhasil"`
}
