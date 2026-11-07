package dto

import (
	"encoding/json"
	"time"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// EntriHargaResponse adalah satu baris daftar entri harga.
type EntriHargaResponse struct {
	// ID adalah ID BIGINT dalam string desimal ("123"). Kode berisi ID lama (HRG-D00-00-01-K) bila entri berasal
	// dari sebelum migrasi 000111, NULL untuk entri baru; tampilkan kode ?? id.
	ID            idx.ID  `json:"id"`
	Kode          *string `json:"kode"`
	KomoditasKode string  `json:"komoditas_kode"`
	KomoditasNama string  `json:"komoditas_nama"`
	PasarID       string  `json:"pasar_id"`
	PasarNama     string  `json:"pasar_nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
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
	// FlagValidasi adalah hasil cek otomatis: rentang, outlier, duplikat, geofence, dan bentrok.
	FlagValidasi domain.FlagValidasi `json:"flag_validasi"`
	DitolakOleh  *string             `json:"ditolak_oleh,omitempty"`
	DitolakPada  *time.Time          `json:"ditolak_pada,omitempty"`
	DibuatPada   time.Time           `json:"dibuat_pada"`
	DiupdatePada time.Time           `json:"diupdate_pada"`
}

// ToEntriHargaResponse memetakan satu baris view ke respons daftar.
func ToEntriHargaResponse(e domain.EntriView) EntriHargaResponse {
	return EntriHargaResponse{
		ID: e.ID, Kode: e.Kode, KomoditasKode: e.KomoditasKode, KomoditasNama: e.KomoditasNama,
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
		DitolakOleh: e.DitolakOleh, DitolakPada: e.DitolakPada, FlagValidasi: e.FlagValidasi,
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
	ID     idx.ID `json:"id"`
	Status string `json:"status"`
}

// BentrokResponse adalah isi 409: entri petugas lain yang bentrok dengan kiriman.
type BentrokResponse struct {
	VersiServer VersiServerResponse `json:"versi_server"`
}

// VersiServerResponse adalah versi server dari harga yang bentrok.
type VersiServerResponse struct {
	ID          idx.ID    `json:"id"`
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
	ID    idx.ID `json:"id"`
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

// VersiEntriResponse adalah satu versi isi entri (tabel entri_harga_versi). Perubahan memuat nama kolom
// entri_harga yang berubah dibanding versi sebelumnya (kolom waktu tidak dihitung; kosong pada versi pertama).
// Data adalah salinan lengkap baris entri pada versi itu, apa adanya dari basis data.
type VersiEntriResponse struct {
	Versi     int             `json:"versi"`
	Aksi      string          `json:"aksi"`
	Status    string          `json:"status"`
	Harga     int             `json:"harga"`
	Oleh      *string         `json:"oleh"`
	OlehNama  *string         `json:"oleh_nama"`
	Waktu     time.Time       `json:"waktu"`
	Perubahan []string        `json:"perubahan"`
	Data      json.RawMessage `json:"data"`
}

func toVersiEntriResponse(v domain.VersiView) VersiEntriResponse {
	perubahan := make([]string, len(v.Perubahan))
	copy(perubahan, v.Perubahan)
	return VersiEntriResponse{
		Versi: v.Versi, Aksi: v.Aksi, Status: v.Status, Harga: v.Harga, Oleh: v.Oleh, OlehNama: v.OlehNama,
		Waktu: v.Waktu, Perubahan: perubahan, Data: v.Data,
	}
}

// ToVersiEntriResponses memetakan versi; hasilnya selalu larik (bukan null).
func ToVersiEntriResponses(items []domain.VersiView) []VersiEntriResponse {
	out := make([]VersiEntriResponse, len(items))
	for i, v := range items {
		out[i] = toVersiEntriResponse(v)
	}
	return out
}

// LineageEntriResponse adalah identitas entri pada lineage.
type LineageEntriResponse struct {
	ID            idx.ID  `json:"id"`
	Kode          *string `json:"kode"`
	KomoditasKode string  `json:"komoditas_kode"`
	KomoditasNama string  `json:"komoditas_nama"`
	PasarID       string  `json:"pasar_id"`
	PasarNama     string  `json:"pasar_nama"`
	KabupatenKode string  `json:"kabupaten_kode"`
	KabupatenNama string  `json:"kabupaten_nama"`
	Tipe          string  `json:"tipe"`
	Harga         int     `json:"harga"`
	Satuan        string  `json:"satuan"`
	Status        string  `json:"status"`
}

// LineageTugasResponse adalah tugas lapangan yang dipenuhi entri.
type LineageTugasResponse struct {
	ID            idx.ID    `json:"id"`
	Kode          *string   `json:"kode"`
	Judul         string    `json:"judul"`
	DitautkanPada time.Time `json:"ditautkan_pada"`
}

// LineageAsalResponse menjawab siapa mencatat, kapan, dan dari mana. DiterimaPada adalah waktu server menerima
// entri (dibuat_pada); selisihnya dengan DicatatPada menunjukkan entri yang dikirim terlambat (sinkron offline).
type LineageAsalResponse struct {
	Sumber        string                 `json:"sumber"`
	PetugasID     *string                `json:"petugas_id"`
	PetugasNama   *string                `json:"petugas_nama"`
	DicatatPada   time.Time              `json:"dicatat_pada"`
	DiterimaPada  time.Time              `json:"diterima_pada"`
	Lat           *float64               `json:"lat"`
	Lng           *float64               `json:"lng"`
	DalamGeofence bool                   `json:"dalam_geofence"`
	KodePedagang  *string                `json:"kode_pedagang"`
	Tugas         []LineageTugasResponse `json:"tugas"`
}

// LineageValidasiOtomatisResponse adalah hasil cek otomatis saat entri diterima.
type LineageValidasiOtomatisResponse struct {
	FlagValidasi domain.FlagValidasi `json:"flag_validasi"`
	SkorAnomali  float64             `json:"skor_anomali"`
	DeviasiPct   float64             `json:"deviasi_pct"`
}

// LineageTahapResponse adalah satu tahap jejak (validasi atau publikasi); Alasan hanya terisi pada penolakan.
type LineageTahapResponse struct {
	Oleh   *string    `json:"oleh"`
	Nama   *string    `json:"nama"`
	Pada   *time.Time `json:"pada"`
	Alasan *string    `json:"alasan,omitempty"`
}

// LineageAuditResponse menunjuk satu baris audit trail yang menyebut entri.
type LineageAuditResponse struct {
	ID            idx.ID    `json:"id"`
	Waktu         time.Time `json:"waktu"`
	Aksi          string    `json:"aksi"`
	AktorNama     *string   `json:"aktor_nama"`
	AktorPeran    *string   `json:"aktor_peran"`
	Hasil         string    `json:"hasil"`
	CorrelationID *string   `json:"correlation_id"`
}

// LineageResponse adalah jejak asal entri harga (PRD §1.4). Validasi, Publikasi, dan Penolakan bernilai null
// bila tahap itu belum (atau tidak lagi) berlaku, mis. Validasi null setelah entri dikembalikan ke DRAF.
type LineageResponse struct {
	Entri            LineageEntriResponse            `json:"entri"`
	Asal             LineageAsalResponse             `json:"asal"`
	ValidasiOtomatis LineageValidasiOtomatisResponse `json:"validasi_otomatis"`
	Validasi         *LineageTahapResponse           `json:"validasi"`
	Publikasi        *LineageTahapResponse           `json:"publikasi"`
	Penolakan        *LineageTahapResponse           `json:"penolakan"`
	VersiTerkini     int                             `json:"versi_terkini"`
	Versi            []VersiEntriResponse            `json:"versi"`
	Riwayat          []RiwayatStatusResponse         `json:"riwayat"`
	Audit            []LineageAuditResponse          `json:"audit"`
}

// tahap mengembalikan blok tahap, atau nil bila pelaku dan waktunya sama-sama kosong.
func tahap(oleh *string, nama *string, pada *time.Time, alasan *string) *LineageTahapResponse {
	if oleh == nil && pada == nil {
		return nil
	}
	return &LineageTahapResponse{Oleh: oleh, Nama: nama, Pada: pada, Alasan: alasan}
}

// ToLineageResponse memetakan lineage; semua larik di dalamnya selalu larik (bukan null).
func ToLineageResponse(l domain.Lineage) LineageResponse {
	e := l.Entri
	tugas := make([]LineageTugasResponse, len(l.Tugas))
	for i, t := range l.Tugas {
		tugas[i] = LineageTugasResponse{ID: t.ID, Kode: t.Kode, Judul: t.Judul, DitautkanPada: t.DitautkanPada}
	}
	audit := make([]LineageAuditResponse, len(l.Audit))
	for i, a := range l.Audit {
		audit[i] = LineageAuditResponse{
			ID: a.ID, Waktu: a.Waktu, Aksi: a.Aksi, AktorNama: a.AktorNama, AktorPeran: a.AktorPeran,
			Hasil: a.Hasil, CorrelationID: a.CorrelationID,
		}
	}
	return LineageResponse{
		Entri: LineageEntriResponse{
			ID: e.ID, Kode: e.Kode, KomoditasKode: e.KomoditasKode, KomoditasNama: e.KomoditasNama,
			PasarID: e.PasarID, PasarNama: e.PasarNama, KabupatenKode: e.KabupatenKode, KabupatenNama: e.KabupatenNama,
			Tipe: e.Tipe, Harga: e.Harga, Satuan: e.Satuan, Status: e.Status,
		},
		Asal: LineageAsalResponse{
			Sumber: e.Sumber, PetugasID: e.PetugasID, PetugasNama: e.PetugasNama,
			DicatatPada: e.DicatatPada, DiterimaPada: e.DibuatPada, Lat: e.Lat, Lng: e.Lng,
			DalamGeofence: e.DalamGeofence, KodePedagang: e.KodePedagang, Tugas: tugas,
		},
		ValidasiOtomatis: LineageValidasiOtomatisResponse{FlagValidasi: e.FlagValidasi, SkorAnomali: e.SkorAnomali, DeviasiPct: e.DeviasiPct},
		Validasi:         tahap(e.DivalidasiOleh, l.Nama.Divalidasi, e.DivalidasiPada, nil),
		Publikasi:        tahap(e.DipublikasiOleh, l.Nama.Dipublikasi, e.DipublikasiPada, nil),
		Penolakan:        tahap(e.DitolakOleh, l.Nama.Ditolak, e.DitolakPada, e.AlasanPenolakan),
		VersiTerkini:     l.VersiTerkini,
		Versi:            ToVersiEntriResponses(l.Versi),
		Riwayat:          ToRiwayatStatusResponses(l.Riwayat),
		Audit:            audit,
	}
}
