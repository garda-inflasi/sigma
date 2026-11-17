// Package useragent meringkas string User-Agent menjadi label perangkat yang mudah dibaca.
package useragent

import "strings"

// Ringkas meringkas user agent menjadi label pendek, mis. "Chrome di macOS". Heuristik sederhana
// (cukup untuk membedakan perangkat di daftar sesi), bukan parser UA lengkap; urutan pemeriksaan penting
// karena hampir semua UA memuat "Mozilla", "Safari", dan kadang "Chrome" sekaligus.
func Ringkas(ua string) string {
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
