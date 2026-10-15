package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/master/dto"
	"github.com/garda-inflasi/api/internal/master/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/master/komoditas
func (h *Handler) ListKomoditas(c *gin.Context) {
	items, err := h.uc.ListKomoditas(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data komoditas")
		return
	}

	res := make([]dto.MasterKomoditasRowResponse, len(items))
	for i, item := range items {
		res[i] = dto.MasterKomoditasRowResponse{
			Kode:     item.Kode,
			Nama:     item.Nama,
			Kategori: item.Kategori,
			Satuan:   item.Satuan,
			Pilot:    item.Pilot,
			Status:   item.Status,
		}
	}

	response.Success(c, res)
}
