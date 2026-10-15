package http

import (
	"strconv"

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

// GET /v1/sumber-data/:id
func (h *Handler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "ID tidak valid")
		return
	}

	item, err := h.uc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Sumber data tidak ditemukan")
		return
	}

	response.Success(c, dto.SumberDataResponse{
		ID:              item.ID,
		Nama:            item.Nama,
		Tipe:            item.Tipe,
		URL:             item.URL,
		Status:          item.Status,
		FrekuensiUpdate: item.FrekuensiUpdate,
		DibuatPada:      item.DibuatPada.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// POST /v1/sumber-data/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkSumberDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	count, err := h.uc.BulkAction(c.Request.Context(), req.IDs, req.Action)
	if err != nil {
		switch err {
		case usecase.ErrInvalidAction:
			response.BadRequest(c, "Aksi tidak valid")
		default:
			response.InternalError(c, "Gagal memproses aksi massal")
		}
		return
	}

	response.Success(c, dto.BulkSumberDataResponse{Berhasil: int(count)})
}
