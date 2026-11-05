package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/lapangan/dto"
	"github.com/garda-inflasi/api/internal/lapangan/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// pemanggil membaca identitas dari token; tugas.kelola membuka tugas petugas lain.
func pemanggil(c *gin.Context) usecase.Pemanggil {
	return usecase.Pemanggil{ID: c.GetString("user_id"), Semua: middleware.HasPermission(c, "tugas.kelola")}
}

// GET /v1/lapangan/tugas/saya
func (h *Handler) ListSaya(c *gin.Context) {
	items, err := h.uc.ListSaya(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		response.InternalError(c, "Gagal mengambil data tugas lapangan")
		return
	}

	res := make([]dto.TugasLapanganResponse, len(items))
	for i, item := range items {
		res[i] = dto.ToTugasLapanganResponse(item)
	}
	response.Success(c, res)
}

// GET /v1/lapangan/tugas/:id
func (h *Handler) Get(c *gin.Context) {
	item, err := h.uc.GetByID(c.Request.Context(), c.Param("id"), pemanggil(c))
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Tugas tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil data tugas lapangan")
		return
	}
	response.Success(c, dto.ToTugasLapanganResponse(*item))
}

// POST /v1/lapangan/tugas/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkTugasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	hasil, err := h.uc.BulkAction(audit.RequestContext(c), req.IDs, req.Aksi, pemanggil(c))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidAction):
			response.BadRequest(c, "Aksi tidak valid")
		case errors.Is(err, usecase.ErrValidasi):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "Gagal memproses aksi massal")
		}
		return
	}

	response.Success(c, dto.BulkTugasResponse{Berhasil: hasil.Berhasil, Gagal: hasil.Gagal})
}
