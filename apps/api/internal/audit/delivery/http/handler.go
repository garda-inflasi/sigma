package http

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/internal/audit/dto"
	"github.com/garda-inflasi/api/internal/audit/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/audit
func (h *Handler) List(c *gin.Context) {
	var req dto.ListAuditRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	filter, err := toFilter(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	page, err := h.uc.List(c.Request.Context(), usecase.Query{Filter: filter, Limit: req.Limit, Cursor: req.Cursor})
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidCursor) {
			response.BadRequest(c, "Cursor tidak valid")
			return
		}
		response.InternalError(c, "Gagal mengambil data audit")
		return
	}

	response.Success(c, dto.ToAuditPageResponse(page))
}

func toFilter(req dto.ListAuditRequest) (domain.Filter, error) {
	filter := domain.Filter{
		Aktor:   strings.TrimSpace(req.Aktor),
		Aksi:    strings.TrimSpace(req.Aksi),
		Entitas: strings.TrimSpace(req.Entitas),
		Hasil:   req.Hasil,
	}
	var err error
	if filter.Dari, err = parseBoundary(req.Dari, false); err != nil {
		return domain.Filter{}, fmt.Errorf("parameter dari tidak valid: %w", err)
	}
	if filter.Sampai, err = parseBoundary(req.Sampai, true); err != nil {
		return domain.Filter{}, fmt.Errorf("parameter sampai tidak valid: %w", err)
	}
	return filter, nil
}

// parseBoundary membaca RFC 3339 atau tanggal. Tanggal untuk batas atas
// (endOfDay) dimajukan sehari agar seluruh hari itu ikut tercakup (batas eksklusif).
func parseBoundary(raw string, endOfDay bool) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, nil
	}
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return nil, errors.New("gunakan RFC 3339 atau YYYY-MM-DD")
	}
	if endOfDay {
		t = t.AddDate(0, 0, 1)
	}
	return &t, nil
}
