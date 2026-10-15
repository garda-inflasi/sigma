package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/dashboard/dto"
	"github.com/garda-inflasi/api/internal/dashboard/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/dashboard/operasional/kpi
func (h *Handler) GetKpiOperasional(c *gin.Context) {
	var req dto.KpiOperasionalRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	items, err := h.uc.GetKpiOperasional(c.Request.Context(), req.Periode)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data KPI")
		return
	}

	response.Success(c, dto.KpiOperasionalResponse{Items: items})
}

// GET /v1/dashboard/operasional/heatmap
func (h *Handler) GetHeatmap(c *gin.Context) {
	var req dto.HeatmapRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	data, err := h.uc.GetHeatmap(c.Request.Context(), req.Periode)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data heatmap")
		return
	}

	response.Success(c, dto.HeatmapResponse{
		Kabupaten: data.Kabupaten,
		Komoditas: data.Komoditas,
		Sel:       data.Sel,
	})
}

// GET /v1/dashboard/eksekutif
func (h *Handler) GetRingkasanEksekutif(c *gin.Context) {
	var req dto.RingkasanEksekutifRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	data, err := h.uc.GetRingkasanEksekutif(c.Request.Context(), req.Periode)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data ringkasan eksekutif")
		return
	}

	response.Success(c, dto.RingkasanEksekutifResponse{Data: data})
}

// GET /v1/dashboard/operasional/kpi — fallback for empty path
func (h *Handler) NotFound(c *gin.Context) {
	response.Error(c, http.StatusNotFound, "404", "Endpoint tidak ditemukan")
}

