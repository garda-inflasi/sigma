package domain

import "context"

type Repository interface {
	ListKomoditas(ctx context.Context) ([]Komoditas, error)
	ListWilayah(ctx context.Context, provinsiKode string) ([]Kabupaten, error)
	ListPasar(ctx context.Context, kabupatenKode string) ([]Pasar, error)
	BulkKomoditas(ctx context.Context, kode []string, action BulkAction) (int64, error)
}
