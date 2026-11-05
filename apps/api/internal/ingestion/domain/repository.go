package domain

import (
	"context"
	"errors"
	"time"

	sumber "github.com/garda-inflasi/api/internal/sumber-data/domain"
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
	ID              int64     `gorm:"column:id"`
	EksekusiID      int64     `gorm:"column:eksekusi_id"`
	KunciPermintaan string    `gorm:"column:kunci_permintaan"`
	DiambilPada     time.Time `gorm:"column:diambil_pada"`
	Ukuran          int       `gorm:"column:ukuran"`
	Cuplikan        string    `gorm:"column:cuplikan"`
}

// Repository adalah akses data engine ingestion.
type Repository interface {
	// Sumber membaca satu baris katalog; ErrSumberTidakAda bila tidak ada.
	Sumber(ctx context.Context, id string) (*sumber.SumberData, error)
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
	SimpanMentah(ctx context.Context, m DataMentah) (id int64, err error)
	// SimpanSilver meng-upsert baris hasil normalisasi (idempoten menurut kunci alami).
	SimpanSilver(ctx context.Context, h Hasil) error
	// SelesaikanEksekusi mengisi hasil akhir dan menyegarkan statistik sumber
	// (terakhir_jalan, status, keberhasilan_7h, latensi_ms).
	SelesaikanEksekusi(ctx context.Context, e *EksekusiSumber) error

	// Eksekusi membaca riwayat eksekusi sebuah sumber, terbaru lebih dulu.
	Eksekusi(ctx context.Context, sumberID string, limit int) ([]EksekusiSumber, error)
	// SampelMentah membaca cuplikan payload bronze terbaru sebuah sumber.
	SampelMentah(ctx context.Context, sumberID string, limit int) ([]SampelMentah, error)
}
