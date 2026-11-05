// Perintah worker menjalankan job berkala di luar proses API: penyegaran
// agregat dashboard dan perhitungan KPI (job lain mendaftar lewat
// register_<modul>.go) serta daemon seperti penjadwal cron ingestion
// (INGEST_SCHEDULER=on). Aman dijalankan lebih dari satu instance: tiap job
// dilindungi advisory lock Postgres, dan eksekusi ingestion dijaga indeks unik
// satu run BERJALAN per sumber.
//
//	worker                 jalankan semua job sesuai interval (bawaan)
//	worker list            cetak job terdaftar dan interval efektifnya
//	worker run <job>       jalankan satu putaran job lalu keluar
//
// Konfigurasi (env): koneksi DB_* seperti API; JOB_<NAMA>_INTERVAL (mis.
// JOB_DASHBOARD_REFRESH_INTERVAL=5m, "off" untuk menonaktifkan);
// WORKER_HEALTH_ADDR (bawaan :8081, "off" untuk mematikan endpoint /healthz).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/garda-inflasi/api/internal/platform/jobs"
	"github.com/garda-inflasi/api/internal/platform/prodcheck"
	"github.com/garda-inflasi/api/pkg/database"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

func main() {
	log := logger.NewDefault()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout, log); err != nil {
		log.Error("worker berhenti dengan galat", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer, log *logger.Logger) error {
	if err := prodcheck.ValidateWorker(getenv); err != nil {
		return fmt.Errorf("konfigurasi production tidak aman: %w", err)
	}
	db, err := database.New(database.DefaultConfig())
	if err != nil {
		return fmt.Errorf("hubungkan basis data: %w", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	sched, err := buildScheduler(deps{db: db, log: log, getenv: getenv})
	if err != nil {
		return err
	}

	switch {
	case len(args) == 0:
		daemons, err := buildDaemons(deps{db: db, log: log, getenv: getenv})
		if err != nil {
			return err
		}
		return serve(ctx, db, sched, daemons, getenv, log)
	case args[0] == "list":
		for _, st := range sched.Statuses() {
			fmt.Fprintf(out, "%s\tinterval=%s\n", st.Name, st.Interval)
		}
		return nil
	case args[0] == "run" && len(args) == 2:
		ran, err := sched.RunOnce(ctx, args[1])
		if err != nil {
			return err
		}
		if !ran {
			fmt.Fprintf(out, "%s: dilewati, sedang dijalankan instance lain\n", args[1])
			return nil
		}
		fmt.Fprintf(out, "%s: selesai\n", args[1])
		return nil
	}
	return errors.New("pemakaian: worker [list | run <job>]")
}

func serve(ctx context.Context, db *gorm.DB, sched *jobs.Scheduler, daemons []daemon, getenv func(string) string, log *logger.Logger) error {
	if addr := healthAddr(getenv); addr != "" {
		srv := &http.Server{Addr: addr, Handler: healthHandler(db, sched), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("endpoint kesehatan worker gagal", map[string]interface{}{"error": err.Error()})
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		}()
	}
	var wg sync.WaitGroup
	for _, dm := range daemons {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dm(ctx)
		}()
	}

	log.Info("worker berjalan", map[string]interface{}{"jobs": sched.Names(), "daemon": len(daemons)})
	sched.Run(ctx)
	wg.Wait()
	log.Info("worker berhenti")
	return nil
}

func healthAddr(getenv func(string) string) string {
	v := strings.TrimSpace(getenv("WORKER_HEALTH_ADDR"))
	switch strings.ToLower(v) {
	case "":
		return ":8081"
	case "off", "false", "0":
		return ""
	}
	return v
}

func healthHandler(db *gorm.DB, sched *jobs.Scheduler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if sqlDB, err := db.DB(); err != nil || sqlDB.PingContext(r.Context()) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "db_unreachable"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "jobs": sched.Statuses()})
	})
	return mux
}
