package http

import (
	"bytes"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/garda-inflasi/api/internal/ml/orchestrator"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerJobsDanTrigger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	crawlerSrv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method == nethttp.MethodPost && r.URL.Path == "/run" {
			w.WriteHeader(nethttp.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "STARTED",
				"state": map[string]interface{}{
					"berjalan":     true,
					"job_id":       "CRW-H1",
					"tahap":        "ANTRIAN",
					"progress_pct": 10,
					"logs":         []string{"start"},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"berjalan":     false,
			"job_id":       "CRW-H1",
			"tahap":        "SELESAI",
			"progress_pct": 100,
			"logs":         []string{"done"},
		})
	}))
	defer crawlerSrv.Close()

	mlSrv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method == nethttp.MethodPost && r.URL.Path == "/run" {
			w.WriteHeader(nethttp.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  "409",
				"error": "sedang berjalan",
				"state": map[string]interface{}{
					"berjalan":     true,
					"job_id":       "MLP-H1",
					"tahap":        "TRAIN_REGRESI",
					"progress_pct": 45,
					"logs":         []string{"training"},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"berjalan":     true,
			"job_id":       "MLP-H1",
			"tahap":        "TRAIN_REGRESI",
			"progress_pct": 45,
			"logs":         []string{"training"},
		})
	}))
	defer mlSrv.Close()

	orch := orchestrator.New(crawlerSrv.URL, mlSrv.URL, crawlerSrv.Client(), nil)
	h := New(nil, nil, orch)

	r := gin.New()
	r.GET("/jobs", h.ListJobs)
	r.GET("/jobs/:id", h.GetJob)
	r.POST("/crawler/run", h.TriggerCrawler)
	r.POST("/pipeline/run", h.TriggerPipeline)

	// 1. Trigger crawler -> 202 Accepted
	w := httptest.NewRecorder()
	req := httptest.NewRequest(nethttp.MethodPost, "/crawler/run", bytes.NewBufferString(`{"gagal_saja":true}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, nethttp.StatusAccepted, w.Code)

	// 2. GetJob by ID -> 200 OK
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, httptest.NewRequest(nethttp.MethodGet, "/jobs/CRW-H1", nil))
	assert.Equal(t, nethttp.StatusOK, wGet.Code)

	// 3. Trigger pipeline saat sedang berjalan -> 409 Conflict
	wPipe := httptest.NewRecorder()
	r.ServeHTTP(wPipe, httptest.NewRequest(nethttp.MethodPost, "/pipeline/run", bytes.NewBufferString("{}")))
	require.Equal(t, nethttp.StatusConflict, wPipe.Code)
}
