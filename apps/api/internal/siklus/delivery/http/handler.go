package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/siklus/dto"
	"github.com/garda-inflasi/api/internal/siklus/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GetAktif handles GET /v1/siklus/aktif
func (h *Handler) GetAktif(c *gin.Context) {
	var req dto.SiklusAktifRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	siklus, err := h.uc.GetAktif(c.Request.Context(), req.WilayahKode)
	if err != nil {
		if errors.Is(err, usecase.ErrSiklusNotFound) {
			response.NotFound(c, "Siklus aktif tidak ditemukan untuk wilayah ini")
			return
		}
		response.InternalError(c, "Kesalahan server")
		return
	}

	response.Success(c, dto.FromSiklus(siklus))
}

// ListAturan handles GET /v1/siklus/aturan: aturan penentuan siklus otomatis, aktif maupun tidak.
func (h *Handler) ListAturan(c *gin.Context) {
	items, err := h.uc.ListAturan(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Kesalahan server")
		return
	}
	response.Success(c, dto.ToAturanResponses(items))
}

// Evaluasi handles POST /v1/siklus/evaluasi: menjalankan mesin penentuan siklus sekarang.
func (h *Handler) Evaluasi(c *gin.Context) {
	hasil, err := h.uc.Evaluasi(audit.RequestContext(c), c.GetString("user_id"))
	if err != nil {
		if errors.Is(err, usecase.ErrEvaluasiSibuk) {
			response.Conflict(c, "Evaluasi siklus lain sedang berjalan. Coba lagi sebentar.")
			return
		}
		response.InternalError(c, "Gagal menjalankan evaluasi siklus")
		return
	}
	response.Success(c, dto.ToEvaluasiResponse(hasil))
}
