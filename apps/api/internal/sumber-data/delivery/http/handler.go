package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	ingest "github.com/garda-inflasi/api/internal/ingestion/domain"
	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"github.com/garda-inflasi/api/internal/sumber-data/dto"
	"github.com/garda-inflasi/api/internal/sumber-data/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/sumber-data
func (h *Handler) List(c *gin.Context) {
	items, err := h.uc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data sumber data")
		return
	}
	response.Success(c, dto.ToSumberDataResponses(items, h.uc.BisaDijalankan))
}

const pesanTidakAda = "Sumber data tidak ditemukan"

// rujukan membaca :id: UUID sumber atau kodenya (alias, mis. SRC-BI). Teks kosong atau lebih panjang dari kode
// dijawab 404, sama dengan sumber yang tidak ada.
func rujukan(c *gin.Context) (idx.RujukanUUID, bool) {
	return idx.ParamUUIDAtauKode(c, "id", domain.MaksKode, pesanTidakAda)
}

// GET /v1/sumber-data/:id
func (h *Handler) Get(c *gin.Context) {
	ref, ok := rujukan(c)
	if !ok {
		return
	}
	item, err := h.uc.Get(c.Request.Context(), ref)
	if errors.Is(err, usecase.ErrNotFound) {
		response.NotFound(c, "Sumber data tidak ditemukan")
		return
	}
	if err != nil {
		response.InternalError(c, "Gagal mengambil sumber data")
		return
	}
	response.Success(c, dto.ToSumberDataResponse(*item, h.uc.BisaDijalankan(item.Kode)))
}

// POST /v1/sumber-data/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkSumberDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	res, err := h.uc.BulkAction(audit.RequestContext(c), req.IDs, req.Action, c.GetString("user_id"))
	if errors.Is(err, usecase.ErrInvalidAction) {
		response.BadRequest(c, "Aksi tidak valid")
		return
	}
	if err != nil {
		response.InternalError(c, "Gagal memproses aksi massal")
		return
	}
	response.Success(c, dto.BulkSumberDataResponse{Berhasil: res.Berhasil, Gagal: res.Gagal})
}

// GET /v1/sumber-data/pipeline
func (h *Handler) Pipeline(c *gin.Context) {
	p, err := h.uc.Pipeline(c.Request.Context(), time.Now())
	if err != nil {
		response.InternalError(c, "Gagal menghitung pipeline data")
		return
	}
	response.Success(c, dto.ToPipelineResponse(p))
}

// Batas daftar riwayat eksekusi dan sampel.
const (
	limitBawaan = 20
	limitMaks   = 100
)

// limit membaca ?limit=; nilai di luar 1..limitMaks ditolak.
func limit(c *gin.Context) (int, bool) {
	raw := c.Query("limit")
	if raw == "" {
		return limitBawaan, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > limitMaks {
		response.BadRequest(c, "limit harus angka 1-"+strconv.Itoa(limitMaks))
		return 0, false
	}
	return n, true
}

// GET /v1/sumber-data/:id/eksekusi
func (h *Handler) Eksekusi(c *gin.Context) {
	ref, ok := rujukan(c)
	if !ok {
		return
	}
	n, ok := limit(c)
	if !ok {
		return
	}
	items, err := h.uc.Eksekusi(c.Request.Context(), ref, n)
	if errors.Is(err, usecase.ErrNotFound) {
		response.NotFound(c, "Sumber data tidak ditemukan")
		return
	}
	if err != nil {
		response.InternalError(c, "Gagal mengambil riwayat eksekusi")
		return
	}
	response.Success(c, dto.ToEksekusiResponses(items))
}

// GET /v1/sumber-data/:id/sampel
func (h *Handler) Sampel(c *gin.Context) {
	ref, ok := rujukan(c)
	if !ok {
		return
	}
	n, ok := limit(c)
	if !ok {
		return
	}
	items, err := h.uc.Sampel(c.Request.Context(), ref, n)
	if errors.Is(err, usecase.ErrNotFound) {
		response.NotFound(c, "Sumber data tidak ditemukan")
		return
	}
	if err != nil {
		response.InternalError(c, "Gagal mengambil sampel data mentah")
		return
	}
	response.Success(c, dto.ToSampelResponses(items))
}

// POST /v1/sumber-data/:id/jalankan: 202 dengan eksekusi BERJALAN; hasilnya dibaca dari riwayat.
func (h *Handler) Jalankan(c *gin.Context) {
	ref, ok := rujukan(c)
	if !ok {
		return
	}
	e, err := h.uc.Jalankan(audit.RequestContext(c), ref, c.GetString("user_id"))
	switch {
	case errors.Is(err, usecase.ErrNotFound), errors.Is(err, ingest.ErrSumberTidakAda):
		response.NotFound(c, "Sumber data tidak ditemukan")
	case errors.Is(err, usecase.ErrTanpaKonektor):
		response.Error(c, http.StatusUnprocessableEntity, "422", "Sumber ini belum memiliki konektor ingestion")
	case errors.Is(err, usecase.ErrDijeda):
		response.Conflict(c, "Sumber sedang dijeda; lanjutkan lebih dulu")
	case errors.Is(err, usecase.ErrSudahBerjalan):
		response.Conflict(c, "Sumber sedang berjalan")
	case err != nil:
		response.InternalError(c, "Gagal memulai ingestion")
	default:
		response.Accepted(c, dto.ToEksekusiResponse(e))
	}
}
