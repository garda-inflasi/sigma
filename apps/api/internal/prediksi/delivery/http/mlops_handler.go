package http

import (
	"errors"
	"net/http"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/prediksi/dto"
	"github.com/garda-inflasi/api/internal/prediksi/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// MLOpsHandler melayani pelatihan, penyegaran, dan evaluasi model (izin model.kelola).
type MLOpsHandler struct {
	uc *usecase.MLOps
}

func NewMLOps(uc *usecase.MLOps) *MLOpsHandler {
	return &MLOpsHandler{uc: uc}
}

// GET /v1/prediksi/ml
func (h *MLOpsHandler) Status(c *gin.Context) {
	s := h.uc.Status(c.Request.Context())
	response.Success(c, dto.StatusMLResponse{Dikonfigurasi: s.Dikonfigurasi, Tersedia: s.Tersedia, Versi: s.Versi})
}

// POST /v1/prediksi/latih
func (h *MLOpsHandler) Latih(c *gin.Context) {
	var req dto.KomoditasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	model, err := h.uc.Latih(audit.RequestContext(c), c.GetString("user_id"), req.KomoditasKode)
	if h.galat(c, err, "Gagal melatih model") {
		return
	}
	response.Created(c, dto.ToModel(*model))
}

// POST /v1/prediksi/segarkan
func (h *MLOpsHandler) Segarkan(c *gin.Context) {
	var req dto.KomoditasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	n, err := h.uc.Segarkan(audit.RequestContext(c), c.GetString("user_id"), req.KomoditasKode)
	if h.galat(c, err, "Gagal menyegarkan proyeksi") {
		return
	}
	response.Success(c, dto.SegarkanResponse{Proyeksi: n})
}

// POST /v1/prediksi/evaluasi
func (h *MLOpsHandler) Evaluasi(c *gin.Context) {
	n, err := h.uc.Evaluasi(audit.RequestContext(c), c.GetString("user_id"))
	if err != nil {
		response.InternalError(c, "Gagal mengevaluasi proyeksi")
		return
	}
	response.Success(c, dto.EvaluasiResponse{Dievaluasi: n})
}

// galat menjawab err (bila ada) dengan status yang sesuai dan melaporkan apakah sudah dijawab.
func (h *MLOpsHandler) galat(c *gin.Context, err error, pesanUmum string) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, usecase.ErrLayananMLMati):
		response.Error(c, http.StatusServiceUnavailable, "503", "Layanan ML tidak tersedia. Proyeksi demo tetap tampil; coba lagi nanti.")
	case errors.Is(err, usecase.ErrMLGagal):
		response.Error(c, http.StatusBadGateway, "502", "Layanan ML menolak atau gagal memproses permintaan.")
	case errors.Is(err, usecase.ErrDataTidakCukup):
		response.Error(c, http.StatusUnprocessableEntity, "422", "Data harga tervalidasi belum cukup untuk komoditas ini.")
	case errors.Is(err, usecase.ErrModelTidakAda):
		response.NotFound(c, "Komoditas tidak punya model produksi")
	case errors.Is(err, usecase.ErrBukanHasilLatih):
		response.Conflict(c, "Model produksi komoditas ini adalah data demo; latih model terlebih dahulu")
	default:
		response.InternalError(c, pesanUmum)
	}
	return true
}
