package domain

import "context"

type Repository interface {
	// ListFasilitas mengembalikan seluruh fasilitas (operasional lebih dulu)
	// beserta volume terisi dan nama wilayahnya.
	ListFasilitas(ctx context.Context) ([]FasilitasView, error)
	// ListStok mengembalikan satu halaman stok menurut kriteria q, beserta
	// jumlah dan total volume seluruh hasil filter.
	ListStok(ctx context.Context, q StokQuery) (StokPage, error)
	// RingkasanStok menghitung agregat seluruh stok; ambang adalah batas
	// risiko food loss (0–1) untuk kategori berisiko tinggi.
	RingkasanStok(ctx context.Context, ambang float64) (RingkasanStok, error)
	// BulkAction memproses aksi massal; id yang tidak ditemukan tidak dihitung.
	BulkAction(ctx context.Context, ids []string, action BulkAction) (BulkResult, error)
	// CreateAuditLog mencatat jejak audit.
	CreateAuditLog(ctx context.Context, log *AuditLog) error
}
