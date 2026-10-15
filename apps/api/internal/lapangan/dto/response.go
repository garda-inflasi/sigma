package dto

type TugasLapanganResponse struct {
	ID         int64   `json:"id"`
	Judul      string  `json:"judul"`
	Lokasi     string  `json:"lokasi"`
	Tipe       string  `json:"tipe"`
	Status     string  `json:"status"`
	PIC        string  `json:"pic"`
	Tenggat    *string `json:"tenggat,omitempty"`
	DibuatPada string  `json:"dibuat_pada"`
}

type BulkTugasResponse struct {
	Berhasil int `json:"berhasil"`
}
