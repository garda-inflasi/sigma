package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/sumber-data/dto"
	"github.com/garda-inflasi/api/internal/sumber-data/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
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
	response.Success(c, dto.ToSumberDataResponses(items))
}

// GET /v1/sumber-data/:id
func (h *Handler) Get(c *gin.Context) {
	item, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, usecase.ErrNotFound) {
		response.NotFound(c, "Sumber data tidak ditemukan")
		return
	}
	if err != nil {
		response.InternalError(c, "Gagal mengambil sumber data")
		return
	}
	response.Success(c, dto.ToSumberDataResponse(*item))
}

// POST /v1/sumber-data/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkSumberDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	res, err := h.uc.BulkAction(audit.RequestContext(c), req.IDs, req.Action, c.GetString("user_id"))
	if errors.Is(err, usecase.ErrInvalidAction) {
		response.BadRequest(c, "Aksi tidak valid")
		return
	}
	if err != nil {
		response.InternalError(c, "Gagal memproses aksi massal")
		return
	}
	response.Success(c, dto.BulkSumberDataResponse{Berhasil: res.Berhasil, Gagal: res.Gagal})
}
