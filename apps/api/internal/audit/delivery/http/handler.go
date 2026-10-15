package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/audit/dto"
	"github.com/garda-inflasi/api/internal/audit/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/audit
func (h *Handler) List(c *gin.Context) {
	var req dto.ListAuditRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if req.Limit < 1 {
		req.Limit = 500
	}

	items, err := h.uc.List(c.Request.Context(), req.Limit)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data audit")
		return
	}

	res := make([]dto.AuditRowResponse, len(items))
	for i, item := range items {
		res[i] = dto.AuditRowResponse{
			ID:        item.ID,
			Waktu:     item.Waktu.Format("2006-01-02T15:04:05Z07:00"),
			AktorID:   item.AktorID,
			Aksi:      item.Aksi,
			Entitas:   item.Entitas,
			EntitasID: item.EntitasID,
			Hasil:     item.Hasil,
			IP:        item.IP,
			Metadata:  item.Metadata,
		}
	}

	response.Success(c, res)
}
