package http

import (
	"strconv"

	"github.com/garda-inflasi/api/internal/ews/dto"
	"github.com/garda-inflasi/api/internal/ews/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// List handles GET /v1/ews
func (h *Handler) List(c *gin.Context) {
	var req dto.ListEWSRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	limit := req.Limit
	if limit < 1 {
		limit = 200
	}

	alerts, err := h.uc.List(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data EWS")
		return
	}

	items := make([]dto.EwsAlertResponse, len(alerts))
	for i, a := range alerts {
		items[i] = dto.EwsAlertResponse{
			ID:                   a.ID,
			KomoditasKode:        a.KomoditasKode,
			WilayahKode:          a.WilayahKode,
			Level:                string(a.Level),
			Skor:                 a.Skor,
			Deskripsi:            a.Deskripsi,
			Status:               a.Status,
			Sumber:               a.Sumber,
			SkorDampak:           a.SkorDampak,
			DirekomendasikanPada: a.DirekomendasikanPada,
			DitindaklanjutiPada:  a.DitindaklanjutiPada,
			DibuatPada:           a.DibuatPada,
		}
		if a.DiakuiOlehID != nil {
			items[i].DiakuiOlehID = a.DiakuiOlehID
		}
		if a.DiakuiPada != nil {
			items[i].DiakuiPada = a.DiakuiPada
		}
	}

	response.Success(c, items)
}

// GetByID handles GET /v1/ews/:id
func (h *Handler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "ID tidak valid")
		return
	}

	alert, err := h.uc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Alert EWS tidak ditemukan")
		return
	}

	resp := dto.EWSDetailResponse{
		ID:                   alert.ID,
		KomoditasKode:        alert.KomoditasKode,
		WilayahKode:          alert.WilayahKode,
		Level:                string(alert.Level),
		Skor:                 alert.Skor,
		Deskripsi:            alert.Deskripsi,
		Status:               alert.Status,
		Sumber:               alert.Sumber,
		SkorDampak:           alert.SkorDampak,
		DirekomendasikanPada: alert.DirekomendasikanPada,
		DitindaklanjutiPada:  alert.DitindaklanjutiPada,
		DibuatPada:           alert.DibuatPada,
	}
	if alert.DiakuiOlehID != nil {
		resp.DiakuiOlehID = alert.DiakuiOlehID
	}
	if alert.DiakuiPada != nil {
		resp.DiakuiPada = alert.DiakuiPada
	}

	response.Success(c, resp)
}

// Bulk handles POST /v1/ews/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkEWSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	userID := c.GetString("user_id")
	count, err := h.uc.BulkAction(c.Request.Context(), req.IDs, req.Action, userID)
	if err != nil {
		switch err {
		case usecase.ErrInvalidAction:
			response.BadRequest(c, "Aksi tidak valid")
		default:
			response.InternalError(c, "Gagal memproses aksi massal")
		}
		return
	}

	response.Success(c, dto.BulkEWSResponse{
		Berhasil: int(count),
	})
}
