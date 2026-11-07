package dto

import (
	"encoding/json"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
)

// AuditRowResponse: ID adalah BIGINT yang ditulis sebagai string desimal; Kode memuat ID lama (alias rute detail)
// dan hanya ada pada baris yang dibuat sebelum migrasi 000115 atau baris demo.
type AuditRowResponse struct {
	ID            idx.ID          `json:"id"`
	Kode          *string         `json:"kode,omitempty"`
	Waktu         time.Time       `json:"waktu"`
	AktorID       *string         `json:"aktor_id,omitempty"`
	AktorNama     *string         `json:"aktor_nama,omitempty"`
	AktorEmail    *string         `json:"aktor_email,omitempty"`
	AktorPeran    *string         `json:"aktor_peran,omitempty"`
	Aksi          string          `json:"aksi"`
	Entitas       string          `json:"entitas"`
	EntitasID     string          `json:"entitas_id"`
	Hasil         string          `json:"hasil"`
	IP            *string         `json:"ip,omitempty"`
	UserAgent     *string         `json:"user_agent,omitempty"`
	CorrelationID *string         `json:"correlation_id,omitempty"`
	Sebelum       json.RawMessage `json:"sebelum,omitempty"`
	Sesudah       json.RawMessage `json:"sesudah,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// AuditPageResponse adalah satu halaman audit; NextCursor kosong pada halaman terakhir.
type AuditPageResponse struct {
	Items      []AuditRowResponse `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

// ToAuditPageResponse memetakan halaman domain ke bentuk API.
func ToAuditPageResponse(page domain.Page) AuditPageResponse {
	items := make([]AuditRowResponse, len(page.Items))
	for i, row := range page.Items {
		items[i] = toAuditRowResponse(row)
	}
	return AuditPageResponse{Items: items, NextCursor: page.NextCursor}
}

func toAuditRowResponse(row domain.AuditLog) AuditRowResponse {
	return AuditRowResponse{
		ID: row.ID, Kode: row.Kode, Waktu: row.Waktu,
		AktorID: row.AktorID, AktorNama: row.AktorNama, AktorEmail: row.AktorEmail, AktorPeran: row.AktorPeran,
		Aksi: row.Aksi, Entitas: row.Entitas, EntitasID: row.EntitasID, Hasil: row.Hasil,
		IP: row.IP, UserAgent: row.UserAgent, CorrelationID: row.CorrelationID,
		Sebelum: rawJSON(row.Sebelum), Sesudah: rawJSON(row.Sesudah), Metadata: rawJSON(row.Metadata),
	}
}

func rawJSON(s *string) json.RawMessage {
	if s == nil {
		return nil
	}
	return json.RawMessage(*s)
}

// PerubahanResponse adalah satu bidang yang berbeda antara sebelum dan sesudah.
type PerubahanResponse struct {
	Path    string          `json:"path"`
	Jenis   string          `json:"jenis"`
	Sebelum json.RawMessage `json:"sebelum,omitempty"`
	Sesudah json.RawMessage `json:"sesudah,omitempty"`
}

// AuditDetailResponse adalah satu entri audit lengkap dengan diff sebelum/sesudah.
type AuditDetailResponse struct {
	AuditRowResponse
	Perubahan []PerubahanResponse `json:"perubahan"`
}

// ToAuditDetailResponse memetakan entri dan perubahannya ke bentuk API.
func ToAuditDetailResponse(row domain.AuditLog, changes []domain.Perubahan) AuditDetailResponse {
	perubahan := make([]PerubahanResponse, len(changes))
	for i, c := range changes {
		perubahan[i] = PerubahanResponse{Path: c.Path, Jenis: c.Jenis, Sebelum: c.Sebelum, Sesudah: c.Sesudah}
	}
	return AuditDetailResponse{AuditRowResponse: toAuditRowResponse(row), Perubahan: perubahan}
}
