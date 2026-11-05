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
	// AlasanPenolakan terisi untuk entri DITOLAK dan terlihat oleh petugas pencatat.
	AlasanPenolakan *string `json:"alasan_penolakan"`
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
	DitolakOleh     *string    `json:"ditolak_oleh,omitempty"`
	DitolakPada     *time.Time `json:"ditolak_pada,omitempty"`
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
		AlasanPenolakan: e.AlasanPenolakan,
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
		DitolakOleh: e.DitolakOleh, DitolakPada: e.DitolakPada,
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
	ID     string `json:"id"`
	Status string `json:"status"`
}

// BentrokResponse adalah isi 409: entri petugas lain yang bentrok dengan kiriman.
type BentrokResponse struct {
	VersiServer VersiServerResponse `json:"versi_server"`
}

// VersiServerResponse adalah versi server dari harga yang bentrok.
type VersiServerResponse struct {
	ID          string    `json:"id"`
	Harga       int       `json:"harga"`
	DicatatPada time.Time `json:"dicatat_pada"`
	Petugas     string    `json:"petugas"`
}

// BulkHargaResponse melaporkan entri yang berubah dan yang dilewati beserta sebabnya.
type BulkHargaResponse struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
	// Rincian memuat tiap entri yang dilewati; selalu larik (bukan null).
	Rincian []RincianBulkResponse `json:"rincian"`
}

// RincianBulkResponse menjelaskan satu entri yang dilewati. Sebab: tidak_ditemukan,
// status_tidak_sesuai, pemisahan_tugas, atau bukan_pemilik.
type RincianBulkResponse struct {
	ID    string `json:"id"`
	Sebab string `json:"sebab"`
}

// RiwayatStatusResponse adalah satu perpindahan status entri.
type RiwayatStatusResponse struct {
	Dari     *string   `json:"dari"`
	Ke       string    `json:"ke"`
	Oleh     *string   `json:"oleh"`
	OlehNama *string   `json:"oleh_nama"`
	Waktu    time.Time `json:"waktu"`
	Alasan   *string   `json:"alasan"`
}

// ToRiwayatStatusResponses memetakan riwayat; hasilnya selalu larik (bukan null).
func ToRiwayatStatusResponses(items []domain.RiwayatView) []RiwayatStatusResponse {
	out := make([]RiwayatStatusResponse, len(items))
	for i, h := range items {
		out[i] = RiwayatStatusResponse{Dari: h.Dari, Ke: h.Ke, Oleh: h.Oleh, OlehNama: h.OlehNama, Waktu: h.Waktu, Alasan: h.Alasan}
	}
	return out
}
