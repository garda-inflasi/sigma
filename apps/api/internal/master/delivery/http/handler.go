package http

import (
	"github.com/garda-inflasi/api/internal/master/dto"
	"github.com/garda-inflasi/api/internal/master/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/master/komoditas
func (h *Handler) ListKomoditas(c *gin.Context) {
	items, err := h.uc.ListKomoditas(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data komoditas")
		return
	}

	res := make([]dto.MasterKomoditasRowResponse, len(items))
	for i, item := range items {
		res[i] = dto.MasterKomoditasRowResponse{
			Kode:     item.Kode,
			Nama:     item.Nama,
			Kategori: item.Kategori,
			Satuan:   item.Satuan,
			Pilot:    item.Pilot,
			Status:   item.Status,
		}
	}

	response.Success(c, res)
}

// GET /v1/master/wilayah
func (h *Handler) ListWilayah(c *gin.Context) {
	provinsiKode := c.Query("provinsi_kode")
	items, err := h.uc.ListWilayah(c.Request.Context(), provinsiKode)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data wilayah")
		return
	}

	res := make([]dto.MasterWilayahRowResponse, len(items))
	for i, item := range items {
		res[i] = dto.MasterWilayahRowResponse{
			Kode:         item.Kode,
			Nama:         item.Nama,
			Jenis:        item.Jenis,
			Ibukota:      item.Ibukota,
			Lat:          item.Lat,
			Lng:          item.Lng,
			ProvinsiKode: item.ProvinsiKode,
		}
	}

	response.Success(c, res)
}

// GET /v1/master/pasar
func (h *Handler) ListPasar(c *gin.Context) {
	kabupatenKode := c.Query("kabupaten_kode")
	items, err := h.uc.ListPasar(c.Request.Context(), kabupatenKode)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data pasar")
		return
	}

	res := make([]dto.MasterPasarRowResponse, len(items))
	for i, item := range items {
		res[i] = dto.MasterPasarRowResponse{
			ID:            item.ID,
			Nama:          item.Nama,
			KabupatenKode: item.KabupatenKode,
			Lat:           item.Lat,
			Lng:           item.Lng,
			RadiusMeter:   item.RadiusMeter,
			Aktif:         item.Aktif,
		}
	}

	response.Success(c, res)
}

// POST /v1/master/komoditas/bulk
func (h *Handler) BulkKomoditas(c *gin.Context) {
	var req dto.BulkKomoditasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	count, err := h.uc.BulkKomoditas(c.Request.Context(), req.Kode, req.Action)
	if err != nil {
		switch err {
		case usecase.ErrInvalidAction:
			response.BadRequest(c, "Aksi tidak valid")
		default:
			response.InternalError(c, "Gagal memproses aksi massal")
		}
		return
	}

	response.Success(c, dto.BulkKomoditasResponse{Berhasil: int(count)})
}
