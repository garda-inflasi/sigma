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
	"github.com/garda-inflasi/api/internal/ews/engine"
	"github.com/garda-inflasi/api/internal/ews/repository/postgres"
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

// Jobs menampung komponen EWS bersama untuk satu koneksi basis data.
type Jobs struct {
	Engine *engine.Engine
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
	j := &Jobs{Engine: engine.New(postgres.New(db), audit.NewWriter(db), log)}
	shared[db] = j
	return j
}

// Run menjalankan pekerjaan latar belakang sampai ctx selesai.
func (j *Jobs) Run(ctx context.Context, cfg Config) {
	j.Engine.Run(ctx, cfg.EvaluasiInterval)
}
