package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/ews/domain"
)

// AlertResponse adalah satu alert EWS. Wilayah adalah nama kabupaten/kota resmi.
type AlertResponse struct {
	ID            string     `json:"id"`
	Severity      string     `json:"severity"`
	Jenis         string     `json:"jenis"`
	Judul         string     `json:"judul"`
	Pesan         string     `json:"pesan"`
	KomoditasKode *string    `json:"komoditas_kode"`
	KomoditasNama *string    `json:"komoditas_nama"`
	KabupatenKode string     `json:"kabupaten_kode"`
	Wilayah       string     `json:"wilayah"`
	Nilai         float64    `json:"nilai"`
	SatuanNilai   string     `json:"satuan_nilai"`
	SkorDampak    int        `json:"skor_dampak"`
	Status        string     `json:"status"`
	Sumber        string     `json:"sumber"`
	DibuatPada    time.Time  `json:"dibuat_pada"`
	DiakuiOlehID  *string    `json:"diakui_oleh_id"`
	DiakuiPada    *time.Time `json:"diakui_pada"`
	AturanID      *string    `json:"aturan_id"`
}

// AturanResponse adalah satu aturan ambang EWS.
type AturanResponse struct {
	ID            string   `json:"id"`
	KomoditasKode *string  `json:"komoditas_kode"`
	KomoditasNama *string  `json:"komoditas_nama"`
	Jenis         string   `json:"jenis"`
	AmbangWaspada float64  `json:"ambang_waspada"`
	AmbangSiaga   float64  `json:"ambang_siaga"`
	AmbangKritis  float64  `json:"ambang_kritis"`
	JendelaHari   int      `json:"jendela_hari"`
	Kanal         []string `json:"kanal"`
	Aktif         bool     `json:"aktif"`
	Keterangan    string   `json:"keterangan"`
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
	}
}

func ToAlertResponses(items []domain.Alert) []AlertResponse {
	out := make([]AlertResponse, len(items))
	for i, a := range items {
		out[i] = ToAlertResponse(a)
	}
	return out
}

func ToAturanResponses(items []domain.Aturan) []AturanResponse {
	out := make([]AturanResponse, len(items))
	for i, a := range items {
		kanal := []string(a.Kanal)
		if kanal == nil {
			kanal = []string{}
		}
		out[i] = AturanResponse{
			ID: a.ID, KomoditasKode: a.KomoditasKode, KomoditasNama: a.KomoditasNama, Jenis: a.Jenis,
			AmbangWaspada: a.AmbangWaspada, AmbangSiaga: a.AmbangSiaga, AmbangKritis: a.AmbangKritis,
			JendelaHari: a.JendelaHari, Kanal: kanal, Aktif: a.Aktif, Keterangan: a.Keterangan,
		}
	}
	return out
}
