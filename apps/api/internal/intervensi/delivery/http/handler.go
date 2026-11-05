// Package http adalah lapisan HTTP modul intervensi.
package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"github.com/garda-inflasi/api/internal/intervensi/dto"
	"github.com/garda-inflasi/api/internal/intervensi/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// List handles GET /v1/intervensi?siklus=&tahap=&prioritas=
func (h *Handler) List(c *gin.Context) {
	var req dto.ListIntervensiRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	cards, err := h.uc.List(c.Request.Context(), domain.ListQuery{Siklus: req.Siklus, Tahap: req.Tahap, Prioritas: req.Prioritas})
	if err != nil {
		response.InternalError(c, "Gagal mengambil data intervensi")
		return
	}
	response.Success(c, dto.ToKartuResponses(cards))
}

// ListTahap handles GET /v1/intervensi/tahap
func (h *Handler) ListTahap(c *gin.Context) {
	defs, err := h.uc.ListTahap(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil definisi tahap")
		return
	}
	response.Success(c, dto.ToTahapResponses(defs))
}

// Create handles POST /v1/intervensi
func (h *Handler) Create(c *gin.Context) {
	var req dto.CreateIntervensiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	created, err := h.uc.Create(audit.RequestContext(c), domain.NewIntervensi{
		Siklus: req.Siklus, Judul: req.Judul, Deskripsi: req.Deskripsi, KomoditasKode: req.KomoditasKode,
		KabupatenKode: req.KabupatenKode, Prioritas: req.Prioritas, Tenggat: req.Tenggat, EwsAlertID: req.EwsAlertID,
		PenanggungJawabIDs: req.PenanggungJawabIDs, DibuatOlehID: c.GetString("user_id"),
	})
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidReference) {
			response.BadRequest(c, "Komoditas, kabupaten, alert, atau penanggung jawab tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal membuat intervensi")
		return
	}
	response.Created(c, dto.CreateIntervensiResponse{ID: created.ID, Tahap: created.Tahap})
}

// UpdateTahap handles PATCH /v1/intervensi/:id/tahap
func (h *Handler) UpdateTahap(c *gin.Context) {
	var req dto.UpdateTahapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	hasPermission := func(permission string) bool { return middleware.HasPermission(c, permission) }
	versi, err := h.uc.Move(audit.RequestContext(c), c.Param("id"), req.Tahap, req.Urutan, req.Versi, c.GetString("user_id"), hasPermission)

	var forbidden *usecase.ForbiddenError
	switch {
	case err == nil:
		response.Success(c, dto.MoveResponse{Versi: versi})
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(c, "Kartu intervensi tidak ditemukan")
	case errors.Is(err, usecase.ErrInvalidTahap):
		response.BadRequest(c, "Tahap tidak valid untuk siklus kartu ini")
	case errors.Is(err, usecase.ErrVersiUsang):
		response.Conflict(c, "Kartu telah diubah pengguna lain. Papan dimuat ulang; ulangi pemindahan bila masih perlu.")
	case errors.As(err, &forbidden):
		response.Forbidden(c, "Tidak dapat memindahkan kartu: "+forbidden.Error())
	default:
		response.InternalError(c, "Gagal memperbarui tahap")
	}
}

// Bulk handles POST /v1/intervensi/bulk. Gagal pada satu kartu (tidak ada, sudah selesai,
// sudah ditugaskan/dieskalasi) tidak menggagalkan kartu lain; rinciannya ada di `hasil`.
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkIntervensiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	res, err := h.uc.BulkAksi(audit.RequestContext(c), req.Action, req.IDs, c.GetString("user_id"))
	switch {
	case err == nil:
		response.Success(c, dto.ToBulkResponse(res))
	case errors.Is(err, usecase.ErrInvalidAction):
		response.BadRequest(c, "Aksi tidak valid")
	default:
		response.InternalError(c, "Gagal memproses aksi massal")
	}
}

// ListRiwayat handles GET /v1/intervensi/:id/riwayat
func (h *Handler) ListRiwayat(c *gin.Context) {
	items, err := h.uc.ListRiwayat(c.Request.Context(), c.Param("id"))
	switch {
	case err == nil:
		response.Success(c, dto.ToRiwayatResponses(items))
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(c, "Kartu intervensi tidak ditemukan")
	default:
		response.InternalError(c, "Gagal mengambil riwayat kartu")
	}
}
