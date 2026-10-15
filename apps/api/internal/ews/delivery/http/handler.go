package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/ews/dto"
	"github.com/garda-inflasi/api/internal/ews/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// List handles GET /v1/ews
func (h *Handler) List(c *gin.Context) {
	var req dto.ListEWSRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	limit := req.Limit
	if limit < 1 {
		limit = 200
	}

	alerts, err := h.uc.List(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data EWS")
		return
	}

	items := make([]dto.EwsAlertResponse, len(alerts))
	for i, a := range alerts {
		items[i] = dto.EwsAlertResponse{
			ID:                   a.ID,
			KomoditasKode:        a.KomoditasKode,
			WilayahKode:          a.WilayahKode,
			Level:                string(a.Level),
			Skor:                 a.Skor,
			Deskripsi:            a.Deskripsi,
			DirekomendasikanPada: a.DirekomendasikanPada,
			DitindaklanjutiPada:  a.DitindaklanjutiPada,
			DibuatPada:           a.DibuatPada,
		}
	}

	response.Success(c, items)
}
