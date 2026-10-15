package dto

import "time"

// ── Harga Responses ───────────────────────────────────────────────────

type EntriHargaResponse struct {
	ID            string    `json:"id"`
	KomoditasKode string    `json:"komoditas_kode"`
	WilayahKode   string    `json:"wilayah_kode"`
	Harga         float64   `json:"harga"`
	Satuan        string    `json:"satuan"`
	Sumber        string    `json:"sumber"`
	Periode       string    `json:"periode"`
	DibuatPada    time.Time `json:"dibuat_pada"`
	DiupdatePada  time.Time `json:"diupdate_pada"`
}

type TrenHargaResponse struct {
	Periode     string  `json:"periode"`
	HargaRerata float64 `json:"harga_rerata"`
	HargaMin    float64 `json:"harga_min"`
	HargaMax    float64 `json:"harga_max"`
	JumlahEntri int     `json:"jumlah_entri"`
}
