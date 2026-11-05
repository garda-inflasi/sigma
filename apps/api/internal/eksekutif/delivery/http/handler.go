package http

import (
	"github.com/garda-inflasi/api/internal/eksekutif/dto"
	"github.com/garda-inflasi/api/internal/eksekutif/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// izinFiskal diperlukan untuk melihat blok ruang fiskal.
const izinFiskal = "fiskal.lihat"

type Handler struct {
	uc  *usecase.UseCase
	log *logger.Logger
}

func New(uc *usecase.UseCase, log *logger.Logger) *Handler {
	return &Handler{uc: uc, log: log}
}

// GET /v1/dashboard/eksekutif
func (h *Handler) Ringkasan(c *gin.Context) {
	res, err := h.uc.Ringkasan(c.Request.Context(), middleware.HasPermission(c, izinFiskal))
	if err != nil {
		h.log.Error("eksekutif: gagal menyusun ringkasan", map[string]interface{}{"error": err.Error()})
		response.InternalError(c, "Gagal mengambil ringkasan eksekutif")
		return
	}
	response.Success(c, dto.ToRingkasanResponse(res))
}
