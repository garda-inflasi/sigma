package prodcheck

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ssoCallbackPath adalah rute BFF Next.js yang menerima pengalihan dari penyedia SSO; redirect URL
// terdaftar di Google harus tepat menunjuk ke sini.
const ssoCallbackPath = "/api/auth/sso/google/callback"

// ValidateSSO memeriksa konfigurasi SSO Google bila dinyalakan (SSO_GOOGLE_CLIENT_ID terisi). SSO
// opsional: tanpa client id dan tanpa variabel SSO lain, tidak ada yang diperiksa. Konfigurasi setengah
// jadi (rahasia atau redirect URL terisi tanpa client id) ditolak karena hampir pasti salah ketik.
// Di luar produksi selalu nil. Pesan menyebut NAMA variabel dan alasannya, tidak pernah nilainya.
func ValidateSSO(getenv func(string) string) error {
	if !IsProduction(getenv) {
		return nil
	}
	value := func(key string) string { return strings.TrimSpace(getenv(key)) }
	clientID := value("SSO_GOOGLE_CLIENT_ID")
	if clientID == "" {
		if value("SSO_GOOGLE_CLIENT_SECRET") != "" || value("SSO_GOOGLE_REDIRECT_URL") != "" {
			return errors.New("SSO_GOOGLE_CLIENT_ID kosong padahal SSO_GOOGLE_CLIENT_SECRET atau SSO_GOOGLE_REDIRECT_URL diisi; lengkapi atau kosongkan semuanya")
		}
		return nil
	}

	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	switch secret := value("SSO_GOOGLE_CLIENT_SECRET"); {
	case secret == "":
		add("SSO_GOOGLE_CLIENT_SECRET wajib diisi bila SSO Google dinyalakan")
	default:
		if why := weakMarker(secret); why != "" {
			add("SSO_GOOGLE_CLIENT_SECRET %s", why)
		}
	}

	if issuer := value("SSO_GOOGLE_ISSUER"); issuer != "" {
		if why := publicHTTPS(issuer); why != "" {
			add("SSO_GOOGLE_ISSUER %s", why)
		}
	}

	redirect := value("SSO_GOOGLE_REDIRECT_URL")
	switch {
	case redirect == "":
		add("SSO_GOOGLE_REDIRECT_URL wajib diisi bila SSO Google dinyalakan")
	default:
		if why := publicHTTPS(redirect); why != "" {
			add("SSO_GOOGLE_REDIRECT_URL %s", why)
		} else if u, _ := url.Parse(redirect); u.Path != ssoCallbackPath || u.RawQuery != "" {
			add("SSO_GOOGLE_REDIRECT_URL harus tepat menunjuk ke %s (tanpa query)", ssoCallbackPath)
		} else if !originAllowed(u, getenv("APP_ALLOWED_ORIGINS")) {
			add("SSO_GOOGLE_REDIRECT_URL harus berasal dari salah satu origin di APP_ALLOWED_ORIGINS")
		}
	}

	if domain := strings.ToLower(value("SSO_GOOGLE_HOSTED_DOMAIN")); domain != "" {
		if strings.ContainsAny(domain, "@/: ") || !strings.Contains(domain, ".") {
			add("SSO_GOOGLE_HOSTED_DOMAIN harus berupa nama domain (mis. kemenkeu.go.id), bukan email atau URL")
		}
	}
	return errors.Join(errs...)
}

// weakMarker memeriksa penanda nilai contoh pada nilai rahasia (tanpa aturan panjang JWT_SECRET).
func weakMarker(value string) string {
	lower := strings.ToLower(value)
	for _, m := range weakMarkers {
		if strings.Contains(lower, m) {
			return "mengandung penanda nilai contoh/bawaan"
		}
	}
	return ""
}

// publicHTTPS: alasan penolakan bila raw bukan URL https absolut ke host publik, selain itu "".
func publicHTTPS(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Fragment != "" {
		return "harus berupa URL absolut tanpa fragmen"
	}
	if u.Scheme != "https" {
		return "harus memakai https di production"
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return "tidak boleh menunjuk ke localhost di production"
	}
	if strings.HasSuffix(u.Hostname(), ".localhost") {
		return "tidak boleh menunjuk ke localhost di production"
	}
	return ""
}

// originAllowed melaporkan apakah origin (skema + host) u ada di daftar APP_ALLOWED_ORIGINS.
func originAllowed(u *url.URL, allowed string) bool {
	want := strings.ToLower(u.Scheme + "://" + u.Host)
	for _, o := range strings.Split(allowed, ",") {
		if strings.EqualFold(strings.TrimRight(strings.TrimSpace(o), "/"), want) {
			return true
		}
	}
	return false
}
