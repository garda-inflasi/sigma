package domain

import (
	"context"
	"time"
)

type Repository interface {
	// Operasional
	// DeretKpi mengembalikan deret harian semua indikator, terurut menaik menurut tanggal.
	DeretKpi(ctx context.Context) ([]TitikKpi, error)
	// TargetKpi mengembalikan target yang berlaku pada tanggal itu.
	TargetKpi(ctx context.Context, tanggal time.Time) ([]TargetKpi, error)
	Heatmap(ctx context.Context) (*HeatmapData, error)
	// PembandingHarga mengembalikan pasangan harga lapangan dan eksternal terbaru (satu per komoditas
	// dan kabupaten) sejak tanggal itu.
	PembandingHarga(ctx context.Context, sejak time.Time) ([]PembandingHarga, error)
	// CuacaTerbaru mengembalikan cuaca harian terbaru tiap kabupaten pada rentang [sejak, sampai].
	CuacaTerbaru(ctx context.Context, sejak, sampai time.Time) ([]CuacaKabupaten, error)
	// AktivitasLapangan menghitung entri antara awal (inklusif) dan akhir (eksklusif) per kabupaten.
	AktivitasLapangan(ctx context.Context, awal, akhir time.Time) ([]AktivitasKabupaten, error)
}
