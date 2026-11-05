// Package jobs merakit komponen EWS yang dipakai bersama oleh route HTTP dan
// pekerjaan latar belakang di cmd/api (rule engine terjadwal).
package jobs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/ews/engine"
	"github.com/garda-inflasi/api/internal/ews/repository/postgres"
	"github.com/garda-inflasi/api/internal/ews/stream"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// DefaultEvaluasiInterval adalah jeda bawaan antar putaran rule engine.
const DefaultEvaluasiInterval = 15 * time.Minute

// Config mengatur pekerjaan latar belakang. Interval 0 menonaktifkan pekerjaannya.
type Config struct {
	EvaluasiInterval time.Duration
}

// ConfigFromEnv membaca EWS_EVAL_INTERVAL (durasi Go, mis. "15m"; "0" atau "off"
// untuk mematikan). Nilai kosong memakai bawaan; nilai tak valid menghasilkan galat.
func ConfigFromEnv() (Config, error) {
	interval, err := ParseInterval(os.Getenv("EWS_EVAL_INTERVAL"), DefaultEvaluasiInterval)
	if err != nil {
		return Config{EvaluasiInterval: DefaultEvaluasiInterval}, fmt.Errorf("EWS_EVAL_INTERVAL: %w", err)
	}
	return Config{EvaluasiInterval: interval}, nil
}

// ParseInterval mengurai durasi; kosong = def, "0" atau "off" = 0.
func ParseInterval(raw string, def time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "":
		return def, nil
	case "0", "off":
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return def, fmt.Errorf("durasi %q tidak valid", raw)
	}
	return d, nil
}

// Jeda polling basis data oleh koneksi SSE: jaring pengaman saja bila Redis menyambungkan
// antar-instance, sering bila tidak ada Redis (sinyal hanya membangunkan proses sendiri).
const (
	pollDenganRedis = 30 * time.Second
	pollTanpaRedis  = 3 * time.Second
	redisPingWindow = 3 * time.Second
)

// Peristiwa SSE disimpan selama ini sebelum dibuang; klien yang tertinggal lebih lama
// dari ini cukup memuat ulang daftar alert.
const (
	retensiEvent        = 7 * 24 * time.Hour
	intervalBersihEvent = time.Hour
)

// Jobs menampung komponen EWS bersama untuk satu koneksi basis data.
type Jobs struct {
	Engine   *engine.Engine
	Streamer *stream.Streamer

	bus   stream.Bus
	store domain.EventRepository
	log   *logger.Logger
}

var (
	sharedMu sync.Mutex
	shared   = map[*gorm.DB]*Jobs{}
)

// Shared mengembalikan satu Jobs per koneksi basis data, agar route dan cmd/api
// memakai Engine yang sama.
func Shared(db *gorm.DB, log *logger.Logger) *Jobs {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if j, ok := shared[db]; ok {
		return j
	}
	repo, hub := postgres.New(db), stream.NewHub()
	bus, poll := newBus(os.Getenv("REDIS_URL"), hub, log)
	j := &Jobs{
		Engine:   engine.New(repo, audit.NewWriter(db), log, stream.NewPublisher(bus)),
		Streamer: stream.NewStreamer(repo, hub, poll),
		bus:      bus, store: repo, log: log,
	}
	shared[db] = j
	return j
}

// newBus memilih Redis pub/sub bila REDIS_URL diisi dan Redis menjawab; selain itu bus lokal.
func newBus(redisURL string, hub *stream.Hub, log *logger.Logger) (stream.Bus, time.Duration) {
	if redisURL == "" {
		log.Info("ews: REDIS_URL kosong, SSE memakai polling basis data antar-instance", nil)
		return stream.NewLocalBus(hub), pollTanpaRedis
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisPingWindow)
	defer cancel()
	bus, err := stream.NewRedisBus(ctx, redisURL, hub, log)
	if err != nil {
		log.Warn("ews: Redis tidak tersedia, SSE memakai polling basis data", map[string]interface{}{"error": err.Error()})
		return stream.NewLocalBus(hub), pollTanpaRedis
	}
	return bus, pollDenganRedis
}

// Run menjalankan pekerjaan latar belakang (rule engine terjadwal, penerima sinyal
// Redis, pembersihan peristiwa lama) dan kembali setelah semuanya berhenti karena ctx selesai.
func (j *Jobs) Run(ctx context.Context, cfg Config) {
	var wg sync.WaitGroup
	for _, job := range []func(){
		func() { j.Engine.Run(ctx, cfg.EvaluasiInterval) },
		func() { j.bus.Run(ctx) },
		func() { j.bersihkanEvent(ctx) },
	} {
		wg.Add(1)
		go func() { defer wg.Done(); job() }()
	}
	wg.Wait()
}

// bersihkanEvent membuang peristiwa SSE yang melewati masa retensi.
func (j *Jobs) bersihkanEvent(ctx context.Context) {
	ticker := time.NewTicker(intervalBersihEvent)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := j.store.PurgeEventsBefore(ctx, time.Now().Add(-retensiEvent)); err != nil && ctx.Err() == nil {
				j.log.Warn("ews: gagal membuang event lama", map[string]interface{}{"error": err.Error()})
			}
		}
	}
}
