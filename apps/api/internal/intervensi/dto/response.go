package dto

import (
	"time"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/internal/intervensi/usecase"
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
	Versi           int                       `json:"versi"`
	DieskalasiPada  *time.Time                `json:"dieskalasi_pada"`
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
		Versi: k.Versi, DieskalasiPada: k.DieskalasiPada,
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

// MoveResponse mengembalikan versi baru kartu yang dipindahkan; klien memakainya untuk
// pemindahan berikutnya tanpa memuat ulang papan.
type MoveResponse struct {
	Versi int `json:"versi"`
}

// HasilKartuResponse adalah hasil aksi massal pada satu kartu.
type HasilKartuResponse struct {
	ID       string `json:"id"`
	Berhasil bool   `json:"berhasil"`
	Alasan   string `json:"alasan,omitempty"`
}

// BulkIntervensiResponse melaporkan hasil aksi massal per kartu.
type BulkIntervensiResponse struct {
	Berhasil int                  `json:"berhasil"`
	Gagal    int                  `json:"gagal"`
	Hasil    []HasilKartuResponse `json:"hasil"`
}

// RiwayatResponse adalah satu peristiwa kartu; OlehID kosong berarti sistem.
type RiwayatResponse struct {
	ID       int64     `json:"id"`
	Aksi     string    `json:"aksi"`
	Dari     *string   `json:"dari"`
	Ke       *string   `json:"ke"`
	OlehID   *string   `json:"oleh_id"`
	OlehNama *string   `json:"oleh_nama"`
	Waktu    time.Time `json:"waktu"`
	Alasan   string    `json:"alasan"`
}

func ToBulkResponse(r usecase.BulkResult) BulkIntervensiResponse {
	out := BulkIntervensiResponse{Berhasil: r.Berhasil, Gagal: r.Gagal, Hasil: make([]HasilKartuResponse, len(r.Hasil))}
	for i, h := range r.Hasil {
		out.Hasil[i] = HasilKartuResponse{ID: h.ID, Berhasil: h.Berhasil, Alasan: h.Alasan}
	}
	return out
}

func ToRiwayatResponses(items []domain.Riwayat) []RiwayatResponse {
	out := make([]RiwayatResponse, len(items))
	for i, r := range items {
		out[i] = RiwayatResponse{ID: r.ID, Aksi: r.Aksi, Dari: r.Dari, Ke: r.Ke, OlehID: r.Oleh, OlehNama: r.OlehNama, Waktu: r.Waktu, Alasan: r.Alasan}
	}
	return out
}

// LangkahPersetujuanResponse adalah satu langkah rantai persetujuan; keputusan kosong bila belum diputuskan.
type LangkahPersetujuanResponse struct {
	Urutan         int        `json:"urutan"`
	Label          string     `json:"label"`
	IzinDibutuhkan string     `json:"izin_dibutuhkan"`
	Keputusan      *string    `json:"keputusan"`
	OlehID         *string    `json:"oleh_id"`
	OlehNama       *string    `json:"oleh_nama"`
	Waktu          *time.Time `json:"waktu"`
	Catatan        string     `json:"catatan"`
}

// PersetujuanResponse adalah permintaan persetujuan beserta rantai langkahnya.
type PersetujuanResponse struct {
	ID              int64                        `json:"id"`
	IntervensiID    string                       `json:"intervensi_id"`
	KartuJudul      string                       `json:"kartu_judul"`
	KartuTahap      string                       `json:"kartu_tahap"`
	Jenis           string                       `json:"jenis"`
	TahapTujuan     *string                      `json:"tahap_tujuan"`
	Status          string                       `json:"status"`
	LangkahKini     int                          `json:"langkah_kini"`
	IzinLangkahKini string                       `json:"izin_langkah_kini"`
	DiajukanOlehID  *string                      `json:"diajukan_oleh_id"`
	DiajukanOleh    *string                      `json:"diajukan_oleh_nama"`
	DiajukanPada    time.Time                    `json:"diajukan_pada"`
	SelesaiPada     *time.Time                   `json:"selesai_pada"`
	Catatan         string                       `json:"catatan"`
	Langkah         []LangkahPersetujuanResponse `json:"langkah"`
}

// AlurResponse adalah definisi alur persetujuan beserta langkahnya.
type AlurResponse struct {
	Kode        string                `json:"kode"`
	Nama        string                `json:"nama"`
	Jenis       string                `json:"jenis"`
	Siklus      *string               `json:"siklus"`
	TahapTujuan *string               `json:"tahap_tujuan"`
	Aktif       bool                  `json:"aktif"`
	Langkah     []AlurLangkahResponse `json:"langkah"`
}

// AlurLangkahResponse adalah satu langkah definisi alur.
type AlurLangkahResponse struct {
	Urutan         int    `json:"urutan"`
	Label          string `json:"label"`
	IzinDibutuhkan string `json:"izin_dibutuhkan"`
}

func ToPersetujuanResponse(p domain.Persetujuan) PersetujuanResponse {
	langkah := make([]LangkahPersetujuanResponse, len(p.Langkah))
	for i, l := range p.Langkah {
		langkah[i] = LangkahPersetujuanResponse{
			Urutan: l.Urutan, Label: l.Label, IzinDibutuhkan: l.IzinDibutuhkan, Keputusan: l.Keputusan,
			OlehID: l.Oleh, OlehNama: l.OlehNama, Waktu: l.Waktu, Catatan: l.Catatan,
		}
	}
	return PersetujuanResponse{
		ID: p.ID, IntervensiID: p.IntervensiID, KartuJudul: p.KartuJudul, KartuTahap: p.KartuTahap, Jenis: p.Jenis,
		TahapTujuan: p.TahapTujuan, Status: p.Status, LangkahKini: p.LangkahKini, IzinLangkahKini: p.IzinLangkahKini(),
		DiajukanOlehID: p.DiajukanOleh, DiajukanOleh: p.DiajukanOlehNm, DiajukanPada: p.DiajukanPada,
		SelesaiPada: p.SelesaiPada, Catatan: p.Catatan, Langkah: langkah,
	}
}

func ToPersetujuanResponses(items []domain.Persetujuan) []PersetujuanResponse {
	out := make([]PersetujuanResponse, len(items))
	for i, p := range items {
		out[i] = ToPersetujuanResponse(p)
	}
	return out
}

func ToAlurResponses(items []domain.Alur) []AlurResponse {
	out := make([]AlurResponse, len(items))
	for i, a := range items {
		langkah := make([]AlurLangkahResponse, len(a.Langkah))
		for j, l := range a.Langkah {
			langkah[j] = AlurLangkahResponse{Urutan: l.Urutan, Label: l.Label, IzinDibutuhkan: l.IzinDibutuhkan}
		}
		out[i] = AlurResponse{Kode: a.Kode, Nama: a.Nama, Jenis: a.Jenis, Siklus: a.Siklus, TahapTujuan: a.TahapTujuan, Aktif: a.Aktif, Langkah: langkah}
	}
	return out
}
