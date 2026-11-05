// Package ingestion merakit engine ingestion untuk proses nyata (cmd/api dan
// cmd/ingest): konektor terdaftar, Fetcher HTTP berlapis (batas laju, robots.txt,
// retry, circuit breaker), dan penyegaran agregat gold setelah data masuk.
package ingestion

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	"github.com/garda-inflasi/api/internal/ingestion/connector/bmkg"
	"github.com/garda-inflasi/api/internal/ingestion/connector/bps"
	"github.com/garda-inflasi/api/internal/ingestion/connector/pihps"
	"github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/ingestion/engine"
	"github.com/garda-inflasi/api/internal/ingestion/repository/postgres"
	"github.com/garda-inflasi/api/internal/ingestion/scheduler"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// Variabel lingkungan ingestion (semua opsional).
const (
	// EnvUserAgent mengganti User-Agent permintaan keluar.
	EnvUserAgent = "INGEST_USER_AGENT"
	// EnvScheduler menyalakan penjadwal di cmd/api ("on", "true", atau "1"). Bawaan mati
	// agar lingkungan lokal dan demo tidak memanggil layanan eksternal.
	EnvScheduler = "INGEST_SCHEDULER"
	// EnvJedaMs mengganti jeda minimum (milidetik) antar permintaan ke host yang sama.
	EnvJedaMs = "INGEST_JEDA_MS"
)

const (
	userAgentBawaan = "garda-inflasi-ingest/1.0 (+https://gardainflasi.id; pemantauan inflasi pangan Aceh)"
	jedaBawaan      = 1200 * time.Millisecond
	timeoutHTTP     = 30 * time.Second

	percobaan    = 3
	backoffAwal  = 2 * time.Second
	ambangSirkit = 3
	tungguSirkit = 5 * time.Minute
)

// Konektor mengembalikan konektor yang dikenal engine. Kunci rahasia dibaca dari lingkungan.
func Konektor() []domain.Konektor {
	return []domain.Konektor{pihps.New(), bmkg.New(), bps.New(os.Getenv(bps.EnvKunci))}
}

// NewFetcher merakit Fetcher nyata: HTTP -> retry -> circuit breaker.
func NewFetcher() domain.Fetcher {
	ua := os.Getenv(EnvUserAgent)
	if ua == "" {
		ua = userAgentBawaan
	}
	jeda := jedaBawaan
	if ms, err := strconv.Atoi(os.Getenv(EnvJedaMs)); err == nil && ms >= 0 {
		jeda = time.Duration(ms) * time.Millisecond
	}
	httpFetcher := engine.NewHTTPFetcher(ua, jeda, timeoutHTTP)
	return &engine.Breaker{
		Next:   &engine.Retry{Next: httpFetcher, Percobaan: percobaan, Awal: backoffAwal},
		Ambang: ambangSirkit, Tunggu: tungguSirkit,
	}
}

// NewRunner merakit Runner nyata di atas db. Setelah baris silver ditulis, agregat
// dashboard (termasuk gold_harga_harian) disegarkan di latar belakang.
func NewRunner(db *gorm.DB, log *logger.Logger) *engine.Runner {
	r := engine.New(postgres.New(db), NewFetcher(), log, Konektor()...)
	r.SetelahSilver = refresher.Shared(db, log).Trigger
	return r
}

// SchedulerAktif benar bila penjadwal diaktifkan lewat lingkungan.
func SchedulerAktif() bool {
	switch strings.ToLower(os.Getenv(EnvScheduler)) {
	case "on", "true", "1":
		return true
	}
	return false
}

// NewScheduler merakit penjadwal di atas Runner nyata.
func NewScheduler(db *gorm.DB, log *logger.Logger) *scheduler.Penjadwal {
	return scheduler.New(postgres.New(db), NewRunner(db, log), log)
}
