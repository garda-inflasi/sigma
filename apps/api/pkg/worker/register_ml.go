package worker

import (
	"context"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	ewsjobs "github.com/garda-inflasi/api/internal/ews/jobs"
	ewspostgres "github.com/garda-inflasi/api/internal/ews/repository/postgres"
	"github.com/garda-inflasi/api/internal/ml"
	"github.com/garda-inflasi/api/internal/ml/jembatan"
	mlpostgres "github.com/garda-inflasi/api/internal/ml/repository/postgres"
	"github.com/garda-inflasi/api/internal/platform/jobs"
	"github.com/garda-inflasi/api/pkg/platform/mongodb"
)

// mlJembatanInterval adalah jeda bawaan jembatan EWS pipeline ML. Siklus ML berjalan tiap jam; putaran yang tidak
// menemukan siklus baru tidak menulis apa pun (kunci yang sudah diterapkan dilewati).
const mlJembatanInterval = 5 * time.Minute

// Jembatan EWS pipeline ML (docs/PIPELINE-DATA-ML.md §5.4): sinyal aktif datasafe_ews_signals menjadi ews_alert.
// JOB_ML_JEMBATAN_EWS_INTERVAL mengatur jedanya ("off" mematikan). Tanpa MONGO_URI/MONGO_URL job tidak didaftarkan.
func init() {
	registerBuilder(func(d deps) ([]jobs.Job, error) {
		interval, err := jobs.IntervalFromEnv(d.getenv, "ml_jembatan_ews", mlJembatanInterval)
		if err != nil {
			return nil, err
		}
		if !mongodb.ConfigDariEnv(d.getenv).Dikonfigurasi() {
			// Tanpa Mongo tidak ada yang bisa dijembatani: job tidak didaftarkan sama sekali.
			if interval > 0 && d.log != nil {
				d.log.Info("ml_jembatan_ews: MONGO_URI/MONGO_URL kosong, job tidak didaftarkan", nil)
			}
			return nil, nil
		}
		var j *jembatan.Jembatan
		bangun := func() *jembatan.Jembatan {
			if j == nil {
				ews := ewsjobs.Shared(d.db, d.log)
				j = jembatan.New(ml.DariEnv(d.getenv, d.log), mlpostgres.New(d.db), ewspostgres.New(d.db),
					audit.NewWriter(d.db), d.log, ews.Publisher, ews.Notifikasi)
			}
			return j
		}
		return []jobs.Job{{
			Name:       "ml_jembatan_ews",
			Interval:   interval,
			Timeout:    2 * time.Minute,
			RunOnStart: true,
			Run: func(ctx context.Context) error {
				r, err := bangun().Jalankan(ctx)
				if err != nil {
					return err
				}
				if r.Dibuat+r.Dinaikkan+r.Mereda+r.Galat > 0 {
					d.log.Info("ml_jembatan_ews: selesai", map[string]interface{}{
						"cycle_id": r.CycleID, "sinyal": r.Sinyal, "dibuat": r.Dibuat, "dinaikkan": r.Dinaikkan,
						"dilewati": r.Dilewati, "diredam": r.Diredam, "mereda": r.Mereda, "diabaikan_info": r.DiabaikanInfo,
						"sudah_diterapkan": r.SudahDiterapkan, "galat": r.Galat,
					})
				}
				return nil
			},
		}}, nil
	})
}
