package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/policy-brief/dto"
	"github.com/garda-inflasi/api/internal/policy-brief/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/policy-brief
func (h *Handler) List(c *gin.Context) {
	items, err := h.uc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data policy brief")
		return
	}

	res := make([]dto.PolicyBriefResponse, len(items))
	for i, item := range items {
		res[i] = dto.PolicyBriefResponse{
			ID:           item.ID,
			Judul:        item.Judul,
			Periode:      item.Periode,
			Ringkasan:    item.Ringkasan,
			Status:       item.Status,
			DibuatPada:   item.DibuatPada.Format("2006-01-02T15:04:05Z07:00"),
			DiupdatePada: item.DiupdatePada.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	response.Success(c, res)
}

// POST /v1/policy-brief/generate
func (h *Handler) Generate(c *gin.Context) {
	var req dto.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	pb, err := h.uc.Generate(c.Request.Context(), req.Periode)
	if err != nil {
		response.InternalError(c, "Gagal membuat policy brief")
		return
	}

	response.Created(c, dto.GenerateResponse{ID: pb.ID})
}
