package http

import (
	"errors"
	"time"

	"github.com/garda-inflasi/api/internal/kpi/domain"
	"github.com/garda-inflasi/api/internal/kpi/dto"
	"github.com/garda-inflasi/api/internal/kpi/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc  *usecase.UseCase
	log *logger.Logger
	now func() time.Time
}

func New(uc *usecase.UseCase, log *logger.Logger) *Handler {
	return &Handler{uc: uc, log: log, now: time.Now}
}

// bindQuery membaca periode dan wilayah; ok false berarti respons 400 sudah dikirim.
func (h *Handler) bindQuery(c *gin.Context) (periode time.Time, wilayah *string, ok bool) {
	var req dto.ListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return time.Time{}, nil, false
	}
	periode, valid := req.ParsePeriode()
	if !valid {
		response.BadRequest(c, "Periode harus berformat YYYY-MM")
		return time.Time{}, nil, false
	}
	if periode.IsZero() {
		periode = h.now()
	}
	return periode, req.WilayahPtr(), true
}

// GET /v1/kpi?periode=YYYY-MM&wilayah=
func (h *Handler) List(c *gin.Context) {
	periode, wilayah, ok := h.bindQuery(c)
	if !ok {
		return
	}
	items, err := h.uc.Daftar(c.Request.Context(), periode, wilayah)
	if err != nil {
		h.log.Error("kpi: gagal mengambil daftar", map[string]interface{}{"error": err.Error()})
		response.InternalError(c, "Gagal mengambil indikator kinerja")
		return
	}
	response.Success(c, dto.ToIndikatorList(items))
}

// GET /v1/kpi/:kode/riwayat?periode=YYYY-MM&wilayah=
func (h *Handler) Riwayat(c *gin.Context) {
	periode, wilayah, ok := h.bindQuery(c)
	if !ok {
		return
	}
	ind, riwayat, err := h.uc.Riwayat(c.Request.Context(), c.Param("kode"), periode, wilayah)
	if errors.Is(err, domain.ErrNotFound) {
		response.NotFound(c, "KPI tidak ditemukan")
		return
	}
	if err != nil {
		h.log.Error("kpi: gagal mengambil riwayat", map[string]interface{}{"error": err.Error()})
		response.InternalError(c, "Gagal mengambil riwayat KPI")
		return
	}
	response.Success(c, dto.RiwayatResponse{Kpi: dto.ToIndikator(*ind), Riwayat: dto.ToNilaiList(riwayat)})
}
