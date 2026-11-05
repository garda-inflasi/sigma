package domain

import (
	"context"
	"time"
)

// Repository membaca bahan dashboard eksekutif. Metode "terbaru" mengembalikan
// nil (bukan galat) bila datanya belum ada, supaya DB kosong menghasilkan
// dashboard kosong, bukan 500.
type Repository interface {
	// InflasiTerbaru adalah baris inflasi bulanan terbaru sebuah wilayah.
	InflasiTerbaru(ctx context.Context, wilayah string) (*InflasiBulanan, error)
	// SeriInflasi mengembalikan paling banyak n bulan sampai dengan periode, urut naik.
	SeriInflasi(ctx context.Context, wilayah string, sampai time.Time, n int) ([]InflasiBulanan, error)
	// TargetBerlaku adalah target tahun itu, atau target tahun terdekat sebelumnya.
	TargetBerlaku(ctx context.Context, tahun int) (*TargetInflasi, error)
	// Penyumbang mengembalikan paling banyak n komoditas dengan |andil| terbesar.
	Penyumbang(ctx context.Context, wilayah string, periode time.Time, n int) ([]Penyumbang, error)
	// NeracaTerbaru adalah neraca pada periode terbaru yang tersedia untuk wilayah.
	NeracaTerbaru(ctx context.Context, wilayah string) ([]NeracaItem, error)
	// RuangFiskalTerbaru adalah semua instrumen pada tanggal laporan terbaru.
	RuangFiskalTerbaru(ctx context.Context) ([]RuangFiskal, error)
	// HitungEWS menghitung peringatan yang belum selesai per severity (INFO, WASPADA, SIAGA, KRITIS).
	HitungEWS(ctx context.Context) (map[string]int, error)
	// HitungIntervensi menghitung kartu aktif dan selesai per siklus.
	HitungIntervensi(ctx context.Context) ([]HitungIntervensi, error)
}
