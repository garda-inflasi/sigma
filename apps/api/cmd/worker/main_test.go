package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/platform/pgtest"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var pg *pgtest.Server

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m, &pg)) }

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func quiet() *logger.Logger { return logger.New(logger.ERROR, os.Stderr) }

func TestBuildSchedulerRegistersDefaultJobs(t *testing.T) {
	s, err := buildScheduler(deps{log: quiet(), getenv: env(nil)})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"dashboard_refresh", "kpi_hitung"}, s.Names())
	for _, st := range s.Statuses() {
		assert.NotEqual(t, "0s", st.Interval, st.Name)
	}
}

func TestBuildDaemonsIngestionOnlyWhenEnabled(t *testing.T) {
	for _, nilai := range []string{"", "off", "false", "0"} {
		daemons, err := buildDaemons(deps{log: quiet(), getenv: env(map[string]string{"INGEST_SCHEDULER": nilai})})
		require.NoError(t, err)
		assert.Empty(t, daemons, "INGEST_SCHEDULER=%q tidak boleh menyalakan penjadwal", nilai)
	}
	for _, nilai := range []string{"on", "true", "1", " ON "} {
		daemons, err := buildDaemons(deps{log: quiet(), getenv: env(map[string]string{"INGEST_SCHEDULER": nilai})})
		require.NoError(t, err)
		assert.Len(t, daemons, 1, "INGEST_SCHEDULER=%q", nilai)
	}
}

func TestBuildSchedulerReadsIntervalsFromEnv(t *testing.T) {
	s, err := buildScheduler(deps{log: quiet(), getenv: env(map[string]string{
		"JOB_DASHBOARD_REFRESH_INTERVAL": "30s",
		"JOB_KPI_HITUNG_INTERVAL":        "off",
	})})
	require.NoError(t, err)
	got := map[string]string{}
	for _, st := range s.Statuses() {
		got[st.Name] = st.Interval
	}
	assert.Equal(t, "30s", got["dashboard_refresh"])
	assert.Equal(t, "0s", got["kpi_hitung"])
}

func TestBuildSchedulerRejectsBadEnv(t *testing.T) {
	for k, v := range map[string]string{
		"JOB_DASHBOARD_REFRESH_INTERVAL": "sering",
		"JOB_KPI_HITUNG_BULAN":           "0",
	} {
		_, err := buildScheduler(deps{log: quiet(), getenv: env(map[string]string{k: v})})
		assert.Error(t, err, k)
	}
}

func TestHealthAddr(t *testing.T) {
	assert.Equal(t, ":8081", healthAddr(env(nil)))
	assert.Equal(t, "", healthAddr(env(map[string]string{"WORKER_HEALTH_ADDR": "off"})))
	assert.Equal(t, ":9000", healthAddr(env(map[string]string{"WORKER_HEALTH_ADDR": " :9000 "})))
}

// Menjalankan job sungguhan terhadap skema hasil migrasi: refresh matview
// dashboard dan perhitungan KPI selesai tanpa galat, dan /healthz melaporkannya.
func TestRunJobsAgainstMigratedDatabase(t *testing.T) {
	db := pg.NewDatabase(t, "worker_jobs")
	pgtest.MigrateUp(t, db)

	s, err := buildScheduler(deps{db: db, log: quiet(), getenv: env(nil)})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, name := range []string{"dashboard_refresh", "kpi_hitung"} {
		ran, err := s.RunOnce(ctx, name)
		require.NoError(t, err, name)
		assert.True(t, ran, name)
	}

	rec := httptest.NewRecorder()
	healthHandler(db, s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), `"dashboard_refresh"`))
	assert.False(t, strings.Contains(rec.Body.String(), "last_error"), rec.Body.String())
}
