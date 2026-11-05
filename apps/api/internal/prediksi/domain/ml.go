package domain

import (
	"context"
	"time"
)

// SeriHarian adalah harga konsumen rata-rata harian satu kabupaten, berurut tanggal.
type SeriHarian struct {
	KabupatenKode string
	Titik         []TitikHarga
}

// MLRepository adalah akses data untuk pelatihan, penyegaran, dan evaluasi model.
type MLRepository interface {
	// SeriKonsumen mengembalikan seri harga konsumen (entri tervalidasi atau
	// terpublikasi) per kabupaten sejak `dari`, berurut kabupaten.
	SeriKonsumen(ctx context.Context, kode string, dari time.Time) ([]SeriHarian, error)
	// SimpanModel menyimpan model beserta proyeksinya dalam satu transaksi
	// (proyeksi di-upsert menurut id).
	SimpanModel(ctx context.Context, m *ModelRegistry, proyeksi []PrediksiHarga) error
	// SimpanProyeksi meng-upsert proyeksi (menurut id).
	SimpanProyeksi(ctx context.Context, proyeksi []PrediksiHarga) error
	// ModelProduksi mengembalikan model PRODUKSI komoditas; gorm.ErrRecordNotFound bila tidak ada.
	ModelProduksi(ctx context.Context, kode string) (*ModelRegistry, error)
	// GetModelView mengembalikan satu baris registri; gorm.ErrRecordNotFound bila tidak ada.
	GetModelView(ctx context.Context, id string) (*ModelView, error)
	// EvaluasiJatuhTempo mengevaluasi proyeksi bersumber MODEL yang tanggal targetnya
	// sebelum `hariIni` dan belum dievaluasi, terhadap harga konsumen rata-rata hari
	// target. Mengembalikan jumlah proyeksi yang dievaluasi (yang tanpa data harga dilewati).
	EvaluasiJatuhTempo(ctx context.Context, hariIni time.Time) (int64, error)
}
