package dto

import (
	"strings"
	"time"
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

// SessionsRevokedResponse melaporkan jumlah sesi yang dicabut oleh satu aksi massal.
type SessionsRevokedResponse struct {
	Dicabut int `json:"dicabut"`
}

// ForceLogoutResponse adalah hasil paksa keluar satu pengguna oleh admin.
type ForceLogoutResponse struct {
	ID      string `json:"id"`
	Dicabut int    `json:"dicabut"`
}

// RingkasPerangkat meringkas user agent menjadi label pendek, mis. "Chrome di macOS". Heuristik sederhana
// (cukup untuk membedakan perangkat di daftar sesi), bukan parser UA lengkap; urutan pemeriksaan penting
// karena hampir semua UA memuat "Mozilla", "Safari", dan kadang "Chrome" sekaligus.
func RingkasPerangkat(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return "Perangkat tidak dikenal"
	}
	peramban := ringkasPeramban(ua)
	os := ringkasSistemOperasi(ua)
	switch {
	case peramban != "" && os != "":
		return peramban + " di " + os
	case peramban != "":
		return peramban
	case os != "":
		return os
	default:
		return "Perangkat tidak dikenal"
	}
}

func ringkasPeramban(ua string) string {
	rules := []struct{ token, nama string }{
		{"Edg/", "Edge"}, {"EdgA/", "Edge"}, {"EdgiOS/", "Edge"},
		{"OPR/", "Opera"}, {"Opera", "Opera"},
		{"SamsungBrowser/", "Samsung Internet"},
		{"Firefox/", "Firefox"}, {"FxiOS/", "Firefox"},
		{"CriOS/", "Chrome"}, {"Chromium/", "Chromium"}, {"Chrome/", "Chrome"},
		{"Safari/", "Safari"},
		{"curl/", "curl"},
		{"undici", "Node.js"},
	}
	for _, r := range rules {
		if strings.Contains(ua, r.token) {
			return r.nama
		}
	}
	if strings.HasPrefix(ua, "node") {
		return "Node.js"
	}
	return ""
}

func ringkasSistemOperasi(ua string) string {
	rules := []struct{ token, nama string }{
		{"Windows", "Windows"},
		{"Android", "Android"},
		{"iPhone", "iOS"}, {"iPad", "iPadOS"}, {"iPod", "iOS"},
		{"Mac OS X", "macOS"}, {"Macintosh", "macOS"},
		{"CrOS", "ChromeOS"},
		{"Linux", "Linux"}, {"X11", "Linux"},
	}
	for _, r := range rules {
		if strings.Contains(ua, r.token) {
			return r.nama
		}
	}
	return ""
}
