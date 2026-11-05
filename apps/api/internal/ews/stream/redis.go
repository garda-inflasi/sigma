package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// RedisChannel adalah kanal pub/sub sinyal peristiwa EWS.
const RedisChannel = "garda:ews:events"

// RedisBus menyebarkan sinyal lewat Redis pub/sub supaya koneksi SSE di semua
// instance API terbangun segera. Bila Redis gagal, sinyal tetap membangunkan
// proses ini dan instance lain jatuh ke polling basis data.
type RedisBus struct {
	client *redis.Client
	hub    *Hub
	log    *logger.Logger
}

var _ Bus = (*RedisBus)(nil)

// NewRedisBus terhubung ke redisURL (mis. redis://localhost:6379) dan memastikan server menjawab.
func NewRedisBus(ctx context.Context, redisURL string, hub *Hub, log *logger.Logger) (*RedisBus, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("REDIS_URL tidak valid: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis tidak menjawab: %w", err)
	}
	return &RedisBus{client: client, hub: hub, log: log}, nil
}

func (b *RedisBus) Notify(ctx context.Context) {
	if err := b.client.Publish(ctx, RedisChannel, "1").Err(); err != nil {
		b.log.Warn("ews: publish redis gagal, hanya membangunkan proses ini", map[string]interface{}{"error": err.Error()})
		b.hub.Wake()
	}
}

// Run berlangganan ke kanal dan membangunkan hub untuk tiap pesan. go-redis
// menyambung ulang otomatis; Run kembali setelah ctx selesai dan klien ditutup.
func (b *RedisBus) Run(ctx context.Context) {
	defer func() { _ = b.client.Close() }()
	sub := b.client.Subscribe(ctx, RedisChannel)
	defer func() { _ = sub.Close() }()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			b.hub.Wake()
		}
	}
}
