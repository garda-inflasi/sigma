package middleware

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultAPIRateLimit adalah batas bawaan permintaan API per menit per alamat klien. Satu
// halaman dashboard memanggil belasan endpoint ditambah polling, dan satu kantor dapat berbagi
// satu IP publik, jadi batasnya jauh di atas pemakaian wajar satu orang.
const DefaultAPIRateLimit = 600

// DefaultTrustedProxies adalah jaringan proxy yang dipercaya mengisi X-Forwarded-For:
// loopback dan alamat privat (Caddy di jaringan Docker). Header dari alamat lain diabaikan
// sehingga klien tidak dapat memalsukan IP-nya untuk mengelak dari pembatasan laju atau
// mengotori IP di audit trail.
var DefaultTrustedProxies = []string{
	"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "::1/128", "fc00::/7",
}

// RateLimitFromEnv membaca batas permintaan per menit (bilangan bulat > 0). Kosong memakai
// def; nilai tidak valid memakai def dan dilaporkan sebagai galat.
func RateLimitFromEnv(getenv func(string) string, key string, def int) (int, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def, fmt.Errorf("%s=%q harus bilangan bulat > 0", key, raw)
	}
	return n, nil
}

// TrustedProxiesFromEnv mengurai daftar CIDR/IP proxy tepercaya dipisah koma. Kosong memakai
// DefaultTrustedProxies; "none" berarti tidak ada proxy yang dipercaya (IP dari koneksi langsung).
func TrustedProxiesFromEnv(raw string) []string {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "":
		return DefaultTrustedProxies
	case "none":
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
