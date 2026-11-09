package worker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/kpi/repository/postgres"
	"github.com/garda-inflasi/api/internal/kpi/usecase"
	"github.com/garda-inflasi/api/internal/platform/jobs"
)

const (
	kpiHitungInterval = time.Hour
	kpiBulanBawaan    = 2
	kpiBulanMaks      = 36
)

func init() {
	registerBuilder(func(d deps) ([]jobs.Job, error) {
		interval, err := jobs.IntervalFromEnv(d.getenv, "kpi_hitung", kpiHitungInterval)
		if err != nil {
			return nil, err
		}
		bulan := kpiBulanBawaan
		if raw := strings.TrimSpace(d.getenv("JOB_KPI_HITUNG_BULAN")); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > kpiBulanMaks {
				return nil, fmt.Errorf("JOB_KPI_HITUNG_BULAN=%q harus bilangan 1..%d", raw, kpiBulanMaks)
			}
			bulan = n
		}
		uc := usecase.New(postgres.New(d.db), audit.NewWriter(d.db), d.log)
		return []jobs.Job{{
			Name:       "kpi_hitung",
			Interval:   interval,
			Timeout:    5 * time.Minute,
			RunOnStart: true,
			// Setara `gardactl kpi hitung -bulan N`: idempoten, nilai sama tidak ditulis ulang.
			Run: func(ctx context.Context) error {
				now := time.Now()
				lap, err := uc.Hitung(ctx, now.AddDate(0, -(bulan-1), 0), now, now)
				if err != nil {
					return err
				}
				d.log.Info("kpi: dihitung", map[string]interface{}{
					"baru": lap.Baru, "direvisi": lap.Direvisi, "tetap": lap.Tetap, "tanpa_dasar": lap.TanpaDasar,
				})
				return nil
			},
		}}, nil
	})
}
