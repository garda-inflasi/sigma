// Package worker menjalankan job berkala dan daemon background Garda Inflasi.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/garda-inflasi/api/internal/platform/jobs"
	"github.com/garda-inflasi/api/internal/platform/prodcheck"
	"github.com/garda-inflasi/api/pkg/database"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
)

// Run menjalankan worker daemon dan scheduler sesuai argumen yang diberikan.
func Run(ctx context.Context, args []string, getenv func(string) string, out io.Writer, log *logger.Logger) error {
	if len(args) == 2 && args[0] == "verifikasi-arsip" {
		return verifikasiArsip(args[1], out)
	}
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
	return errors.New("pemakaian: worker [list | run <job> | verifikasi-arsip <manifest.json>]")
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
