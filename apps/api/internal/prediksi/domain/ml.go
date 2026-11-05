package domain

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrModelTidakDitemukan: id model tidak ada di registri.
	ErrModelTidakDitemukan = errors.New("model tidak ditemukan")
	// ErrStatusModelTidakValid: status model sekarang tidak memungkinkan aksi itu.
	ErrStatusModelTidakValid = errors.New("status model tidak memungkinkan aksi ini")
	// ErrModelTanpaProyeksi: model belum punya proyeksi; mempromosikannya membuat komoditas tanpa proyeksi.
	ErrModelTanpaProyeksi = errors.New("model belum punya proyeksi")
	// ErrModelSudahAda: versi model dengan id yang sama sudah tersimpan (mis. pelatihan ganda dalam satu detik).
	ErrModelSudahAda = errors.New("versi model sudah ada")
)

// HasilPromosi melaporkan akibat promosi: model yang menjadi PRODUKSI dan, bila ada,
// model PRODUKSI sebelumnya yang diarsipkan.
type HasilPromosi struct {
	Model      ModelRegistry
	Digantikan *ModelRegistry
	DariStatus string
}

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
	// (proyeksi di-upsert menurut id). ErrModelSudahAda bila id model sudah dipakai.
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
	// GetModel mengembalikan satu baris registri; ErrModelTidakDitemukan bila tidak ada.
	GetModel(ctx context.Context, id string) (*ModelRegistry, error)
	// Promosikan menjadikan model berstatus KANDIDAT atau ARSIP sebagai PRODUKSI dan
	// mengarsipkan model PRODUKSI komoditas yang sama, dalam satu transaksi beserta
	// riwayat model_promosi. ErrModelTidakDitemukan bila id tidak ada,
	// ErrStatusModelTidakValid bila sudah PRODUKSI, ErrModelTanpaProyeksi bila model
	// belum punya baris prediksi_harga.
	Promosikan(ctx context.Context, id, oleh string, waktu time.Time) (*HasilPromosi, error)
	// Arsipkan mengarsipkan model berstatus KANDIDAT. ErrStatusModelTidakValid untuk status
	// lain (model PRODUKSI tidak dapat diarsipkan langsung: promosikan penggantinya).
	Arsipkan(ctx context.Context, id, oleh string, waktu time.Time) (*ModelRegistry, error)
}

// SimulasiRepository adalah akses data untuk simulasi skenario.
type SimulasiRepository interface {
	// GetTerbaru mengembalikan proyeksi terbaru model PRODUKSI; gorm.ErrRecordNotFound bila tidak ada.
	GetTerbaru(ctx context.Context, kode, kabupaten string, horizon int) (*ProyeksiView, error)
	// SimpanSkenario menyimpan hasil simulasi.
	SimpanSkenario(ctx context.Context, s *SkenarioSimulasi) error
}
