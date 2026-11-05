package dto

import (
	"encoding/json"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
)

type AuditRowResponse struct {
	ID            string          `json:"id"`
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
		items[i] = AuditRowResponse{
			ID: row.ID, Waktu: row.Waktu,
			AktorID: row.AktorID, AktorNama: row.AktorNama, AktorEmail: row.AktorEmail, AktorPeran: row.AktorPeran,
			Aksi: row.Aksi, Entitas: row.Entitas, EntitasID: row.EntitasID, Hasil: row.Hasil,
			IP: row.IP, UserAgent: row.UserAgent, CorrelationID: row.CorrelationID,
			Sebelum: rawJSON(row.Sebelum), Sesudah: rawJSON(row.Sesudah), Metadata: rawJSON(row.Metadata),
		}
	}
	return AuditPageResponse{Items: items, NextCursor: page.NextCursor}
}

func rawJSON(s *string) json.RawMessage {
	if s == nil {
		return nil
	}
	return json.RawMessage(*s)
}
