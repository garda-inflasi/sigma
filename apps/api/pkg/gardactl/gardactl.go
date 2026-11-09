// Package gardactl mengimplementasikan CLI utilitas terpadu Garda Inflasi:
//
//	gardactl ingest [jalankan|daftar|pemetaan|konfigurasi] [argumen...]
//	gardactl kpi [hitung] [-bulan N]
//	gardactl master impor-kecamatan [-out file.csv] [-periode ...]
package gardactl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/garda-inflasi/api/internal/ingestion/domain"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Options menyediakan konfigurasi eksekusi gardactl (dapat disuntikkan saat pengujian).
type Options struct {
	Getenv     func(string) string
	Out        io.Writer
	Fetcher    domain.Fetcher
	Now        time.Time
	HTTPClient *http.Client
}

func (o *Options) getenv(key string) string {
	if o.Getenv != nil {
		return o.Getenv(key)
	}
	return os.Getenv(key)
}

func (o *Options) out() io.Writer {
	if o.Out != nil {
		return o.Out
	}
	return os.Stdout
}

func (o *Options) now() time.Time {
	if !o.Now.IsZero() {
		return o.Now
	}
	return time.Now()
}

func (o *Options) httpClient() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	return &http.Client{}
}

func connectDB(getenv func(string) string) (*gorm.DB, error) {
	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("DATABASE_URL wajib diisi")
	}
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("hubungkan basis data: %w", err)
	}
	return db, nil
}

// Run menjalankan perintah gardactl dengan argumen dan opsi yang diberikan.
func Run(ctx context.Context, args []string, opts Options) error {
	if len(args) == 0 {
		return errors.New("perintah wajib: gardactl [ingest|kpi|master] [argumen...]")
	}

	switch args[0] {
	case "ingest":
		return runIngest(ctx, args[1:], opts)
	case "kpi":
		return runKpi(ctx, args[1:], opts)
	case "master":
		return runMaster(ctx, args[1:], opts)
	case "help", "--help", "-h":
		fmt.Fprintln(opts.out(), `gardactl - CLI Utilitas Terpadu Garda Inflasi

Pemakaian:
  gardactl ingest [jalankan|daftar|pemetaan|konfigurasi] [argumen...]
  gardactl kpi [hitung] [-bulan N]
  gardactl master impor-kecamatan [-out file.csv] [-periode ...]

Gunakan "gardactl <perintah> -h" untuk bantuan masing-masing perintah.`)
		return nil
	default:
		return fmt.Errorf("perintah %q tidak dikenal (pilih: ingest, kpi, master)", args[0])
	}
}
