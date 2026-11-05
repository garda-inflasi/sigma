package domain

import "context"

// Repository adalah akses data baca-saja untuk peta.
type Repository interface {
	// ListMarker mengembalikan seluruh wilayah, terurut menurut kode.
	ListMarker(ctx context.Context) ([]Marker, error)
	ListNeraca(ctx context.Context, q NeracaQuery) ([]Neraca, error)
	// ListRute mengembalikan rute terurut dari yang berbiaya terbesar.
	ListRute(ctx context.Context) ([]Rute, error)
	// ListKandidat mengembalikan kandidat terurut dari skor tertinggi.
	ListKandidat(ctx context.Context) ([]Kandidat, error)
	// ListGeometri mengembalikan geometri wilayah terurut menurut kode; bila bbox
	// diberikan, hanya yang kotak pembatasnya beririsan.
	ListGeometri(ctx context.Context, bbox *BBox) ([]GeometriWilayah, error)
}
