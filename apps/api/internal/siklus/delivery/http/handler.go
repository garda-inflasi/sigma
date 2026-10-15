package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/siklus/dto"
	"github.com/garda-inflasi/api/internal/siklus/usecase"
	"github.com/garda-inflasi/api/pkg/response"
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
		response.BadRequest(c, "Validasi gagal: parameter wilayah_kode diperlukan")
		return
	}

	siklus, err := h.uc.GetAktif(c.Request.Context(), req.WilayahKode)
	if err != nil {
		switch err {
		case usecase.ErrSiklusNotFound:
			response.NotFound(c, "Siklus aktif tidak ditemukan untuk wilayah ini")
		default:
			response.InternalError(c, "Kesalahan server")
		}
		return
	}

	resp := dto.ToSiklusAktifResponse(siklus.Siklus, siklus.MulaiPada)
	response.Success(c, resp)
}
