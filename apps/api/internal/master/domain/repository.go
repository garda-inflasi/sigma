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
	// StatusKomoditas memetakan kode → status untuk kode yang ada; kode tak dikenal tidak muncul.
	StatusKomoditas(ctx context.Context, kode []string) (map[string]string, error)
	// UbahStatusKomoditas menerapkan transisi pada satu komoditas hanya bila statusnya
	// masih salah satu t.Dari (penjaga balapan). ok false berarti tidak ada baris yang berubah.
	UbahStatusKomoditas(ctx context.Context, kode string, t Transisi) (ok bool, err error)
}
