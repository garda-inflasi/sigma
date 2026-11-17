package worker

import (
	"context"
	"errors"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/platform/jobs"
	"github.com/garda-inflasi/api/internal/prediksi/mlclient"
	"github.com/garda-inflasi/api/internal/prediksi/repository/postgres"
	"github.com/garda-inflasi/api/internal/prediksi/usecase"
)

const prediksiRetrainInterval = 168 * time.Hour

func init() {
	registerBuilder(func(d deps) ([]jobs.Job, error) {
		interval, err := jobs.IntervalFromEnv(d.getenv, "prediksi_retrain", prediksiRetrainInterval)
		if err != nil {
			return nil, err
		}
		return []jobs.Job{{
			Name:       "prediksi_retraining",
			Interval:   interval,
			Timeout:    10 * time.Minute,
			RunOnStart: false,
			Run: func(ctx context.Context) error {
				ml := mlclient.FromEnv()
				if ml == nil {
					d.log.Info("prediksi_retraining: ML_SERVICE_URL tidak dikonfigurasi, dilewati", nil)
					return nil
				}
				uc := usecase.NewMLOps(postgres.New(d.db), ml, audit.NewWriter(d.db), d.log)
				ring, err := uc.RetrainOtomatis(ctx, "")
				if errors.Is(err, usecase.ErrLayananMLMati) {
					d.log.Warn("prediksi_retraining: layanan ML tidak tersedia, dicoba lagi pada jadwal berikutnya", nil)
					return nil
				}
				if err != nil {
					return err
				}
				d.log.Info("prediksi_retraining: selesai", map[string]interface{}{
					"komoditas":       ring.Komoditas,
					"dilatih":         ring.Dilatih,
					"dipromosikan":    ring.Dipromosikan,
					"ditolak_gerbang": ring.DitolakGerbang,
					"dilewati":        ring.Dilewati,
					"gagal":           ring.Gagal,
				})
				return nil
			},
		}}, nil
	})
}
