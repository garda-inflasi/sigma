package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/ews/domain"
)

// AlertResponse adalah satu alert EWS. Wilayah adalah nama kabupaten/kota resmi.
type AlertResponse struct {
	ID             string     `json:"id"`
	Severity       string     `json:"severity"`
	Jenis          string     `json:"jenis"`
	Judul          string     `json:"judul"`
	Pesan          string     `json:"pesan"`
	KomoditasKode  *string    `json:"komoditas_kode"`
	KomoditasNama  *string    `json:"komoditas_nama"`
	KabupatenKode  string     `json:"kabupaten_kode"`
	Wilayah        string     `json:"wilayah"`
	Nilai          float64    `json:"nilai"`
	SatuanNilai    string     `json:"satuan_nilai"`
	SkorDampak     int        `json:"skor_dampak"`
	Status         string     `json:"status"`
	Sumber         string     `json:"sumber"`
	DibuatPada     time.Time  `json:"dibuat_pada"`
	DiakuiOlehID   *string    `json:"diakui_oleh_id"`
	DiakuiPada     *time.Time `json:"diakui_pada"`
	AturanID       *string    `json:"aturan_id"`
	DieskalasiPada *time.Time `json:"dieskalasi_pada"`
	SelesaiPada    *time.Time `json:"selesai_pada"`
}

// AturanResponse adalah satu aturan ambang EWS.
type AturanResponse struct {
	ID            string     `json:"id"`
	KomoditasKode *string    `json:"komoditas_kode"`
	KomoditasNama *string    `json:"komoditas_nama"`
	KabupatenKode *string    `json:"kabupaten_kode"`
	Jenis         string     `json:"jenis"`
	AmbangWaspada float64    `json:"ambang_waspada"`
	AmbangSiaga   float64    `json:"ambang_siaga"`
	AmbangKritis  float64    `json:"ambang_kritis"`
	JendelaHari   int        `json:"jendela_hari"`
	CooldownJam   int        `json:"cooldown_jam"`
	Kanal         []string   `json:"kanal"`
	Aktif         bool       `json:"aktif"`
	Keterangan    string     `json:"keterangan"`
	Versi         int        `json:"versi"`
	DiubahOlehID  *string    `json:"diubah_oleh_id"`
	DiubahPada    *time.Time `json:"diubah_pada"`
}

// RiwayatResponse adalah satu perubahan status alert; Oleh kosong berarti sistem.
type RiwayatResponse struct {
	ID       string    `json:"id"`
	Dari     *string   `json:"dari"`
	Ke       string    `json:"ke"`
	OlehID   *string   `json:"oleh_id"`
	OlehNama *string   `json:"oleh_nama"`
	Waktu    time.Time `json:"waktu"`
	Catatan  string    `json:"catatan"`
}

// EvaluasiResponse adalah ringkasan satu putaran rule engine.
type EvaluasiResponse struct {
	Aturan    int `json:"aturan"`
	Diukur    int `json:"diukur"`
	Dibuat    int `json:"dibuat"`
	Dinaikkan int `json:"dinaikkan"`
	Dilewati  int `json:"dilewati"`
	Diredam   int `json:"diredam"`
	Galat     int `json:"galat"`
}

// BulkEWSResponse melaporkan jumlah alert yang berubah dan yang tidak.
type BulkEWSResponse struct {
	Berhasil int `json:"berhasil"`
	Gagal    int `json:"gagal"`
}

func ToAlertResponse(a domain.Alert) AlertResponse {
	return AlertResponse{
		ID: a.ID, Severity: a.Severity, Jenis: a.Jenis, Judul: a.Judul, Pesan: a.Pesan,
		KomoditasKode: a.KomoditasKode, KomoditasNama: a.KomoditasNama,
		KabupatenKode: a.KabupatenKode, Wilayah: a.KabupatenNama,
		Nilai: a.Nilai, SatuanNilai: a.SatuanNilai, SkorDampak: a.SkorDampak,
		Status: a.Status, Sumber: a.Sumber, DibuatPada: a.DibuatPada,
		DiakuiOlehID: a.DiakuiOlehID, DiakuiPada: a.DiakuiPada, AturanID: a.AturanID,
		DieskalasiPada: a.DieskalasiPada, SelesaiPada: a.SelesaiPada,
	}
}

func ToAlertResponses(items []domain.Alert) []AlertResponse {
	out := make([]AlertResponse, len(items))
	for i, a := range items {
		out[i] = ToAlertResponse(a)
	}
	return out
}

func ToAturanResponse(a domain.Aturan) AturanResponse {
	kanal := []string(a.Kanal)
	if kanal == nil {
		kanal = []string{}
	}
	return AturanResponse{
		ID: a.ID, KomoditasKode: a.KomoditasKode, KomoditasNama: a.KomoditasNama, KabupatenKode: a.KabupatenKode, Jenis: a.Jenis,
		AmbangWaspada: a.AmbangWaspada, AmbangSiaga: a.AmbangSiaga, AmbangKritis: a.AmbangKritis,
		JendelaHari: a.JendelaHari, CooldownJam: a.CooldownJam, Kanal: kanal, Aktif: a.Aktif, Keterangan: a.Keterangan,
		Versi: a.Versi, DiubahOlehID: a.DiubahOlehID, DiubahPada: a.DiubahPada,
	}
}

func ToAturanResponses(items []domain.Aturan) []AturanResponse {
	out := make([]AturanResponse, len(items))
	for i, a := range items {
		out[i] = ToAturanResponse(a)
	}
	return out
}

func ToRiwayatResponses(items []domain.Riwayat) []RiwayatResponse {
	out := make([]RiwayatResponse, len(items))
	for i, r := range items {
		out[i] = RiwayatResponse{
			ID: r.ID, Dari: r.Dari, Ke: r.Ke, OlehID: r.Oleh, OlehNama: r.OlehNama, Waktu: r.Waktu, Catatan: r.Catatan,
		}
	}
	return out
}

func ToEvaluasiResponse(r domain.RingkasanEvaluasi) EvaluasiResponse {
	return EvaluasiResponse(r)
}
