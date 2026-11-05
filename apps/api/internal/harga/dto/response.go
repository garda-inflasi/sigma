package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/harga/domain"
)

// EntriHargaResponse adalah satu baris daftar entri harga.
type EntriHargaResponse struct {
	ID            string `json:"id"`
	KomoditasKode string `json:"komoditas_kode"`
	KomoditasNama string `json:"komoditas_nama"`
	PasarID       string `json:"pasar_id"`
	PasarNama     string `json:"pasar_nama"`
	KabupatenKode string `json:"kabupaten_kode"`
	// Wilayah adalah nama resmi kabupaten/kota, mis. "KABUPATEN ACEH BESAR".
	Wilayah       string    `json:"wilayah"`
	Tipe          string    `json:"tipe"`
	Harga         int       `json:"harga"`
	Satuan        string    `json:"satuan"`
	Kualitas      string    `json:"kualitas"`
	PetugasID     *string   `json:"petugas_id"`
	PetugasNama   *string   `json:"petugas_nama"`
	DicatatPada   time.Time `json:"dicatat_pada"`
	Status        string    `json:"status"`
	DeviasiPct    float64   `json:"deviasi_pct"`
	SkorAnomali   float64   `json:"skor_anomali"`
	AdaFoto       bool      `json:"ada_foto"`
	DalamGeofence bool      `json:"dalam_geofence"`
}

// HargaDetailResponse menambahkan lokasi, bukti, dan jejak validasi pada baris daftar.
type HargaDetailResponse struct {
	EntriHargaResponse
	Sumber          string     `json:"sumber"`
	Lat             *float64   `json:"lat,omitempty"`
	Lng             *float64   `json:"lng,omitempty"`
	FotoURL         *string    `json:"foto_url,omitempty"`
	Catatan         *string    `json:"catatan,omitempty"`
	DivalidasiOleh  *string    `json:"divalidasi_oleh,omitempty"`
	DivalidasiPada  *time.Time `json:"divalidasi_pada,omitempty"`
	DipublikasiOleh *string    `json:"dipublikasi_oleh,omitempty"`
	DipublikasiPada *time.Time `json:"dipublikasi_pada,omitempty"`
	DibuatPada      time.Time  `json:"dibuat_pada"`
	DiupdatePada    time.Time  `json:"diupdate_pada"`
}

// ToEntriHargaResponse memetakan satu baris view ke respons daftar.
func ToEntriHargaResponse(e domain.EntriView) EntriHargaResponse {
	return EntriHargaResponse{
		ID: e.ID, KomoditasKode: e.KomoditasKode, KomoditasNama: e.KomoditasNama,
		PasarID: e.PasarID, PasarNama: e.PasarNama, KabupatenKode: e.KabupatenKode, Wilayah: e.KabupatenNama,
		Tipe: e.Tipe, Harga: e.Harga, Satuan: e.Satuan, Kualitas: e.Kualitas,
		PetugasID: e.PetugasID, PetugasNama: e.PetugasNama, DicatatPada: e.DicatatPada, Status: e.Status,
		DeviasiPct: e.DeviasiPct, SkorAnomali: e.SkorAnomali, AdaFoto: e.AdaFoto, DalamGeofence: e.DalamGeofence,
	}
}

// ToEntriHargaResponses memetakan banyak baris; hasilnya selalu larik (bukan null).
func ToEntriHargaResponses(items []domain.EntriView) []EntriHargaResponse {
	out := make([]EntriHargaResponse, len(items))
	for i, e := range items {
		out[i] = ToEntriHargaResponse(e)
	}
	return out
}

// ToHargaDetailResponse memetakan satu baris view ke respons detail.
func ToHargaDetailResponse(e domain.EntriView) HargaDetailResponse {
	return HargaDetailResponse{
		EntriHargaResponse: ToEntriHargaResponse(e),
		Sumber:             e.Sumber, Lat: e.Lat, Lng: e.Lng, FotoURL: e.FotoURL, Catatan: e.Catatan,
		DivalidasiOleh: e.DivalidasiOleh, DivalidasiPada: e.DivalidasiPada,
		DipublikasiOleh: e.DipublikasiOleh, DipublikasiPada: e.DipublikasiPada,
		DibuatPada: e.DibuatPada, DiupdatePada: e.DiupdatePada,
	}
}

// RingkasanHargaResponse menghitung entri per status.
type RingkasanHargaResponse struct {
	Draf        int64 `json:"draf"`
	Diajukan    int64 `json:"diajukan"`
	Divalidasi  int64 `json:"divalidasi"`
	Dipublikasi int64 `json:"dipublikasi"`
	Ditolak     int64 `json:"ditolak"`
	// Anomali adalah entri dengan deviasi mutlak di atas 25%.
	Anomali int64 `json:"anomali"`
}

// TrenHargaResponse adalah rata-rata harga satu hari atau pekan; sisi tanpa data bernilai null.
type TrenHargaResponse struct {
	Tanggal  time.Time `json:"tanggal"`
	Produsen *float64  `json:"produsen"`
	Konsumen *float64  `json:"konsumen"`
}

type CreateHargaResponse struct {
	ID string `json:"id"`
}

// BulkHargaResponse melaporkan entri yang berubah dan yang dilewati.
type BulkHargaResponse struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}
