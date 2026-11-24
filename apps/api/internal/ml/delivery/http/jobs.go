package http

import (
	"encoding/json"
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/ml/orchestrator"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// ListJobs handles GET /v1/ml/jobs: snapshot status realtime worker crawler, ML pipeline, dan rantai eksekusi.
func (h *Handler) ListJobs(c *gin.Context) {
	if h.orch == nil {
		response.Success(c, orchestrator.New("", "", nil, nil).Status(c.Request.Context()))
		return
	}
	response.Success(c, h.orch.Status(c.Request.Context()))
}

// GetJob handles GET /v1/ml/jobs/:id: mengembalikan detail job aktif/terakhir jika cocok dengan crawler, pipeline, atau chain.
func (h *Handler) GetJob(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		response.BadRequest(c, "id job wajib diisi")
		return
	}
	orch := h.orch
	if orch == nil {
		orch = orchestrator.New("", "", nil, nil)
	}
	snap := orch.Status(c.Request.Context())
	switch id {
	case snap.Crawler.JobID:
		response.Success(c, gin.H{"jenis": "crawler", "job": snap.Crawler, "snapshot": snap})
	case snap.Pipeline.JobID:
		response.Success(c, gin.H{"jenis": "pipeline", "job": snap.Pipeline, "snapshot": snap})
	case snap.Chain.ChainID:
		response.Success(c, gin.H{"jenis": "chain", "job": snap.Chain, "snapshot": snap})
	default:
		response.NotFound(c, "Job tidak ditemukan")
	}
}

// TriggerCrawler handles POST /v1/ml/crawler/run: memicu crawl manual/retry gagal dan opsional merangkai ML training.
func (h *Handler) TriggerCrawler(c *gin.Context) {
	var req orchestrator.TriggerCrawlerRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		response.BadRequest(c, "Payload tidak valid: "+err.Error())
		return
	}
	if h.orch == nil {
		response.Error(c, nethttp.StatusServiceUnavailable, "503", orchestrator.ErrTidakDikonfigurasi.Error())
		return
	}

	snap, code, err := h.orch.TriggerCrawler(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, orchestrator.ErrSedangBerjalan):
			response.ErrorWithData(c, nethttp.StatusConflict, "409", "Crawler atau rantai pipeline sedang berjalan", snap)
		case errors.Is(err, orchestrator.ErrTidakDikonfigurasi), errors.Is(err, orchestrator.ErrTidakTerjangkau):
			response.ErrorWithData(c, nethttp.StatusServiceUnavailable, "503", err.Error(), snap)
		default:
			response.ErrorWithData(c, code, fmt.Sprintf("%d", code), err.Error(), snap)
		}
		return
	}
	response.Accepted(c, snap)
}

// TriggerPipeline handles POST /v1/ml/pipeline/run: memicu training ulang ML worker datasafe secara manual.
func (h *Handler) TriggerPipeline(c *gin.Context) {
	if h.orch == nil {
		response.Error(c, nethttp.StatusServiceUnavailable, "503", orchestrator.ErrTidakDikonfigurasi.Error())
		return
	}
	snap, code, err := h.orch.TriggerPipeline(c.Request.Context())
	if err != nil {
		switch {
		case errors.Is(err, orchestrator.ErrSedangBerjalan):
			response.ErrorWithData(c, nethttp.StatusConflict, "409", "ML pipeline sedang berjalan", snap)
		case errors.Is(err, orchestrator.ErrTidakDikonfigurasi), errors.Is(err, orchestrator.ErrTidakTerjangkau):
			response.ErrorWithData(c, nethttp.StatusServiceUnavailable, "503", err.Error(), snap)
		default:
			response.ErrorWithData(c, code, fmt.Sprintf("%d", code), err.Error(), snap)
		}
		return
	}
	response.Accepted(c, snap)
}

// StreamJobs handles GET /v1/ml/jobs/stream: aliran SSE progres crawler & training ML secara realtime.
func (h *Handler) StreamJobs(c *gin.Context) {
	ctx, cancel := middleware.TokenLifetimeContext(c)
	defer cancel()

	_ = nethttp.NewResponseController(c.Writer).SetWriteDeadline(time.Time{})

	header := c.Writer.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("X-Accel-Buffering", "no")
	c.Status(nethttp.StatusOK)

	orch := h.orch
	if orch == nil {
		orch = orchestrator.New("", "", nil, nil)
	}

	kirim := func(ev string, snap orchestrator.Snapshot) bool {
		b, err := json.Marshal(snap)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", ev, b); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	awal := orch.Status(ctx)
	if !kirim("snapshot", awal) {
		return
	}

	subCh, unsub := orch.Subscribe()
	defer unsub()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case snap, ok := <-subCh:
			if !ok {
				return
			}
			if !kirim("snapshot", snap) {
				return
			}
		case <-ticker.C:
			snap := orch.Status(ctx)
			if !kirim("snapshot", snap) {
				return
			}
		}
	}
}
