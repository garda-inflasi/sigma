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

// OptimasiRepository adalah akses data untuk optimasi rute.
type OptimasiRepository interface {
	// ListMarker dipakai untuk koordinat pusat wilayah.
	ListMarker(ctx context.Context) ([]Marker, error)
	ListNeraca(ctx context.Context, q NeracaQuery) ([]Neraca, error)
	// ListJarak mengembalikan seluruh cache jarak antar wilayah.
	ListJarak(ctx context.Context) ([]JarakAntarWilayah, error)
	// SimpanOptimasi menulis rencana, menandai rute optimasi lama untuk komoditas
	// dan periode yang sama sebagai DIGANTIKAN, menyisipkan rute baru, dan mengisi
	// cache jarak (pasangan yang sudah ada tidak ditimpa) dalam satu transaksi.
	// ID rencana dan rute dibuat basis data (identity): rencana.ID dan rencana.Kode terisi
	// sesudah berhasil, dan rute menunjuk ID itu lewat OptimasiID.
	SimpanOptimasi(ctx context.Context, rencana *OptimasiRute, rute []RuteDistribusi, jarak []JarakAntarWilayah) error
	// ListOptimasi mengembalikan rencana tersimpan, terbaru lebih dulu.
	ListOptimasi(ctx context.Context, limit int) ([]RencanaOptimasi, error)
}
