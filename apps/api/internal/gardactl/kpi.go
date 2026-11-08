package gardactl

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	kpipostgres "github.com/garda-inflasi/api/internal/kpi/repository/postgres"
	"github.com/garda-inflasi/api/internal/kpi/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
)

const (
	bulanBawaan   = 2
	bulanMaks     = 36
	kpiJobTimeout = 5 * time.Minute
)

func runKpi(ctx context.Context, args []string, opts Options) error {
	if len(args) > 0 && args[0] == "hitung" {
		args = args[1:]
	}

	fs := flag.NewFlagSet("kpi", flag.ContinueOnError)
	bulan := fs.Int("bulan", bulanBawaan, "jumlah bulan terakhir yang dihitung, termasuk bulan berjalan")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *bulan < 1 || *bulan > bulanMaks {
		return fmt.Errorf("-bulan harus antara 1 dan %d", bulanMaks)
	}

	db, err := connectDB(opts.getenv)
	if err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	now := opts.now()
	jobCtx, cancel := context.WithTimeout(ctx, kpiJobTimeout)
	defer cancel()

	uc := usecase.New(kpipostgres.New(db), audit.NewWriter(db), logger.NewDefault())
	lap, err := uc.Hitung(jobCtx, now.AddDate(0, -(*bulan-1), 0), now, now)
	if err != nil {
		return err
	}
	fmt.Fprintf(opts.out(), "kpi: baru=%d direvisi=%d tetap=%d tanpa_dasar=%d\n", lap.Baru, lap.Direvisi, lap.Tetap, lap.TanpaDasar)
	return nil
}
