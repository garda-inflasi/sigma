package dto

import "github.com/garda-inflasi/api/internal/dashboard/domain"

// FilterRequest adalah query GET /v1/dashboard/operasional/kpi dan /heatmap. Semua isian opsional. Isinya sengaja
// tidak diperiksa lewat tag binding: domain.NormalisasiFilter yang memeriksanya (format, urutan, batas rentang, dan
// "hari ini" menurut waktu Aceh) dengan pesan galat yang jelas.
type FilterRequest struct {
	// KabupatenKode adalah kode BPS 4 digit kabupaten/kota, mis. 1171.
	KabupatenKode string `form:"kabupaten_kode"`
	// KomoditasKode adalah kode komoditas, mis. CBM-KRT.
	KomoditasKode string `form:"komoditas_kode"`
	// Dari dan Sampai berformat YYYY-MM-DD (tanggal waktu Aceh), inklusif.
	Dari   string `form:"dari"`
	Sampai string `form:"sampai"`
}

// Permintaan menyalin isian ke bentuk yang dipakai usecase.
func (r FilterRequest) Permintaan() domain.PermintaanFilter {
	return domain.PermintaanFilter{
		KabupatenKode: r.KabupatenKode, KomoditasKode: r.KomoditasKode, Dari: r.Dari, Sampai: r.Sampai,
	}
}

// KabupatenRequest adalah query GET /v1/dashboard/operasional/aktivitas-petugas. Hanya kabupaten_kode yang dibaca:
// parameter lain (mis. komoditas_kode) sengaja tidak dibaca supaya tidak ikut membentuk kunci cache dan tidak
// diam-diam tampak berlaku.
type KabupatenRequest struct {
	// KabupatenKode adalah kode BPS 4 digit kabupaten/kota, mis. 1171.
	KabupatenKode string `form:"kabupaten_kode"`
}

// SaringanRequest adalah query GET /v1/dashboard/operasional/{stok,produsen,logistik}. Semua isian opsional dan
// diperiksa domain.NormalisasiSaringan (format, dengan pesan galat yang jelas).
type SaringanRequest struct {
	// KabupatenKode adalah kode BPS 4 digit kabupaten/kota, mis. 1171.
	KabupatenKode string `form:"kabupaten_kode"`
	// KomoditasKode adalah kode komoditas, mis. CBM-KRT.
	KomoditasKode string `form:"komoditas_kode"`
}

// Permintaan menyalin isian ke bentuk yang dipakai usecase.
func (r SaringanRequest) Permintaan() domain.PermintaanSaringan {
	return domain.PermintaanSaringan{KabupatenKode: r.KabupatenKode, KomoditasKode: r.KomoditasKode}
}
