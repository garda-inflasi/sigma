// Package http adalah lapisan HTTP modul spasial.
package http

import (
	"github.com/garda-inflasi/api/internal/spasial/domain"
	"github.com/garda-inflasi/api/internal/spasial/dto"
	"github.com/garda-inflasi/api/internal/spasial/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// ListWilayah handles GET /v1/spasial/wilayah
func (h *Handler) ListWilayah(c *gin.Context) {
	items, err := h.uc.ListMarker(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data wilayah")
		return
	}
	response.Success(c, dto.ToMarkerResponses(items))
}

// ListNeraca handles GET /v1/spasial/neraca?komoditas=&periode=YYYY-MM-DD
func (h *Handler) ListNeraca(c *gin.Context) {
	var req dto.NeracaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	items, err := h.uc.ListNeraca(c.Request.Context(), domain.NeracaQuery{KomoditasKode: req.Komoditas, Periode: req.Periode})
	if err != nil {
		response.InternalError(c, "Gagal mengambil neraca wilayah")
		return
	}
	response.Success(c, dto.ToNeracaResponses(items))
}

// ListRute handles GET /v1/spasial/rute
func (h *Handler) ListRute(c *gin.Context) {
	items, err := h.uc.ListRute(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil rute distribusi")
		return
	}
	response.Success(c, dto.ToRuteResponses(items))
}

// ListKandidat handles GET /v1/spasial/kandidat-cold-storage
func (h *Handler) ListKandidat(c *gin.Context) {
	items, err := h.uc.ListKandidat(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil kandidat cold storage")
		return
	}
	out, err := dto.ToKandidatResponses(items)
	if err != nil {
		response.InternalError(c, "Data kandidat cold storage tidak valid")
		return
	}
	response.Success(c, out)
}
