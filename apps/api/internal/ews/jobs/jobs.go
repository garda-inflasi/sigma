// Package jobs merakit komponen EWS yang dipakai bersama oleh route HTTP (cmd/api)
// dan job berkala (cmd/worker): rule engine terjadwal, aliran SSE, dan notifikasi.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	authusecase "github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/ews/engine"
	"github.com/garda-inflasi/api/internal/ews/notifikasi"
	"github.com/garda-inflasi/api/internal/ews/repository/postgres"
	"github.com/garda-inflasi/api/internal/ews/stream"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// DefaultEvaluasiInterval adalah jeda bawaan rule engine. Eskalasi otomatis mati
// sampai EWS_ESKALASI_MENIT diisi.
const DefaultEvaluasiInterval = 15 * time.Minute

// Config mengatur pekerjaan latar belakang. Interval 0 menonaktifkan pekerjaannya.
type Config struct {
	// EvaluasiInterval: rule engine (EWS_EVAL_INTERVAL).
	EvaluasiInterval time.Duration
	// NotifInterval: pengiriman notifikasi dan pemeriksaan eskalasi (EWS_NOTIF_INTERVAL).
	NotifInterval time.Duration
	// EskalasiSetelah: alert BARU berseverity SIAGA atau lebih yang tak diakui selama ini
	// dieskalasi otomatis (EWS_ESKALASI_MENIT; 0 = mati).
	EskalasiSetelah time.Duration
}

// ConfigFromEnv membaca EWS_EVAL_INTERVAL dan EWS_NOTIF_INTERVAL (durasi Go, mis. "15m";
// "0" atau "off" mematikan) serta EWS_ESKALASI_MENIT. Nilai kosong memakai bawaan; nilai
// tak valid memakai bawaan dan ikut dilaporkan sebagai galat.
func ConfigFromEnv() (Config, error) {
	cfg := Config{EvaluasiInterval: DefaultEvaluasiInterval, NotifInterval: notifikasi.DefaultInterval}
	var errs []error

	if d, err := ParseInterval(os.Getenv("EWS_EVAL_INTERVAL"), cfg.EvaluasiInterval); err != nil {
		errs = append(errs, fmt.Errorf("EWS_EVAL_INTERVAL: %w", err))
	} else {
		cfg.EvaluasiInterval = d
	}
	if d, err := ParseInterval(os.Getenv("EWS_NOTIF_INTERVAL"), cfg.NotifInterval); err != nil {
		errs = append(errs, fmt.Errorf("EWS_NOTIF_INTERVAL: %w", err))
	} else {
		cfg.NotifInterval = d
	}
	if raw := strings.TrimSpace(os.Getenv("EWS_ESKALASI_MENIT")); raw != "" {
		if menit, err := strconv.Atoi(raw); err != nil || menit < 0 {
			errs = append(errs, fmt.Errorf("EWS_ESKALASI_MENIT: %q bukan bilangan bulat >= 0", raw))
		} else {
			cfg.EskalasiSetelah = time.Duration(menit) * time.Minute
		}
	}
	return cfg, errors.Join(errs...)
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

// Jeda polling basis data oleh koneksi SSE. Dengan Redis sinyal sampai antar-instance dan
// polling hanya jaring pengaman; tanpa Redis sinyal hanya membangunkan proses sendiri.
const (
	pollDenganRedis = 30 * time.Second
	pollTanpaRedis  = 3 * time.Second
	redisPingWindow = 3 * time.Second
)

// Peristiwa SSE dibuang setelah masa ini; klien yang tertinggal lebih lama cukup memuat
// ulang daftar alert.
const (
	retensiEvent        = 7 * 24 * time.Hour
	intervalBersihEvent = time.Hour
)

// Jobs menampung komponen EWS bersama untuk satu koneksi basis data.
type Jobs struct {
	Engine     *engine.Engine
	Streamer   *stream.Streamer
	Notifikasi *notifikasi.Service

	cfg   Config
	bus   stream.Bus
	store domain.EventRepository
	log   *logger.Logger
}

var (
	sharedMu sync.Mutex
	shared   = map[*gorm.DB]*Jobs{}
)

// Shared mengembalikan satu Jobs per koneksi basis data, agar route dan job berkala dalam
// satu proses memakai komponen yang sama. Konfigurasi dibaca dari env saat pertama dibuat.
func Shared(db *gorm.DB, log *logger.Logger) *Jobs {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if j, ok := shared[db]; ok {
		return j
	}
	cfg, err := ConfigFromEnv()
	if err != nil {
		log.Warn("ews: konfigurasi env tidak valid, memakai bawaan untuk nilai tersebut", map[string]interface{}{"error": err.Error()})
	}

	repo, hub, auditWriter := postgres.New(db), stream.NewHub(), audit.NewWriter(db)
	bus, poll := newBus(os.Getenv("REDIS_URL"), hub, log)
	notif := notifikasi.New(repo, authusecase.RolesWithPermission(notifikasi.PermissionPenerima),
		notifikasi.Config{Interval: cfg.NotifInterval, EskalasiSetelah: cfg.EskalasiSetelah, VAPIDPublicKey: notifikasi.VAPIDFromEnv().PublicKey},
		auditWriter, log, notifikasi.DariEnv(repo)...)

	j := &Jobs{
		Engine:     engine.New(repo, auditWriter, log, stream.NewPublisher(bus), notif),
		Streamer:   stream.NewStreamer(repo, hub, poll),
		Notifikasi: notif,
		cfg:        cfg, bus: bus, store: repo, log: log,
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

// RunStream menjalankan penerima sinyal Redis yang membangunkan koneksi SSE di proses ini.
// Dijalankan setiap instance API; kembali setelah ctx selesai.
func (j *Jobs) RunStream(ctx context.Context) { j.bus.Run(ctx) }

// RunBerkala menjalankan job berkala (rule engine terjadwal, pengiriman notifikasi dan
// eskalasi, pembersihan peristiwa lama) dan kembali setelah semuanya berhenti karena ctx
// selesai. Pemiliknya cmd/worker; aman bila lebih dari satu proses menjalankannya karena
// alert idempoten per kondisi dan notifikasi diklaim berlease.
func (j *Jobs) RunBerkala(ctx context.Context) {
	var wg sync.WaitGroup
	for _, job := range []func(){
		func() { j.Engine.Run(ctx, j.cfg.EvaluasiInterval) },
		func() { j.Notifikasi.Run(ctx) },
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
