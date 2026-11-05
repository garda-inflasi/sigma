package http

import (
	"github.com/garda-inflasi/api/internal/dashboard/dto"
	"github.com/garda-inflasi/api/internal/dashboard/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/dashboard/operasional/kpi
func (h *Handler) GetKpiOperasional(c *gin.Context) {
	items, err := h.uc.GetKpiOperasional(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data KPI")
		return
	}

	response.Success(c, dto.ToKpiResponses(items))
}

// GET /v1/dashboard/operasional/heatmap
func (h *Handler) GetHeatmap(c *gin.Context) {
	data, err := h.uc.GetHeatmap(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data heatmap")
		return
	}

	response.Success(c, dto.ToHeatmapResponse(data))
}

// GET /v1/dashboard/operasional/aktivitas-lapangan
func (h *Handler) GetAktivitasLapangan(c *gin.Context) {
	items, err := h.uc.GetAktivitasLapangan(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil aktivitas lapangan")
		return
	}

	response.Success(c, dto.ToAktivitasLapanganResponses(items))
}
