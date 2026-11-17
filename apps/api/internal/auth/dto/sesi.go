package dto

import (
	"time"

	"github.com/garda-inflasi/api/pkg/useragent"
)

// LogoutRequest adalah isi opsional POST /v1/auth/logout. RefreshToken membantu server menentukan sesi yang
// diakhiri bila access token tidak membawa klaim sid (token dari sebelum fitur sesi).
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// SessionResponse adalah satu sesi aktif milik pengguna (GET /v1/auth/sessions). id adalah id sesi untuk
// pencabutan; saat_ini menandai sesi tempat permintaan ini berasal.
type SessionResponse struct {
	ID                  string    `json:"id"`
	Perangkat           string    `json:"perangkat"`
	UserAgent           string    `json:"user_agent,omitempty"`
	IP                  string    `json:"ip,omitempty"`
	DibuatPada          time.Time `json:"dibuat_pada"`
	TerakhirDipakaiPada time.Time `json:"terakhir_dipakai_pada"`
	KedaluwarsaPada     time.Time `json:"kedaluwarsa_pada"`
	SaatIni             bool      `json:"saat_ini"`
}

// RiwayatSesiRequest adalah query GET /v1/auth/me/riwayat-sesi.
type RiwayatSesiRequest struct {
	Status string `form:"status" binding:"omitempty,oneof=semua aktif berakhir"`
	Limit  int    `form:"limit" binding:"omitempty,min=1"`
	Cursor string `form:"cursor"`
}

// RiwayatSesiResponse adalah satu sesi pada riwayat (GET /v1/auth/me/riwayat-sesi). status: AKTIF, DICABUT, atau
// KEDALUWARSA; berakhir_pada kosong selama sesi aktif.
type RiwayatSesiResponse struct {
	ID                  string     `json:"id"`
	Perangkat           string     `json:"perangkat"`
	IP                  string     `json:"ip,omitempty"`
	MulaiPada           time.Time  `json:"mulai_pada"`
	TerakhirDipakaiPada time.Time  `json:"terakhir_dipakai_pada"`
	BerakhirPada        *time.Time `json:"berakhir_pada,omitempty"`
	Status              string     `json:"status"`
	SaatIni             bool       `json:"saat_ini"`
}

// RiwayatSesiPageResponse adalah satu halaman riwayat sesi; next_cursor kosong pada halaman terakhir.
type RiwayatSesiPageResponse struct {
	Items      []RiwayatSesiResponse `json:"items"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

// SessionsRevokedResponse melaporkan jumlah sesi yang dicabut oleh satu aksi massal.
type SessionsRevokedResponse struct {
	Dicabut int `json:"dicabut"`
}

// ForceLogoutResponse adalah hasil paksa keluar satu pengguna oleh admin.
type ForceLogoutResponse struct {
	ID      string `json:"id"`
	Dicabut int    `json:"dicabut"`
}

// RingkasPerangkat meringkas user agent menjadi label pendek, mis. "Chrome di macOS" (lihat useragent.Ringkas).
func RingkasPerangkat(ua string) string { return useragent.Ringkas(ua) }
