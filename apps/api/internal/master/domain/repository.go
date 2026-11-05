package domain

import "context"

// ListQuery adalah pencarian dan paginasi opsional. Limit 0 berarti semua baris.
type ListQuery struct {
	// Search mencocokkan kode atau nama tanpa peka huruf besar-kecil.
	Search string
	Page   int
	Limit  int
}

// Repository mengembalikan baris halaman ini beserta jumlah seluruh hasil pencarian.
type Repository interface {
	ListKomoditas(ctx context.Context, q ListQuery) ([]Komoditas, int64, error)
	ListWilayah(ctx context.Context, provinsiKode string, q ListQuery) ([]Kabupaten, int64, error)
	ListPasar(ctx context.Context, kabupatenKode string, q ListQuery) ([]Pasar, int64, error)
	BulkKomoditas(ctx context.Context, kode []string, action BulkAction) (int64, error)
}
