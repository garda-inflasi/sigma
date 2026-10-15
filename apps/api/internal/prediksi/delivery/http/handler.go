package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/prediksi/dto"
	"github.com/garda-inflasi/api/internal/prediksi/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/prediksi/:kode
func (h *Handler) GetByKode(c *gin.Context) {
	var req dto.GetProyeksiRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	proyeksi, err := h.uc.GetByKode(c.Request.Context(), req.Kode)
	if err != nil {
		if err == usecase.ErrNotFound {
			response.NotFound(c, "Proyeksi tidak ditemukan")
			return
		}
		response.InternalError(c, "Kesalahan server")
		return
	}

	resp := dto.ProyeksiXaiResponse{
		ID:                 proyeksi.ID,
		KomoditasKode:      proyeksi.KomoditasKode,
		ProyeksiHarga:      proyeksi.ProyeksiHarga,
		IntervalAtas:       proyeksi.IntervalAtas,
		IntervalBawah:      proyeksi.IntervalBawah,
		FaktorDominan:      proyeksi.FaktorDominan,
		TingkatKepercayaan: proyeksi.TingkatKepercayaan,
		Periode:            proyeksi.Periode,
	}

	response.Success(c, resp)
}

// GET /v1/prediksi
func (h *Handler) List(c *gin.Context) {
	var req dto.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}

	items, total, err := h.uc.List(c.Request.Context(), req.Page, req.Limit)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data")
		return
	}

	proyeksiList := make([]dto.ProyeksiXaiResponse, len(items))
	for i, p := range items {
		proyeksiList[i] = dto.ProyeksiXaiResponse{
			ID:                 p.ID,
			KomoditasKode:      p.KomoditasKode,
			ProyeksiHarga:      p.ProyeksiHarga,
			IntervalAtas:       p.IntervalAtas,
			IntervalBawah:      p.IntervalBawah,
			FaktorDominan:      p.FaktorDominan,
			TingkatKepercayaan: p.TingkatKepercayaan,
			Periode:            p.Periode,
		}
	}

	response.Success(c, dto.ListResponse{
		Items: proyeksiList,
		Total: total,
		Page:  req.Page,
		Limit: req.Limit,
	})
}

// GET /v1/prediksi/:kode/export
func (h *Handler) Export(c *gin.Context) {
	kode := c.Param("kode")
	proyeksi, err := h.uc.GetByKode(c.Request.Context(), kode)
	if err != nil {
		if err == usecase.ErrNotFound {
			response.NotFound(c, "Proyeksi tidak ditemukan")
			return
		}
		response.InternalError(c, "Kesalahan server")
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=proyeksi_"+kode+".json")
	c.JSON(http.StatusOK, proyeksi)
}
