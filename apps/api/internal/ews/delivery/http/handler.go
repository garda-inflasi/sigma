// Package http adalah lapisan HTTP modul EWS.
package http

import (
	"errors"
	"strconv"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/ews/dto"
	"github.com/garda-inflasi/api/internal/ews/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// totalCountHeader membawa jumlah seluruh hasil filter untuk klien yang memaginasi.
const totalCountHeader = "X-Total-Count"

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// List handles GET /v1/ews?status=&severity=&jenis=&kabupaten_kode=&page=&limit=
func (h *Handler) List(c *gin.Context) {
	var req dto.ListEWSRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	items, total, err := h.uc.List(c.Request.Context(), domain.ListQuery{
		Status: req.Status, Severity: req.Severity, Jenis: req.Jenis, KabupatenKode: req.KabupatenKode,
		Page: req.Page, Limit: req.Limit,
	})
	if err != nil {
		response.InternalError(c, "Gagal mengambil data EWS")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToAlertResponses(items))
}

// GetByID handles GET /v1/ews/:id
func (h *Handler) GetByID(c *gin.Context) {
	alert, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Alert EWS tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil alert EWS")
		return
	}
	response.Success(c, dto.ToAlertResponse(*alert))
}

// ListAturan handles GET /v1/ews/aturan
func (h *Handler) ListAturan(c *gin.Context) {
	items, err := h.uc.ListAturan(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil aturan ambang EWS")
		return
	}
	response.Success(c, dto.ToAturanResponses(items))
}

// Bulk handles POST /v1/ews/bulk. Route meminta ews.akui atau ews.eskalasi;
// izin yang tepat menurut aksi diperiksa di sini.
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkEWSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	if !middleware.HasPermission(c, izinAksi(req.Action)) {
		response.Forbidden(c, "Tidak memiliki izin untuk aksi ini")
		return
	}

	res, err := h.uc.BulkAction(audit.RequestContext(c), req.IDs, req.Action, c.GetString("user_id"))
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidAction) {
			response.BadRequest(c, "Aksi tidak valid")
			return
		}
		response.InternalError(c, "Gagal memproses aksi massal")
		return
	}
	response.Success(c, dto.BulkEWSResponse{Berhasil: res.Berhasil, Gagal: res.Gagal})
}

// izinAksi adalah izin yang dibutuhkan untuk satu aksi massal.
func izinAksi(action string) string {
	if action == string(domain.BulkEskalasi) {
		return "ews.eskalasi"
	}
	return "ews.akui"
}
