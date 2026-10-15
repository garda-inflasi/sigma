package dto

import "time"

// ── Harga Responses ───────────────────────────────────────────────────

type EntriHargaResponse struct {
	ID            string    `json:"id"`
	KomoditasKode string    `json:"komoditas_kode"`
	PasarID       string    `json:"pasar_id"`
	Harga         float64   `json:"harga"`
	Satuan        string    `json:"satuan"`
	Sumber        string    `json:"sumber"`
	Periode       string    `json:"periode"`
	Status        string    `json:"status"`
	SkorAnomali   float64   `json:"skor_anomali"`
	PetugasID     *string   `json:"petugas_id,omitempty"`
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

type HargaDetailResponse struct {
	ID              string     `json:"id"`
	KomoditasKode   string     `json:"komoditas_kode"`
	PasarID         string     `json:"pasar_id"`
	Harga           float64    `json:"harga"`
	Satuan          string     `json:"satuan"`
	Kualitas        string     `json:"kualitas"`
	Sumber          string     `json:"sumber"`
	Status          string     `json:"status"`
	Periode         string     `json:"periode"`
	DeviasiPct      float64    `json:"deviasi_pct"`
	SkorAnomali     float64    `json:"skor_anomali"`
	Lat             *float64   `json:"lat,omitempty"`
	Lng             *float64   `json:"lng,omitempty"`
	DalamGeofence   bool       `json:"dalam_geofence"`
	FotoURL         *string    `json:"foto_url,omitempty"`
	Catatan         *string    `json:"catatan,omitempty"`
	PetugasID       *string    `json:"petugas_id,omitempty"`
	DivalidasiOleh  *string    `json:"divalidasi_oleh,omitempty"`
	DivalidasiPada  *time.Time `json:"divalidasi_pada,omitempty"`
	DipublikasiOleh *string    `json:"dipublikasi_oleh,omitempty"`
	DipublikasiPada *time.Time `json:"dipublikasi_pada,omitempty"`
	DibuatPada      time.Time  `json:"dibuat_pada"`
	DiupdatePada    time.Time  `json:"diupdate_pada"`
}

type CreateHargaResponse struct {
	ID string `json:"id"`
}

type BulkHargaResponse struct {
	Berhasil int      `json:"berhasil"`
	Gagal    int      `json:"gagal"`
	Errors   []string `json:"errors,omitempty"`
}
