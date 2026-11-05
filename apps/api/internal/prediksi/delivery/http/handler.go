package http

import (
	"errors"
	"net/http"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/internal/prediksi/dto"
	"github.com/garda-inflasi/api/internal/prediksi/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// bindQuery membaca kabupaten dan horizon, memakai bawaan bila kosong. Mengembalikan
// false setelah menjawab 400 bila ada yang tidak valid.
func bindQuery(c *gin.Context) (kabupaten string, horizon int, ok bool) {
	var q dto.ProyeksiQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return "", 0, false
	}
	kabupaten, horizon = q.Kabupaten, usecase.DefaultHorizon
	if kabupaten == "" {
		kabupaten = usecase.DefaultKabupaten
	}
	if q.Horizon != nil {
		horizon = *q.Horizon
	}
	return kabupaten, horizon, true
}

// GET /v1/prediksi?kabupaten=1171&horizon=14
func (h *Handler) List(c *gin.Context) {
	kabupaten, horizon, ok := bindQuery(c)
	if !ok {
		return
	}

	items, err := h.uc.Ringkasan(c.Request.Context(), kabupaten, horizon)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data proyeksi")
		return
	}
	response.Success(c, dto.ToRingkasan(items))
}

// GET /v1/prediksi/registri
func (h *Handler) Registri(c *gin.Context) {
	items, err := h.uc.Model(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil registri model")
		return
	}
	response.Success(c, dto.ToModels(items))
}

// GET /v1/prediksi/:kode?kabupaten=1171&horizon=14
func (h *Handler) GetByKode(c *gin.Context) {
	detail, ok := h.detail(c)
	if !ok {
		return
	}
	response.Success(c, dto.ToProyeksi(*detail))
}

// GET /v1/prediksi/:kode/export
func (h *Handler) Export(c *gin.Context) {
	detail, ok := h.detail(c)
	if !ok {
		return
	}

	c.Header("Content-Disposition", "attachment; filename=proyeksi_"+detail.Proyeksi.KomoditasKode+".json")
	c.JSON(http.StatusOK, dto.ToProyeksi(*detail))
}

// detail memuat proyeksi lengkap untuk :kode; false bila sudah dijawab dengan galat.
func (h *Handler) detail(c *gin.Context) (*domain.Detail, bool) {
	var req dto.GetProyeksiRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return nil, false
	}
	kabupaten, horizon, ok := bindQuery(c)
	if !ok {
		return nil, false
	}

	detail, err := h.uc.Detail(c.Request.Context(), req.Kode, kabupaten, horizon)
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(c, "Proyeksi tidak ditemukan")
		return nil, false
	case err != nil:
		response.InternalError(c, "Gagal mengambil proyeksi")
		return nil, false
	}
	return detail, true
}
