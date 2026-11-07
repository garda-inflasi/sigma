package dto

import (
	"encoding/json"
	"time"

	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// AjukanPerubahanRequest adalah usulan TAMBAH atau UBAH. Perubahan memuat nilai baru per bidang.
type AjukanPerubahanRequest struct {
	Entitas   string                     `json:"entitas" binding:"required"`
	Jenis     string                     `json:"jenis" binding:"required"`
	EntitasID string                     `json:"entitas_id" binding:"required,max=50"`
	Perubahan map[string]json.RawMessage `json:"perubahan" binding:"required"`
	Alasan    string                     `json:"alasan"`
}

// KeputusanRequest membawa alasan penolakan (wajib) atau pembatalan (opsional).
type KeputusanRequest struct {
	Alasan string `json:"alasan"`
}

// ListPerubahanRequest menyaring antrean dan riwayat usulan.
type ListPerubahanRequest struct {
	Status    string `form:"status"`
	Entitas   string `form:"entitas"`
	EntitasID string `form:"entitas_id"`
	Page      int    `form:"page" binding:"omitempty,min=1"`
	Limit     int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

// PelakuResponse adalah pengguna yang mengajukan atau memutuskan usulan.
type PelakuResponse struct {
	ID   string  `json:"id"`
	Nama *string `json:"nama"`
}

// DiffResponse adalah satu bidang yang diusulkan berubah; Sebelum null untuk data baru.
type DiffResponse struct {
	Bidang  string          `json:"bidang"`
	Sebelum json.RawMessage `json:"sebelum"`
	Sesudah json.RawMessage `json:"sesudah"`
}

// PerubahanResponse adalah satu usulan perubahan master data.
type PerubahanResponse struct {
	ID              idx.ID          `json:"id"`
	Kode            *string         `json:"kode,omitempty"`
	Entitas         string          `json:"entitas"`
	EntitasID       string          `json:"entitas_id"`
	EntitasNama     *string         `json:"entitas_nama"`
	Jenis           string          `json:"jenis"`
	Versi           int             `json:"versi"`
	Status          string          `json:"status"`
	Alasan          *string         `json:"alasan,omitempty"`
	AlasanPenolakan *string         `json:"alasan_penolakan,omitempty"`
	Perubahan       json.RawMessage `json:"perubahan"`
	Sebelum         json.RawMessage `json:"sebelum"`
	Diff            []DiffResponse  `json:"diff"`
	DiajukanOleh    *PelakuResponse `json:"diajukan_oleh"`
	DibuatPada      time.Time       `json:"dibuat_pada"`
	DisetujuiOleh   *PelakuResponse `json:"disetujui_oleh"`
	DisetujuiPada   *time.Time      `json:"disetujui_pada"`
	DitolakOleh     *PelakuResponse `json:"ditolak_oleh"`
	DitolakPada     *time.Time      `json:"ditolak_pada"`
}

var jsonNull = json.RawMessage("null")

// ToPerubahanResponse memetakan usulan ke bentuk API, termasuk diff per bidang.
func ToPerubahanResponse(v domain.PerubahanView) (PerubahanResponse, error) {
	diff, err := domain.DiffUsulan(v.PerubahanMaster)
	if err != nil {
		return PerubahanResponse{}, err
	}
	rows := make([]DiffResponse, len(diff))
	for i, d := range diff {
		sebelum := d.Sebelum
		if sebelum == nil {
			sebelum = jsonNull
		}
		rows[i] = DiffResponse{Bidang: d.Bidang, Sebelum: sebelum, Sesudah: d.Sesudah}
	}
	return PerubahanResponse{
		ID: v.ID, Kode: v.Kode, Entitas: v.Entitas, EntitasID: v.EntitasID, EntitasNama: v.EntitasNama,
		Jenis: v.Jenis, Versi: v.Versi, Status: v.Status, Alasan: v.Alasan, AlasanPenolakan: v.AlasanPenolakan,
		Perubahan: objekJSON(v.Perubahan), Sebelum: objekJSON(v.Sebelum), Diff: rows,
		DiajukanOleh: pelaku(&v.DiajukanOlehID, v.DiajukanNama), DibuatPada: v.DibuatPada,
		DisetujuiOleh: pelaku(v.DisetujuiOlehID, v.DisetujuiNama), DisetujuiPada: v.DisetujuiPada,
		DitolakOleh: pelaku(v.DitolakOlehID, v.DitolakNama), DitolakPada: v.DitolakPada,
	}, nil
}

// ToPerubahanResponses memetakan daftar usulan.
func ToPerubahanResponses(items []domain.PerubahanView) ([]PerubahanResponse, error) {
	out := make([]PerubahanResponse, len(items))
	for i, v := range items {
		r, err := ToPerubahanResponse(v)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}

func pelaku(id *string, nama *string) *PelakuResponse {
	if id == nil || *id == "" {
		return nil
	}
	return &PelakuResponse{ID: *id, Nama: nama}
}

func objekJSON(s string) json.RawMessage {
	if s == "" {
		return json.RawMessage("{}")
	}
	return json.RawMessage(s)
}
