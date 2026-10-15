package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/spasial/dto"
	"github.com/garda-inflasi/api/internal/spasial/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/spasial/wilayah
func (h *Handler) ListWilayah(c *gin.Context) {
	items, err := h.uc.ListWilayah(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data wilayah")
		return
	}

	res := make([]dto.WilayahMarkerResponse, len(items))
	for i, item := range items {
		res[i] = dto.WilayahMarkerResponse{
			Kode:  item.Kode,
			Nama:  item.Nama,
			Lat:   item.Lat,
			Lng:   item.Lng,
			Level: item.Level,
		}
	}

	response.Success(c, res)
}
