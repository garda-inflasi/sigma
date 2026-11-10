package domain

import (
	"context"
	"time"
)

// DeretKpiQuery adalah permintaan fakta harian KPI. Tanggalnya DATE waktu Aceh, kedua ujungnya inklusif.
type DeretKpiQuery struct {
	// Cakupan membatasi kabupaten yang boleh dibaca (02-B); nilai nol tertutup: tidak ada baris.
	Cakupan Cakupan
	// KabupatenKode dan KomoditasKode adalah filter permintaan, digabung (AND) dengan Cakupan. Kosong: tanpa filter.
	KabupatenKode string
	KomoditasKode string
	Awal, Akhir   time.Time
}

// HeatmapQuery adalah permintaan sel peta panas.
type HeatmapQuery struct {
	// Cakupan membatasi kabupaten yang boleh dibaca (02-B); nilai nol tertutup: tidak ada sel.
	Cakupan Cakupan
	// KabupatenKode dan KomoditasKode adalah filter permintaan, digabung (AND) dengan Cakupan. Kosong: tanpa filter.
	KabupatenKode string
	KomoditasKode string
	// Acuan adalah tanggal acuan (DATE waktu Aceh): sel menunjukkan harga terakhir dalam 3 hari sampai Acuan
	// terhadap rata-rata tertimbang 7 hari sebelum tanggal harga itu.
	Acuan time.Time
	// Langsung menghitung sel dari agregat_harga_harian dan gold_harga_harian untuk Acuan. Bila salah, sel dibaca
	// dari mv_heatmap_harga yang selalu relatif terhadap hari ini (jalur bawaan, cepat), dan Acuan harus hari ini.
	Langsung bool
	// Dari membatasi sel pada yang harga terakhirnya jatuh pada atau setelah tanggal ini. Hanya dipakai Langsung;
	// jendela 3 hari sampai Acuan tidak pernah dilebarkan.
	Dari time.Time
}

type Repository interface {
	// PenggunaCakupan membaca peran dan kabupaten tugas pengguna dari basis data (bukan dari klaim token, yang bisa
	// basi sampai access token habis). Mengembalikan ErrPenggunaTidakAda bila akun tidak ada.
	PenggunaCakupan(ctx context.Context, penggunaID string) (PenggunaCakupan, error)

	// Operasional
	// DeretKpi menyatukan fakta harian mv_kpi_operasional pada [Awal, Akhir] atas kabupaten dan komoditas yang lolos
	// cakupan dan filter: satu TitikKpi per indikator dan hari yang punya fakta, terurut menaik menurut tanggal.
	DeretKpi(ctx context.Context, q DeretKpiQuery) ([]TitikKpi, error)
	// TargetKpi mengembalikan target yang berlaku pada tanggal itu.
	TargetKpi(ctx context.Context, tanggal time.Time) ([]TargetKpi, error)
	Heatmap(ctx context.Context, q HeatmapQuery) (*HeatmapData, error)
	// PembandingHarga mengembalikan pasangan harga lapangan dan eksternal terbaru (satu per komoditas
	// dan kabupaten) sejak tanggal itu, hanya di kabupaten dalam cakupan.
	PembandingHarga(ctx context.Context, sejak time.Time, cakupan Cakupan) ([]PembandingHarga, error)
	// CuacaTerbaru mengembalikan cuaca harian terbaru tiap kabupaten pada rentang [sejak, sampai], hanya di
	// kabupaten dalam cakupan.
	CuacaTerbaru(ctx context.Context, sejak, sampai time.Time, cakupan Cakupan) ([]CuacaKabupaten, error)
	// AktivitasLapangan menghitung entri antara awal (inklusif) dan akhir (eksklusif) per kabupaten dalam cakupan.
	AktivitasLapangan(ctx context.Context, awal, akhir time.Time, cakupan Cakupan) ([]AktivitasKabupaten, error)

	// AktivitasPetugas mengembalikan ringkasan aktivitas tiap petugas lapangan aktif yang kabupaten tugasnya lolos
	// cakupan dan filter. Petugas tanpa aktivitas tetap ada (nilai nol); urutan tidak dijamin (usecase mengurutkan).
	AktivitasPetugas(ctx context.Context, q AktivitasPetugasQuery) ([]AktivitasPetugas, error)
	// FaktaStok membaca stok (lot yang belum habis) dan kapasitas fasilitas dalam satu snapshot konsisten.
	FaktaStok(ctx context.Context, q StokQuery) (FaktaStok, error)
	// Produsen mengembalikan fakta harga produsen, harga konsumen pembanding, dan produksi neraca per komoditas.
	Produsen(ctx context.Context, q ProdusenQuery) ([]ProdusenFakta, error)
	// Logistik membaca ringkasan transaksi stok, transaksi terbaru, dan rute aktif dalam satu snapshot konsisten.
	Logistik(ctx context.Context, q LogistikQuery) (LogistikFakta, error)
}
