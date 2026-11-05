package domain

import (
	"context"
	"time"
)

type Repository interface {
	// ListTerbaru mengembalikan, per komoditas, proyeksi terbaru dari model
	// PRODUKSI untuk kabupaten dan horizon tertentu.
	ListTerbaru(ctx context.Context, kabupaten string, horizon int) ([]ProyeksiView, error)
	// GetTerbaru seperti ListTerbaru untuk satu komoditas;
	// gorm.ErrRecordNotFound bila tidak ada.
	GetTerbaru(ctx context.Context, kode, kabupaten string, horizon int) (*ProyeksiView, error)
	// Jalur mengembalikan titik proyeksi model yang sama dengan hari dasar yang
	// sama, dengan horizon sampai maxHorizon, berurut horizon naik.
	Jalur(ctx context.Context, p ProyeksiView, maxHorizon int) ([]TitikProyeksi, error)
	// Riwayat mengembalikan harga konsumen rata-rata harian (entri tervalidasi
	// atau terpublikasi) pada rentang [dari, sampai).
	Riwayat(ctx context.Context, kode, kabupaten string, dari, sampai time.Time) ([]TitikHarga, error)
	// ListModel mengembalikan registri model, komoditas lalu versi terbaru dulu.
	ListModel(ctx context.Context) ([]ModelView, error)
}
