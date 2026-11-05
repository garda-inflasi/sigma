package http

import (
	"errors"
	"strconv"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/internal/master/dto"
	"github.com/garda-inflasi/api/internal/master/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// totalCountHeader membawa jumlah seluruh hasil pencarian untuk klien yang memaginasi.
const totalCountHeader = "X-Total-Count"

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/master/komoditas?q=&page=&limit=
func (h *Handler) ListKomoditas(c *gin.Context) {
	query, ok := bindListQuery(c)
	if !ok {
		return
	}
	items, total, err := h.uc.ListKomoditas(c.Request.Context(), query)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data komoditas")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToKomoditasRows(items))
}

// GET /v1/master/wilayah?provinsi_kode=&q=&page=&limit=
func (h *Handler) ListWilayah(c *gin.Context) {
	query, ok := bindListQuery(c)
	if !ok {
		return
	}
	items, total, err := h.uc.ListWilayah(c.Request.Context(), c.Query("provinsi_kode"), query)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data wilayah")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToWilayahRows(items))
}

// GET /v1/master/pasar?kabupaten_kode=&q=&page=&limit=
func (h *Handler) ListPasar(c *gin.Context) {
	query, ok := bindListQuery(c)
	if !ok {
		return
	}
	items, total, err := h.uc.ListPasar(c.Request.Context(), c.Query("kabupaten_kode"), query)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data pasar")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToPasarRows(items))
}

// bindListQuery membaca q/page/limit; ok false berarti respons 400 sudah dikirim.
func bindListQuery(c *gin.Context) (domain.ListQuery, bool) {
	var req dto.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return domain.ListQuery{}, false
	}
	return domain.ListQuery{Search: req.Q, Page: req.Page, Limit: req.Limit}, true
}

// POST /v1/master/komoditas/bulk
func (h *Handler) BulkKomoditas(c *gin.Context) {
	var req dto.BulkKomoditasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	count, err := h.uc.BulkKomoditas(audit.RequestContext(c), req.Kode, req.Action, c.GetString("user_id"))
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidAction) {
			response.BadRequest(c, "Aksi tidak valid")
			return
		}
		response.InternalError(c, "Gagal memproses aksi massal")
		return
	}
	response.Success(c, dto.BulkKomoditasResponse{Berhasil: int(count)})
}
