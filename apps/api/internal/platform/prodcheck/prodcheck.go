// Package prodcheck menolak start proses bila APP_ENV=production tetapi
// konfigurasi rahasia masih bawaan pengembangan atau terlalu lemah. Pesan galat
// menyebut NAMA variabel dan alasannya, tidak pernah nilainya.
package prodcheck

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ProductionEnv adalah nilai APP_ENV yang mengaktifkan pemeriksaan.
const ProductionEnv = "production"

const (
	// MinJWTSecretLen adalah panjang minimum JWT_SECRET (HS256).
	MinJWTSecretLen = 32
	// AESKeyLen adalah panjang tepat AES_KEY (AES-256).
	AESKeyLen        = 32
	minDistinctChars = 8
)

// Nilai bawaan yang tersebar di env.example, compose.yaml, dan kode; tak boleh dipakai di produksi.
var knownDefaults = []string{
	"garda-inflasi-dev-secret-key-change-in-production",
	"0123456789abcdef0123456789abcdef",
}

// Potongan teks yang menandai nilai contoh/bawaan.
var weakMarkers = []string{"change-in-production", "changeme", "ganti_dengan", "ganti-dengan", "dev-secret", "example", "contoh"}

var weakDBPasswords = map[string]bool{"": true, "garda": true, "garda_secret": true, "password": true, "postgres": true, "secret": true}

// IsProduction melaporkan apakah APP_ENV bernilai production.
func IsProduction(getenv func(string) string) bool {
	return strings.EqualFold(strings.TrimSpace(getenv("APP_ENV")), ProductionEnv)
}

func distinct(s string) int {
	seen := map[rune]struct{}{}
	for _, r := range s {
		seen[r] = struct{}{}
	}
	return len(seen)
}

func weak(value string) string {
	lower := strings.ToLower(value)
	for _, d := range knownDefaults {
		if value == d {
			return "memakai nilai bawaan pengembangan"
		}
	}
	for _, m := range weakMarkers {
		if strings.Contains(lower, m) {
			return "mengandung penanda nilai contoh/bawaan"
		}
	}
	if distinct(value) < minDistinctChars {
		return fmt.Sprintf("terlalu monoton (kurang dari %d karakter berbeda)", minDistinctChars)
	}
	return ""
}

// ValidateAPI memeriksa konfigurasi yang dibutuhkan cmd/api. Di luar produksi
// selalu nil. Semua masalah dikumpulkan dalam satu galat.
func ValidateAPI(getenv func(string) string) error {
	if !IsProduction(getenv) {
		return nil
	}
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	jwt := getenv("JWT_SECRET")
	switch {
	case jwt == "":
		add("JWT_SECRET wajib diisi di production")
	case len(jwt) < MinJWTSecretLen:
		add("JWT_SECRET terlalu pendek (minimal %d karakter)", MinJWTSecretLen)
	default:
		if why := weak(jwt); why != "" {
			add("JWT_SECRET %s", why)
		}
	}

	aes := getenv("AES_KEY")
	switch {
	case aes == "":
		add("AES_KEY wajib diisi di production")
	case len(aes) != AESKeyLen:
		add("AES_KEY harus tepat %d byte", AESKeyLen)
	default:
		if why := weak(aes); why != "" {
			add("AES_KEY %s", why)
		}
		if aes == jwt {
			add("AES_KEY tidak boleh sama dengan JWT_SECRET")
		}
	}

	if err := validateOrigins(getenv("APP_ALLOWED_ORIGINS")); err != nil {
		errs = append(errs, err)
	}
	if err := ValidateWorker(getenv); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// ValidateWorker memeriksa konfigurasi basis data yang dipakai semua proses
// server (api, worker). Di luar produksi selalu nil.
func ValidateWorker(getenv func(string) string) error {
	if !IsProduction(getenv) {
		return nil
	}
	if weakDBPasswords[strings.ToLower(getenv("DB_PASSWORD"))] {
		return errors.New("DB_PASSWORD kosong atau memakai sandi bawaan; set sandi kuat di production")
	}
	return nil
}

// validateOrigins: di produksi origin publik wajib diisi dan bukan hanya localhost,
// kalau tidak proteksi CSRF menolak semua penulisan lewat cookie.
func validateOrigins(raw string) error {
	var public int
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || u.Host == "" {
			return fmt.Errorf("APP_ALLOWED_ORIGINS memuat origin tidak valid: %q", o)
		}
		switch u.Hostname() {
		case "localhost", "127.0.0.1", "::1":
		default:
			public++
		}
	}
	if public == 0 {
		return errors.New("APP_ALLOWED_ORIGINS wajib berisi origin publik (bukan hanya localhost) di production")
	}
	return nil
}
