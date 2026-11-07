package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/internal/policy-brief/dto"
	"github.com/garda-inflasi/api/internal/policy-brief/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
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

// GET /v1/policy-brief
func (h *Handler) List(c *gin.Context) {
	items, err := h.uc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data policy brief")
		return
	}

	response.Success(c, dto.ToBriefResponses(items))
}

// GET /v1/policy-brief/:id
//
// :id adalah UUID brief atau kodenya (mis. PB-2026-10, alias tautan lama). Teks kosong atau lebih panjang dari
// 64 karakter dijawab 404 tanpa menyentuh basis data; kode yang tidak ada juga 404.
func (h *Handler) Get(c *gin.Context) {
	ref, ok := idx.ParamUUIDAtauKode(c, "id", usecase.MaksKode, "Policy brief tidak ditemukan")
	if !ok {
		return
	}
	detail, err := h.uc.Detail(c.Request.Context(), ref)
	switch {
	case errors.Is(err, usecase.ErrBriefTidakAda):
		response.NotFound(c, "Policy brief tidak ditemukan")
	case err != nil:
		response.InternalError(c, "Gagal mengambil policy brief")
	default:
		response.Success(c, dto.ToBriefDetail(*detail))
	}
}

// POST /v1/policy-brief/generate
//
// Penyusunan berjalan di latar belakang: jawabannya brief berstatus DIPROSES yang dipantau lewat GET /:id.
func (h *Handler) Generate(c *gin.Context) {
	var req dto.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	pb, err := h.uc.Generate(audit.RequestContext(c), c.GetString("user_id"), req.Periode)
	switch {
	case errors.Is(err, usecase.ErrPeriodeTidakValid):
		response.BadRequest(c, "Periode harus berformat YYYY-MM, mis. 2026-10")
	case errors.Is(err, usecase.ErrPeriodeSudahAda):
		response.Conflict(c, "Brief untuk periode ini sudah ada (satu periode hanya punya satu brief)")
	case err != nil:
		response.InternalError(c, "Gagal membuat policy brief")
	default:
		response.Created(c, dto.ToBriefResponse(*pb))
	}
}

// Bulk menangani POST /v1/policy-brief/bulk. Mengajukan telaah butuh brief.susun,
// menyetujui butuh brief.setujui. Setiap elemen ids adalah UUID atau kode brief.
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	if !middleware.HasPermission(c, izinAksi(req.Action)) {
		response.Forbidden(c, "Tidak memiliki izin untuk aksi ini")
		return
	}

	hasil, err := h.uc.BulkAction(audit.RequestContext(c), req.IDs, req.Action, c.GetString("user_id"))
	if err != nil {
		if errors.Is(err, usecase.ErrAksiTidakValid) {
			response.BadRequest(c, "Aksi tidak valid")
			return
		}
		response.InternalError(c, "Gagal memproses aksi massal")
		return
	}
	response.Success(c, dto.BulkResponse{Berhasil: hasil.Berhasil, Gagal: hasil.Gagal})
}

// izinAksi adalah izin yang dibutuhkan untuk satu aksi massal.
func izinAksi(aksi string) string {
	if aksi == domain.AksiSetujui {
		return "brief.setujui"
	}
	return "brief.susun"
}
