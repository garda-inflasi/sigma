package worker

import (
	"context"
	"time"

	ewsjobs "github.com/garda-inflasi/api/internal/ews/jobs"
	"github.com/garda-inflasi/api/internal/platform/jobs"
	siklusjobs "github.com/garda-inflasi/api/internal/siklus/jobs"
)

// Penentuan siklus ekonomi otomatis (08-B), dua jalur yang berbagi satu mesin:
//
//   - job terjadwal siklus_evaluasi (JOB_SIKLUS_EVALUASI_INTERVAL, bawaan 5m, off mematikan): menetapkan siklus aktif
//     dari aturan_siklus dan data nyata, termasuk pemulihan (alert diselesaikan, stok turun) yang tidak ada sinyalnya;
//   - daemon pemicu: begitu rule engine EWS di proses ini menulis alert baru atau menaikkan tingkatnya, siklus
//     dievaluasi ulang (digabung beberapa detik), tanpa menunggu jadwal berikutnya. Mati bersama jobnya (off).
//
// Aman dijalankan bersamaan di banyak instance: evaluasi memegang kunci advisory per wilayah dan job dibungkus kunci
// advisory penjadwal, jadi pergantian siklus tidak pernah ganda. Evaluasi pertama dijalankan saat worker mulai,
// sehingga basis data baru langsung punya siklus aktif.
func init() {
	registerBuilder(func(d deps) ([]jobs.Job, error) {
		interval, err := siklusjobs.IntervalFromEnv(d.getenv)
		if err != nil {
			return nil, err
		}
		return []jobs.Job{{
			Name:       siklusjobs.NamaJob,
			Interval:   interval,
			Timeout:    2 * time.Minute,
			RunOnStart: true,
			Run:        siklusjobs.Shared(d.db, d.log).Evaluasi,
		}}, nil
	})

	registerDaemon(func(d deps) (daemon, error) {
		interval, err := siklusjobs.IntervalFromEnv(d.getenv)
		if err != nil {
			return nil, err
		}
		if interval <= 0 {
			return nil, nil
		}
		pemicu := siklusjobs.Shared(d.db, d.log).Pemicu
		ewsjobs.Shared(d.db, d.log).Engine.AddObserver(pemicu)
		return func(ctx context.Context) { pemicu.Run(ctx) }, nil
	})
}
