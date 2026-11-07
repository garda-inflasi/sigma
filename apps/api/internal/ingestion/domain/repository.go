package domain

import (
	"context"
	"errors"
	"time"

	sumber "github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

var (
	// ErrSumberTidakAda: id sumber tidak ada di katalog.
	ErrSumberTidakAda = errors.New("sumber data tidak ditemukan")
	// ErrTanpaKonektor: sumber ada di katalog tetapi belum punya konektor.
	ErrTanpaKonektor = errors.New("sumber ini belum memiliki konektor ingestion")
	// ErrDijeda: sumber sedang dijeda, tidak boleh dijalankan.
	ErrDijeda = errors.New("sumber sedang dijeda")
	// ErrSudahBerjalan: sumber masih punya eksekusi BERJALAN.
	ErrSudahBerjalan = errors.New("sumber sedang berjalan")
)

// SampelMentah adalah cuplikan satu payload bronze (payload utuh bisa ratusan KB).
type SampelMentah struct {
	ID              idx.ID    `gorm:"column:id"`
	EksekusiID      idx.ID    `gorm:"column:eksekusi_id"`
	KunciPermintaan string    `gorm:"column:kunci_permintaan"`
	DiambilPada     time.Time `gorm:"column:diambil_pada"`
	Ukuran          int       `gorm:"column:ukuran"`
	Cuplikan        string    `gorm:"column:cuplikan"`
}

// Pipeline merangkum isi lapisan medallion dan mutu data dari eksekusi nyata. Persentase
// bernilai nil bila belum ada bahan untuk menghitungnya.
type Pipeline struct {
	Bronze, Silver, Gold int64
	// Demo benar bila ada eksekusi contoh (pemicu DEMO) di dalam hitungan.
	Demo bool
	// Kelengkapan: baris diterima (bukan kosong/rusak) dari seluruh baris dibaca, 7 hari terakhir.
	Kelengkapan *float64
	// Konsistensi: baris yang kode wilayah/komoditasnya terpetakan ke master, 7 hari terakhir.
	Konsistensi *float64
	// Akurasi: pasangan harga lapangan vs eksternal yang selisihnya dalam BatasSelisihPct, 14 hari terakhir.
	Akurasi *float64
}

// BatasSelisihPct adalah selisih harga lapangan vs eksternal yang masih dianggap sepakat.
const BatasSelisihPct = 10.0

// Repository adalah akses data engine ingestion.
type Repository interface {
	// Sumber membaca satu baris katalog menurut UUID atau kode (alias, mis. SRC-BI dari CLI);
	// ErrSumberTidakAda bila tidak ada.
	Sumber(ctx context.Context, ref string) (*sumber.SumberData, error)
	// SumberAktif mengembalikan sumber yang tidak dijeda (bahan penjadwal).
	SumberAktif(ctx context.Context) ([]sumber.SumberData, error)
	// Pemetaan memuat pemetaan alias sebuah sumber beserta satuan komoditas master.
	Pemetaan(ctx context.Context, sumberID string) (*Pemetaan, error)

	// MulaiEksekusi menutup eksekusi BERJALAN yang macet (mulai sebelum macetSebelum)
	// lalu membuat eksekusi baru. ErrDijeda bila sumber dijeda, ErrSudahBerjalan
	// bila masih ada eksekusi aktif.
	MulaiEksekusi(ctx context.Context, sumberID, pemicu string, aktorID *string, macetSebelum time.Time) (*EksekusiSumber, error)
	// SimpanMentah menyimpan payload bronze; payload identik (sumber, checksum)
	// tidak menggandakan baris dan mengembalikan id baris yang sudah ada.
	SimpanMentah(ctx context.Context, m DataMentah) (id idx.ID, err error)
	// SimpanSilver meng-upsert baris hasil normalisasi (idempoten menurut kunci alami).
	SimpanSilver(ctx context.Context, h Hasil) error
	// SelesaikanEksekusi mengisi hasil akhir dan menyegarkan statistik sumber
	// (terakhir_jalan, status, keberhasilan_7h, latensi_ms).
	SelesaikanEksekusi(ctx context.Context, e *EksekusiSumber) error

	// Pipeline menghitung isi lapisan dan mutu data.
	Pipeline(ctx context.Context) (Pipeline, error)
	// TerakhirSukses mengembalikan waktu selesai eksekusi SUKSES/SEBAGIAN terakhir per sumber (kunci: ID UUID).
	TerakhirSukses(ctx context.Context) (map[string]time.Time, error)

	// Eksekusi membaca riwayat eksekusi sebuah sumber (ID UUID), terbaru lebih dulu.
	Eksekusi(ctx context.Context, sumberID string, limit int) ([]EksekusiSumber, error)
	// SampelMentah membaca cuplikan payload bronze terbaru sebuah sumber (ID UUID).
	SampelMentah(ctx context.Context, sumberID string, limit int) ([]SampelMentah, error)
}
