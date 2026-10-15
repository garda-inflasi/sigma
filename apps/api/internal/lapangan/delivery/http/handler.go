package http

import (
	"strconv"

	"github.com/garda-inflasi/api/internal/lapangan/dto"
	"github.com/garda-inflasi/api/internal/lapangan/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
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

// GET /v1/lapangan/tugas/:id
func (h *Handler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "ID tidak valid")
		return
	}

	item, err := h.uc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Tugas tidak ditemukan")
		return
	}

	var tenggat *string
	if item.Tenggat != nil {
		t := item.Tenggat.Format("2006-01-02T15:04:05Z07:00")
		tenggat = &t
	}

	response.Success(c, dto.TugasLapanganResponse{
		ID:         item.ID,
		Judul:      item.Judul,
		Lokasi:     item.Lokasi,
		Tipe:       item.Tipe,
		Status:     item.Status,
		PIC:        item.PIC,
		Tenggat:    tenggat,
		DibuatPada: item.DibuatPada.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// POST /v1/lapangan/tugas/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkTugasRequest
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

	response.Success(c, dto.BulkTugasResponse{Berhasil: int(count)})
}
