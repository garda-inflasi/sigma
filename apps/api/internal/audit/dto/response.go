package dto

import (
	"encoding/hex"
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
	// Hash dan HashSebelumnya (hex) menautkan entri ke rantai hash (migrasi 000116).
	Hash           string `json:"hash,omitempty"`
	HashSebelumnya string `json:"hash_sebelumnya,omitempty"`
}

// ToAuditDetailResponse memetakan entri dan perubahannya ke bentuk API.
func ToAuditDetailResponse(row domain.AuditLog, changes []domain.Perubahan) AuditDetailResponse {
	perubahan := make([]PerubahanResponse, len(changes))
	for i, c := range changes {
		perubahan[i] = PerubahanResponse{Path: c.Path, Jenis: c.Jenis, Sebelum: c.Sebelum, Sesudah: c.Sesudah}
	}
	return AuditDetailResponse{
		AuditRowResponse: toAuditRowResponse(row), Perubahan: perubahan,
		Hash: hex.EncodeToString(row.Hash), HashSebelumnya: hex.EncodeToString(row.HashSebelumnya),
	}
}

// MasalahResponse adalah satu temuan verifikasi rantai.
type MasalahResponse struct {
	ID         idx.ID `json:"id"`
	Jenis      string `json:"jenis"`
	Keterangan string `json:"keterangan"`
}

// VerifikasiResponse adalah hasil satu verifikasi rantai hash.
type VerifikasiResponse struct {
	ID             idx.ID            `json:"id"`
	Mulai          time.Time         `json:"mulai"`
	Selesai        time.Time         `json:"selesai"`
	Pemicu         string            `json:"pemicu"`
	Status         string            `json:"status"`
	JumlahBaris    int64             `json:"jumlah_baris"`
	JumlahSegmen   int64             `json:"jumlah_segmen"`
	JumlahMasalah  int64             `json:"jumlah_masalah"`
	IDAwal         *idx.ID           `json:"id_awal,omitempty"`
	IDAkhir        *idx.ID           `json:"id_akhir,omitempty"`
	HashAkhir      string            `json:"hash_akhir,omitempty"`
	MasalahPertama *MasalahResponse  `json:"masalah_pertama,omitempty"`
	Masalah        []MasalahResponse `json:"masalah,omitempty"`
	Keterangan     *string           `json:"keterangan,omitempty"`
}

// PartisiResponse adalah satu partisi bulanan audit_log yang aktif.
type PartisiResponse struct {
	Nama           string     `json:"nama"`
	Dari           *time.Time `json:"dari"`
	Sampai         *time.Time `json:"sampai"`
	PerkiraanBaris *int64     `json:"perkiraan_baris"`
}

// ArsipResponse adalah satu partisi yang sudah dilepas ke penyimpanan dingin.
type ArsipResponse struct {
	ID             idx.ID    `json:"id"`
	Partisi        string    `json:"partisi"`
	Dari           time.Time `json:"dari"`
	Sampai         time.Time `json:"sampai"`
	JumlahBaris    int64     `json:"jumlah_baris"`
	IDPertama      *idx.ID   `json:"id_pertama,omitempty"`
	IDTerakhir     *idx.ID   `json:"id_terakhir,omitempty"`
	Berkas         string    `json:"berkas"`
	SHA256         string    `json:"sha256"`
	RetensiBulan   int       `json:"retensi_bulan"`
	DiarsipkanPada time.Time `json:"diarsipkan_pada"`
}

// IntegritasResponse adalah status integritas audit: verifikasi terakhir (null bila belum pernah), partisi aktif
// (terbaru lebih dulu), dan arsip (terbaru lebih dulu).
type IntegritasResponse struct {
	VerifikasiTerakhir *VerifikasiResponse `json:"verifikasi_terakhir"`
	Partisi            []PartisiResponse   `json:"partisi"`
	Arsip              []ArsipResponse     `json:"arsip"`
}

// ToIntegritasResponse memetakan ringkasan integritas ke bentuk API; masalah adalah daftar temuan verifikasi terakhir.
func ToIntegritasResponse(v *domain.Verifikasi, masalah []domain.Masalah, partisi []domain.Partisi, arsip []domain.Arsip) IntegritasResponse {
	out := IntegritasResponse{Partisi: make([]PartisiResponse, len(partisi)), Arsip: make([]ArsipResponse, len(arsip))}
	if v != nil {
		r := &VerifikasiResponse{
			ID: v.ID, Mulai: v.Mulai, Selesai: v.Selesai, Pemicu: v.Pemicu, Status: v.Status,
			JumlahBaris: v.JumlahBaris, JumlahSegmen: v.JumlahSegmen, JumlahMasalah: v.JumlahMasalah,
			IDAwal: idOpsional(v.IDAwal), IDAkhir: idOpsional(v.IDAkhir), HashAkhir: hex.EncodeToString(v.HashAkhir),
			Keterangan: v.Keterangan,
		}
		for _, m := range masalah {
			r.Masalah = append(r.Masalah, MasalahResponse{ID: idx.ID(m.ID), Jenis: m.Jenis, Keterangan: m.Keterangan})
		}
		if len(r.Masalah) > 0 {
			r.MasalahPertama = &r.Masalah[0]
		}
		out.VerifikasiTerakhir = r
	}
	for i, p := range partisi {
		out.Partisi[i] = PartisiResponse{Nama: p.Nama, Dari: p.Dari, Sampai: p.Sampai, PerkiraanBaris: p.PerkiraanBaris}
	}
	for i, a := range arsip {
		out.Arsip[i] = ArsipResponse{
			ID: a.ID, Partisi: a.Partisi, Dari: a.Dari, Sampai: a.Sampai, JumlahBaris: a.JumlahBaris,
			IDPertama: idOpsional(a.IDPertama), IDTerakhir: idOpsional(a.IDTerakhir), Berkas: a.Berkas,
			SHA256: a.SHA256Berkas, RetensiBulan: a.RetensiBulan, DiarsipkanPada: a.DiarsipkanPada,
		}
	}
	return out
}

func idOpsional(v *int64) *idx.ID {
	if v == nil {
		return nil
	}
	id := idx.ID(*v)
	return &id
}
