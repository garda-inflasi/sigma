package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/sumber-data/dto"
	"github.com/garda-inflasi/api/internal/sumber-data/usecase"
	"github.com/garda-inflasi/api/pkg/response"
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

	res := make([]dto.SumberDataResponse, len(items))
	for i, item := range items {
		res[i] = dto.SumberDataResponse{
			ID:              item.ID,
			Nama:            item.Nama,
			Tipe:            item.Tipe,
			URL:             item.URL,
			Status:          item.Status,
			FrekuensiUpdate: item.FrekuensiUpdate,
			DibuatPada:      item.DibuatPada.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	response.Success(c, res)
}
