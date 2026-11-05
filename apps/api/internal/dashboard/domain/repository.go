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
	// AktivitasLapangan menghitung entri antara awal (inklusif) dan akhir (eksklusif) per kabupaten.
	AktivitasLapangan(ctx context.Context, awal, akhir time.Time) ([]AktivitasKabupaten, error)

	// Eksekutif
	GetRingkasanEksekutif(ctx context.Context, periode string) (*RingkasanEksekutif, error)
}
