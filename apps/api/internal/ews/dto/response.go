package dto

import "time"

type EwsAlertResponse struct {
	ID                   int64      `json:"id"`
	KomoditasKode        string     `json:"komoditas_kode"`
	WilayahKode          string     `json:"wilayah_kode"`
	Level                string     `json:"level"`
	Skor                 float64    `json:"skor"`
	Deskripsi            string     `json:"deskripsi"`
	Status               string     `json:"status"`
	Sumber               string     `json:"sumber"`
	SkorDampak           int        `json:"skor_dampak"`
	DiakuiOlehID         *string    `json:"diakui_oleh_id,omitempty"`
	DiakuiPada           *time.Time `json:"diakui_pada,omitempty"`
	DirekomendasikanPada *time.Time `json:"direkomendasikan_pada,omitempty"`
	DitindaklanjutiPada  *time.Time `json:"ditindaklanjuti_pada,omitempty"`
	DibuatPada           time.Time  `json:"dibuat_pada"`
}

type EWSDetailResponse struct {
	ID                   int64      `json:"id"`
	KomoditasKode        string     `json:"komoditas_kode"`
	WilayahKode          string     `json:"wilayah_kode"`
	Level                string     `json:"level"`
	Skor                 float64    `json:"skor"`
	Deskripsi            string     `json:"deskripsi"`
	Status               string     `json:"status"`
	Sumber               string     `json:"sumber"`
	SkorDampak           int        `json:"skor_dampak"`
	DiakuiOlehID         *string    `json:"diakui_oleh_id,omitempty"`
	DiakuiPada           *time.Time `json:"diakui_pada,omitempty"`
	DirekomendasikanPada *time.Time `json:"direkomendasikan_pada,omitempty"`
	DitindaklanjutiPada  *time.Time `json:"ditindaklanjuti_pada,omitempty"`
	DibuatPada           time.Time  `json:"dibuat_pada"`
}

type BulkEWSResponse struct {
	Berhasil int      `json:"berhasil"`
	Gagal    int      `json:"gagal"`
	Errors   []string `json:"errors,omitempty"`
}
