package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/lapangan/dto"
	"github.com/garda-inflasi/api/internal/lapangan/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/lapangan/tugas/saya
func (h *Handler) ListSaya(c *gin.Context) {
	pic := c.GetString("user_id")
	if pic == "" {
		pic = c.Query("pic")
	}

	items, err := h.uc.ListSaya(c.Request.Context(), pic)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data tugas lapangan")
		return
	}

	res := make([]dto.TugasLapanganResponse, len(items))
	for i, item := range items {
		var tenggat *string
		if item.Tenggat != nil {
			t := item.Tenggat.Format("2006-01-02T15:04:05Z07:00")
			tenggat = &t
		}
		res[i] = dto.TugasLapanganResponse{
			ID:         item.ID,
			Judul:      item.Judul,
			Lokasi:     item.Lokasi,
			Tipe:       item.Tipe,
			Status:     item.Status,
			PIC:        item.PIC,
			Tenggat:    tenggat,
			DibuatPada: item.DibuatPada.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	response.Success(c, res)
}
