package dto

import "time"

type EwsAlertResponse struct {
	ID                   int64      `json:"id"`
	KomoditasKode        string     `json:"komoditas_kode"`
	WilayahKode          string     `json:"wilayah_kode"`
	Level                string     `json:"level"`
	Skor                 float64    `json:"skor"`
	Deskripsi            string     `json:"deskripsi"`
	DirekomendasikanPada *time.Time `json:"direkomendasikan_pada,omitempty"`
	DitindaklanjutiPada  *time.Time `json:"ditindaklanjuti_pada,omitempty"`
	DibuatPada           time.Time  `json:"dibuat_pada"`
}
