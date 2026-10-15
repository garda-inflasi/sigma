package domain

import "context"

// TrenHarga represents aggregated trend data for a commodity in a region.
type TrenHarga struct {
	Periode     string  `json:"periode"`
	HargaRerata float64 `json:"harga_rerata"`
	HargaMin    float64 `json:"harga_min"`
	HargaMax    float64 `json:"harga_max"`
	JumlahEntri int     `json:"jumlah_entri"`
}

type Repository interface {
	List(ctx context.Context, limit int) ([]EntriHarga, error)
	GetTren(ctx context.Context, komoditas, wilayah string) ([]TrenHarga, error)
}
