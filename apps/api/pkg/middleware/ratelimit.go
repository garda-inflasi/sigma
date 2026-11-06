package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// RateLimiter provides per-IP rate limiting with a sliding window.
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter.
// limit: max requests per window.
// window: sliding window duration.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		threshold := now.Add(-rl.window)
		for ip, times := range rl.requests {
			var active []time.Time
			for _, t := range times {
				if t.After(threshold) {
					active = append(active, t)
				}
			}
			if len(active) == 0 {
				delete(rl.requests, ip)
			} else {
				rl.requests[ip] = active
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	ok, _ := rl.allow(ip)
	return ok
}

// allow seperti Allow dan, bila ditolak, melaporkan berapa lama lagi sampai permintaan berikutnya boleh lewat:
// sampai entri tertua yang menahan jendela habis. Permintaan yang diizinkan dicatat; yang ditolak tidak.
func (rl *RateLimiter) allow(ip string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	threshold := now.Add(-rl.window)

	times := rl.requests[ip]
	var active []time.Time
	for _, t := range times {
		if t.After(threshold) {
			active = append(active, t)
		}
	}

	if len(active) >= rl.limit {
		rl.requests[ip] = active
		penahan := active[len(active)-rl.limit]
		return false, penahan.Add(rl.window).Sub(now)
	}

	active = append(active, now)
	rl.requests[ip] = active
	return true, 0
}

// RateLimit returns a Gin middleware that rate-limits by client IP.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	rl := NewRateLimiter(limit, window)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.Allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    "429",
				"message": "Terlalu banyak permintaan. Coba lagi nanti.",
			})
			return
		}
		c.Next()
	}
}

// KodeBatasLaju adalah kode galat 429 batas laju auth. Sama dengan kode yang dilahirkan BFF web untuk batas laju
// per IP-nya (apps/web/src/lib/auth/pesan-login.ts), jadi BFF meneruskan jawaban Go tanpa mengubahnya dan form
// menampilkan pesan serta hitung mundur yang sama.
const KodeBatasLaju = "RATE_LIMITED"

// BatasLajuData adalah `data` pada galat 429 RATE_LIMITED. Header Retry-After memuat nilai yang sama.
type BatasLajuData struct {
	// SisaDetik adalah lama tunggu sampai percobaan berikutnya diizinkan, dibulatkan ke atas (minimal 1).
	SisaDetik int `json:"sisa_detik"`
}

// AuthRateLimit membatasi percobaan ke endpoint auth publik (login, validasi OTP, lupa dan atur ulang kata
// sandi, mulai/selesai SSO) per IP klien: paling banyak limit permintaan per window. Kunci adalah c.ClientIP(),
// yang hanya memercayai X-Forwarded-For dari proxy tepercaya (TRUSTED_PROXIES), jadi klien tidak bisa membuka
// ember baru dengan memalsukan header itu. BFF meneruskan IP peramban lewat header yang sama sehingga batasnya
// berlaku juga bagi lalu lintas dari web; Caddy membuka /api/v1/* langsung ke Go sehingga batas ini juga yang
// menahan penebakan kata sandi lintas email di jalur itu (lockout hanya per email).
//
// Ditolak dengan 429, kode RATE_LIMITED, header Retry-After (detik), dan data.sisa_detik: kontrak yang sama
// dengan batas laju BFF. Permintaan yang ditolak tidak sampai ke handler, jadi tidak dihitung lockout. Setiap
// pemanggilan membuat penghitung sendiri; pasang satu instans pada semua endpoint yang harus berbagi ember.
func AuthRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	rl := NewRateLimiter(limit, window)

	return func(c *gin.Context) {
		ok, tunggu := rl.allow(c.ClientIP())
		if ok {
			c.Next()
			return
		}
		detik := max(1, int(math.Ceil(tunggu.Seconds())))
		c.Header("Retry-After", strconv.Itoa(detik))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Envelope{
			Code:    KodeBatasLaju,
			Message: "Terlalu banyak percobaan dari perangkat ini. Coba lagi nanti.",
			Data:    BatasLajuData{SisaDetik: detik},
		})
	}
}
