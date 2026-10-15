package http

import (
	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/harga/dto"
	"github.com/garda-inflasi/api/internal/harga/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/harga
func (h *Handler) ListHarga(c *gin.Context) {
	var req dto.ListHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 500
	}

	entries, err := h.uc.ListHarga(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data harga")
		return
	}

	items := make([]dto.EntriHargaResponse, len(entries))
	for i, e := range entries {
		items[i] = dto.EntriHargaResponse{
			ID:            e.ID,
			KomoditasKode: e.KomoditasKode,
			WilayahKode:   e.WilayahKode,
			Harga:         e.Harga,
			Satuan:        e.Satuan,
			Sumber:        e.Sumber,
			Periode:       e.Periode,
			DibuatPada:    e.DibuatPada,
			DiupdatePada:  e.DiupdatePada,
		}
	}

	response.Success(c, items)
}

// GET /v1/harga/tren
func (h *Handler) GetTren(c *gin.Context) {
	var req dto.TrenHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	tren, err := h.uc.GetTren(c.Request.Context(), req.Komoditas, req.Wilayah)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data tren")
		return
	}

	items := make([]dto.TrenHargaResponse, len(tren))
	for i, t := range tren {
		items[i] = dto.TrenHargaResponse{
			Periode:     t.Periode,
			HargaRerata: t.HargaRerata,
			HargaMin:    t.HargaMin,
			HargaMax:    t.HargaMax,
			JumlahEntri: t.JumlahEntri,
		}
	}

	response.Success(c, items)
}
