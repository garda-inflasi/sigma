package main

import (
	"context"
	"sync"

	ewsjobs "github.com/garda-inflasi/api/internal/ews/jobs"
	intervensiworkflow "github.com/garda-inflasi/api/internal/intervensi/workflow"
)

// Job berkala EWS dan workflow intervensi: rule engine terjadwal (EWS_EVAL_INTERVAL),
// pengiriman notifikasi dan eskalasi (EWS_NOTIF_INTERVAL, EWS_ESKALASI_MENIT), pembersihan
// peristiwa SSE lama, dan eskalasi SLA kartu (INTERVENSI_SLA_INTERVAL). Interval "off"
// mematikan masing-masing. Alert yang ditulis engine di sini juga menjadi kartu intervensi
// otomatis, dan sinyalnya sampai ke koneksi SSE di api lewat Redis atau outbox ews_event.
func init() {
	registerDaemon(func(d deps) (daemon, error) {
		ews := ewsjobs.Shared(d.db, d.log)
		workflow := intervensiworkflow.Shared(d.db, d.log)
		ews.Engine.AddObserver(workflow)
		return func(ctx context.Context) {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); ews.RunBerkala(ctx) }()
			go func() { defer wg.Done(); workflow.Run(ctx) }()
			wg.Wait()
		}, nil
	})
}
