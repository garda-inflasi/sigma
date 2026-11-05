package main

import (
	"context"

	"github.com/garda-inflasi/api/internal/ingestion"
	"github.com/garda-inflasi/api/internal/ingestion/scheduler"
)

// Penjadwal cron ingestion berjalan di worker hanya bila INGEST_SCHEDULER=on, agar
// lingkungan lokal dan demo tidak memanggil PIHPS, BMKG, atau BPS. Saat worker
// berhenti, eksekusi yang sedang berjalan dibatalkan lalu ditunggu sampai statusnya
// tercatat, sehingga tidak ada run yang tertinggal berstatus BERJALAN.
func init() {
	registerDaemon(func(d deps) (daemon, error) {
		if !ingestion.SchedulerAktif(d.getenv) {
			return nil, nil
		}
		p := ingestion.NewScheduler(d.db, d.log)
		return func(ctx context.Context) {
			d.log.Info("ingestion: penjadwal cron aktif")
			p.Jalankan(ctx, scheduler.Interval)
			p.Tunggu()
		}, nil
	})
}
