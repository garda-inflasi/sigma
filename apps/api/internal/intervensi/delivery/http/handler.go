package http

import (
	"strconv"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/internal/intervensi/dto"
	"github.com/garda-inflasi/api/internal/intervensi/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/intervensi
func (h *Handler) List(c *gin.Context) {
	kartu, err := h.uc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data intervensi")
		return
	}

	resp := dto.ToKartuIntervensiListResponse(kartu)
	response.Success(c, resp)
}

// POST /v1/intervensi
func (h *Handler) Create(c *gin.Context) {
	var req dto.CreateIntervensiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	userID := c.GetString("user_id")

	kartu := &domain.KartuIntervensi{
		Judul:         req.Judul,
		Deskripsi:     req.Deskripsi,
		Prioritas:     req.Prioritas,
		PIC:           req.PIC,
		KomoditasKode: req.KomoditasKode,
		WilayahKode:   req.WilayahKode,
		DibuatOlehID:  userID,
	}

	if err := h.uc.Create(c.Request.Context(), kartu); err != nil {
		response.InternalError(c, "Gagal membuat intervensi")
		return
	}

	response.Created(c, dto.CreateIntervensiResponse{ID: kartu.ID})
}

// PATCH /v1/intervensi/:id/tahap
func (h *Handler) UpdateTahap(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "ID tidak valid")
		return
	}

	var req dto.UpdateTahapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if err := h.uc.UpdateTahap(c.Request.Context(), id, req.Tahap, req.Urutan); err != nil {
		switch err {
		case usecase.ErrNotFound:
			response.NotFound(c, "Kartu intervensi tidak ditemukan")
		case usecase.ErrInvalidTahap:
			response.BadRequest(c, "Tahap tidak valid")
		default:
			response.InternalError(c, "Gagal memperbarui tahap")
		}
		return
	}

	response.Success(c, nil)
}

// POST /v1/intervensi/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkIntervensiRequest
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

	response.Success(c, dto.BulkIntervensiResponse{
		Berhasil: int(count),
	})
}
