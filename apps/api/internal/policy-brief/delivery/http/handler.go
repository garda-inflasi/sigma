package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/policy-brief/dto"
	"github.com/garda-inflasi/api/internal/policy-brief/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
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

	response.Success(c, dto.ToBriefResponses(items))
}

// POST /v1/policy-brief/generate
func (h *Handler) Generate(c *gin.Context) {
	var req dto.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	pb, err := h.uc.Generate(audit.RequestContext(c), c.GetString("user_id"), req.Periode)
	switch {
	case errors.Is(err, usecase.ErrPeriodeTidakValid):
		response.BadRequest(c, "Periode harus berformat YYYY-MM, mis. 2026-10")
	case errors.Is(err, usecase.ErrPeriodeSudahAda):
		response.Conflict(c, "Brief untuk periode ini sudah ada")
	case err != nil:
		response.InternalError(c, "Gagal membuat policy brief")
	default:
		response.Created(c, dto.ToBriefResponse(*pb))
	}
}
