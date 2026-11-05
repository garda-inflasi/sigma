// Perintah kpi menghitung nilai KPI otomatis dari data yang ada. Aman dijalankan
// berulang (cron harian/jam): nilai yang sama tidak ditulis ulang, nilai yang
// berubah menjadi versi baru.
//
//	kpi [-bulan N]   hitung N bulan terakhir termasuk bulan berjalan (bawaan 2)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/kpi/repository/postgres"
	"github.com/garda-inflasi/api/internal/kpi/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	bulanBawaan = 2
	bulanMaks   = 36
	jobTimeout  = 5 * time.Minute
)

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout, time.Now()); err != nil {
		log.Fatalf("kpi: %v", err)
	}
}

func run(args []string, getenv func(string) string, out io.Writer, now time.Time) error {
	fs := flag.NewFlagSet("kpi", flag.ContinueOnError)
	bulan := fs.Int("bulan", bulanBawaan, "jumlah bulan terakhir yang dihitung, termasuk bulan berjalan")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *bulan < 1 || *bulan > bulanMaks {
		return fmt.Errorf("-bulan harus antara 1 dan %d", bulanMaks)
	}
	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL wajib diisi")
	}

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Warn)})
	if err != nil {
		return fmt.Errorf("hubungkan basis data: %w", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()

	uc := usecase.New(postgres.New(db), audit.NewWriter(db), logger.NewDefault())
	lap, err := uc.Hitung(ctx, now.AddDate(0, -(*bulan-1), 0), now, now)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "kpi: baru=%d direvisi=%d tetap=%d tanpa_dasar=%d\n", lap.Baru, lap.Direvisi, lap.Tetap, lap.TanpaDasar)
	return nil
}
