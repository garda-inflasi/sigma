package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	disruptionVersionKey = "spasial:gangguan:versi"
	routeCacheTTL        = 12 * time.Hour
)

// Cache membungkus Redis opsional untuk cache rute berbasis versi gangguan jalan.
type Cache struct {
	client *redis.Client
}

// NewCacheFromEnv menghubungkan Redis bila REDIS_URL diisi; bila kosong mengembalikan cache no-op yang aman.
func NewCacheFromEnv() *Cache {
	raw := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if raw == "" {
		return &Cache{}
	}
	opts, err := redis.ParseURL(raw)
	if err != nil {
		return &Cache{}
	}
	return &Cache{client: redis.NewClient(opts)}
}

// NewCacheWithClient memudahkan pengujian dengan miniredis.
func NewCacheWithClient(client *redis.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) Enabled() bool {
	return c != nil && c.client != nil
}

func (c *Cache) Version(ctx context.Context) string {
	if !c.Enabled() {
		return "0"
	}
	val, err := c.client.Get(ctx, disruptionVersionKey).Result()
	if err != nil || val == "" {
		return "0"
	}
	return val
}

// InvalidateDisruptions menaikkan nomor versi gangguan agar cache rute sebelumnya langsung basi secara atomik.
func (c *Cache) InvalidateDisruptions(ctx context.Context) {
	if !c.Enabled() {
		return
	}
	_ = c.client.Incr(ctx, disruptionVersionKey).Err()
}

func RouteCacheKey(asalLat, asalLng, tujuanLat, tujuanLng float64, profile, version string) string {
	return fmt.Sprintf("spasial:rute:%.4f,%.4f:%.4f,%.4f:%s:v%s", asalLat, asalLng, tujuanLat, tujuanLng, profile, version)
}

func (c *Cache) GetRoute(ctx context.Context, key string, dst any) bool {
	if !c.Enabled() {
		return false
	}
	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

func (c *Cache) SetRoute(ctx context.Context, key string, val any) {
	if !c.Enabled() {
		return
	}
	raw, err := json.Marshal(val)
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, key, raw, routeCacheTTL).Err()
}
