package orchestrator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestratorStatusTanpaKonfigurasi(t *testing.T) {
	orch := New("", "", nil, nil)
	snap := orch.Status(t.Context())
	assert.False(t, snap.Crawler.Configured)
	assert.False(t, snap.Pipeline.Configured)
	assert.Equal(t, "SIAGA", snap.Chain.Tahap)

	_, code, err := orch.TriggerCrawler(t.Context(), TriggerCrawlerRequest{GagalSaja: true})
	assert.ErrorIs(t, err, ErrTidakDikonfigurasi)
	assert.Equal(t, http.StatusServiceUnavailable, code)
}

func TestOrchestratorChainCrawlerKeML(t *testing.T) {
	var mu sync.Mutex
	crawlerCalls := 0
	mlCalls := 0
	crawlerDone := false
	mlDone := false

	crawlerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost && r.URL.Path == "/run" {
			crawlerCalls++
			crawlerDone = true
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "STARTED",
				"state": map[string]interface{}{
					"berjalan":     true,
					"job_id":       "CRW-TEST1",
					"tahap":        "ANTRIAN",
					"progress_pct": 10,
					"logs":         []string{"mulai"},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"berjalan":     !crawlerDone,
			"job_id":       "CRW-TEST1",
			"tahap":        "SELESAI",
			"progress_pct": 100,
			"logs":         []string{"ok"},
		})
	}))
	defer crawlerSrv.Close()

	mlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost && r.URL.Path == "/run" {
			mlCalls++
			mlDone = true
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "STARTED",
				"state": map[string]interface{}{
					"berjalan":     true,
					"job_id":       "MLP-TEST1",
					"tahap":        "EKSTRAKSI_FITUR",
					"progress_pct": 15,
					"logs":         []string{"fitur"},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"berjalan":     !mlDone,
			"job_id":       "MLP-TEST1",
			"cycle_id":     "C20261124T12Z",
			"tahap":        "SELESAI",
			"progress_pct": 100,
			"logs":         []string{"ml selesai"},
		})
	}))
	defer mlSrv.Close()

	orch := New(crawlerSrv.URL, mlSrv.URL, crawlerSrv.Client(), nil)
	snap, code, err := orch.TriggerCrawler(t.Context(), TriggerCrawlerRequest{
		Sumber:    []string{"PIHPS"},
		GagalSaja: false,
		LanjutML:  true,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, code)
	assert.True(t, snap.Chain.Aktif || snap.Chain.Tahap == "SELESAI")

	// Tunggu pemantau rantai selesai (intervalPoll = 800ms -> 2 siklus <= 2.5s)
	require.Eventually(t, func() bool {
		st := orch.Status(t.Context())
		return !st.Chain.Aktif && st.Chain.Tahap == "SELESAI"
	}, 6*time.Second, 100*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, crawlerCalls)
	assert.Equal(t, 1, mlCalls)
}
