package middleware

import (
	"net/http"
	"sync"
	"time"

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
		return false
	}

	active = append(active, now)
	rl.requests[ip] = active
	return true
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

// LoginRateLimit is a stricter rate limiter specifically for login endpoints.
func LoginRateLimit() gin.HandlerFunc {
	// 5 attempts per minute per IP
	return RateLimit(5, time.Minute)
}
