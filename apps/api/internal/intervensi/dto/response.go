package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
)

// PenanggungJawabResponse adalah pengguna yang ditugasi pada kartu.
type PenanggungJawabResponse struct {
	ID   string `json:"id"`
	Nama string `json:"nama"`
}

// ProgresResponse adalah kemajuan checklist kartu.
type ProgresResponse struct {
	Selesai int `json:"selesai"`
	Total   int `json:"total"`
}

// KartuResponse adalah satu kartu kanban.
type KartuResponse struct {
	ID              string                    `json:"id"`
	Siklus          string                    `json:"siklus"`
	Tahap           string                    `json:"tahap"`
	Urutan          int                       `json:"urutan"`
	Judul           string                    `json:"judul"`
	Deskripsi       string                    `json:"deskripsi"`
	KomoditasKode   string                    `json:"komoditas_kode"`
	KabupatenKode   string                    `json:"kabupaten_kode"`
	Prioritas       string                    `json:"prioritas"`
	Tenggat         time.Time                 `json:"tenggat"`
	RekomendasiAI   bool                      `json:"rekomendasi_ai"`
	Progres         ProgresResponse           `json:"progres"`
	EwsAlertID      *string                   `json:"ews_alert_id"`
	PenanggungJawab []PenanggungJawabResponse `json:"penanggung_jawab"`
	DibuatPada      time.Time                 `json:"dibuat_pada"`
	SelesaiPada     *time.Time                `json:"selesai_pada"`
}

// TahapResponse adalah definisi satu tahap pada satu siklus.
type TahapResponse struct {
	Siklus         string `json:"siklus"`
	KodeTahap      string `json:"kode_tahap"`
	Urutan         int    `json:"urutan"`
	Label          string `json:"label"`
	Petunjuk       string `json:"petunjuk"`
	IzinDibutuhkan string `json:"izin_dibutuhkan"`
	SLAHari        *int   `json:"sla_hari"`
}

// CreateIntervensiResponse mengembalikan id kartu baru beserta tahap awalnya.
type CreateIntervensiResponse struct {
	ID    string `json:"id"`
	Tahap string `json:"tahap"`
}

func ToKartuResponse(k domain.Kartu) KartuResponse {
	pj := make([]PenanggungJawabResponse, len(k.PenanggungJawab))
	for i, p := range k.PenanggungJawab {
		pj[i] = PenanggungJawabResponse{ID: p.ID, Nama: p.Nama}
	}
	return KartuResponse{
		ID: k.ID, Siklus: k.Siklus, Tahap: k.Tahap, Urutan: k.Urutan, Judul: k.Judul, Deskripsi: k.Deskripsi,
		KomoditasKode: k.KomoditasKode, KabupatenKode: k.KabupatenKode, Prioritas: k.Prioritas,
		Tenggat: k.Tenggat, RekomendasiAI: k.RekomendasiAI,
		Progres:    ProgresResponse{Selesai: k.ChecklistSelesai, Total: k.ChecklistTotal},
		EwsAlertID: k.EwsAlertID, PenanggungJawab: pj, DibuatPada: k.DibuatPada, SelesaiPada: k.SelesaiPada,
	}
}

func ToKartuResponses(cards []domain.Kartu) []KartuResponse {
	out := make([]KartuResponse, len(cards))
	for i, k := range cards {
		out[i] = ToKartuResponse(k)
	}
	return out
}

func ToTahapResponses(defs []domain.TahapDef) []TahapResponse {
	out := make([]TahapResponse, len(defs))
	for i, d := range defs {
		out[i] = TahapResponse{
			Siklus: d.Siklus, KodeTahap: d.KodeTahap, Urutan: d.Urutan, Label: d.Label, Petunjuk: d.Petunjuk,
			IzinDibutuhkan: d.IzinDibutuhkan, SLAHari: d.SLAHari,
		}
	}
	return out
}
