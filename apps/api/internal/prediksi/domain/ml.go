package domain

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/pkg/idx"
)

var (
	// ErrModelTidakDitemukan: id atau kode model tidak ada di registri.
	ErrModelTidakDitemukan = errors.New("model tidak ditemukan")
	// ErrStatusModelTidakValid: status model sekarang tidak memungkinkan aksi itu.
	ErrStatusModelTidakValid = errors.New("status model tidak memungkinkan aksi ini")
	// ErrModelTanpaProyeksi: model belum punya proyeksi; mempromosikannya membuat komoditas tanpa proyeksi.
	ErrModelTanpaProyeksi = errors.New("model belum punya proyeksi")
	// ErrModelSudahAda: versi atau kode model yang sama sudah tersimpan (mis. pelatihan ganda dalam satu detik).
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
	// SimpanModel menyimpan model beserta proyeksinya dalam satu transaksi: ID model diberikan basis data
	// (diisi ke m.ID), lalu setiap proyeksi ditautkan ke model itu (ModelID) dan di-upsert menurut kunci
	// alaminya. ErrModelSudahAda bila versi atau kode model sudah dipakai.
	SimpanModel(ctx context.Context, m *ModelRegistry, proyeksi []PrediksiHarga) error
	// SimpanProyeksi meng-upsert proyeksi menurut kunci alaminya (komoditas, kabupaten, tanggal target,
	// horizon, versi model); ModelID setiap proyeksi wajib terisi.
	SimpanProyeksi(ctx context.Context, proyeksi []PrediksiHarga) error
	// ModelProduksi mengembalikan model PRODUKSI komoditas; gorm.ErrRecordNotFound bila tidak ada.
	ModelProduksi(ctx context.Context, kode string) (*ModelRegistry, error)
	// GetModelView mengembalikan satu baris registri; gorm.ErrRecordNotFound bila tidak ada.
	GetModelView(ctx context.Context, id idx.ID) (*ModelView, error)
	// EvaluasiJatuhTempo mengevaluasi proyeksi bersumber MODEL yang tanggal targetnya
	// sebelum `hariIni` dan belum dievaluasi, terhadap harga konsumen rata-rata hari
	// target. Mengembalikan jumlah proyeksi yang dievaluasi (yang tanpa data harga dilewati).
	EvaluasiJatuhTempo(ctx context.Context, hariIni time.Time) (int64, error)
	// GetModel mengembalikan satu baris registri menurut ID atau kode; ErrModelTidakDitemukan bila tidak ada.
	GetModel(ctx context.Context, ref idx.RujukanID) (*ModelRegistry, error)
	// Promosikan menjadikan model berstatus KANDIDAT atau ARSIP sebagai PRODUKSI dan
	// mengarsipkan model PRODUKSI komoditas yang sama, dalam satu transaksi beserta
	// riwayat model_promosi. ErrModelTidakDitemukan bila model (ID atau kode) tidak ada,
	// ErrStatusModelTidakValid bila sudah PRODUKSI, ErrModelTanpaProyeksi bila model
	// belum punya baris prediksi_harga.
	Promosikan(ctx context.Context, ref idx.RujukanID, oleh string, waktu time.Time) (*HasilPromosi, error)
	// Arsipkan mengarsipkan model berstatus KANDIDAT. ErrStatusModelTidakValid untuk status
	// lain (model PRODUKSI tidak dapat diarsipkan langsung: promosikan penggantinya).
	Arsipkan(ctx context.Context, ref idx.RujukanID, oleh string, waktu time.Time) (*ModelRegistry, error)
}

// SimulasiRepository adalah akses data untuk simulasi skenario.
type SimulasiRepository interface {
	// GetTerbaru mengembalikan proyeksi terbaru model PRODUKSI; gorm.ErrRecordNotFound bila tidak ada.
	GetTerbaru(ctx context.Context, kode, kabupaten string, horizon int) (*ProyeksiView, error)
	// SimpanSkenario menyimpan hasil simulasi; ID diberikan basis data dan diisi ke s.ID.
	SimpanSkenario(ctx context.Context, s *SkenarioSimulasi) error
}
