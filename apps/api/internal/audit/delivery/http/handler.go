package http

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/internal/audit/dto"
	"github.com/garda-inflasi/api/internal/audit/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc         *usecase.UseCase
	integritas *usecase.Integritas
}

func New(uc *usecase.UseCase, integritas *usecase.Integritas) *Handler {
	return &Handler{uc: uc, integritas: integritas}
}

// GET /v1/audit/integritas: verifikasi rantai hash terakhir, partisi bulanan aktif, dan partisi yang sudah
// diarsipkan (03-C). Verifikasinya sendiri dijalankan worker (job audit_verifikasi), bukan permintaan ini.
func (h *Handler) Integritas(c *gin.Context) {
	ringkasan, err := h.integritas.Ringkasan(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil status integritas audit")
		return
	}
	var masalah []domain.Masalah
	if ringkasan.VerifikasiTerakhir != nil {
		masalah = usecase.MasalahVerifikasi(*ringkasan.VerifikasiTerakhir)
	}
	response.Success(c, dto.ToIntegritasResponse(ringkasan.VerifikasiTerakhir, masalah, ringkasan.Partisi, ringkasan.Arsip))
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

// maksKodeAudit adalah panjang kode ID lama (kolom kode varchar(64)).
const maksKodeAudit = 64

// GET /v1/audit/:id
// :id adalah ID BIGINT, atau kode (ID lama baris yang dibuat sebelum migrasi 000115) sebagai alias.
// ID yang salah bentuk dijawab 404, sama dengan ID yang tidak ada.
func (h *Handler) Get(c *gin.Context) {
	ref, ok := idx.ParamIDAtauKode(c, "id", maksKodeAudit, "Entri audit tidak ditemukan")
	if !ok {
		return
	}
	detail, err := h.uc.Get(c.Request.Context(), ref)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			response.NotFound(c, "Entri audit tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil entri audit")
		return
	}
	response.Success(c, dto.ToAuditDetailResponse(detail.Row, detail.Perubahan))
}

// csvHeader adalah kolom berkas ekspor; urutannya sama dengan csvRecord.
var csvHeader = []string{
	"waktu", "aktor_nama", "aktor_email", "aktor_peran", "aksi", "entitas", "entitas_id", "hasil",
	"ip", "user_agent", "correlation_id", "sebelum", "sesudah", "metadata",
}

// GET /v1/audit/export: CSV (titik koma, UTF-8 BOM, seperti ekspor sisi klien)
// atas filter yang sama dengan daftar. Ekspor sendiri dicatat sebagai EKSPOR_DATA.
func (h *Handler) Export(c *gin.Context) {
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

	var buf bytes.Buffer
	buf.WriteString("\ufeff")
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	if err := w.Write(csvHeader); err != nil {
		response.InternalError(c, "Gagal menyusun ekspor audit")
		return
	}
	n, truncated, err := h.uc.Export(c.Request.Context(), filter, func(row domain.AuditLog) error {
		return w.Write(csvRecord(row))
	})
	w.Flush()
	if err == nil {
		err = w.Error()
	}
	if err != nil {
		response.InternalError(c, "Gagal mengekspor data audit")
		return
	}

	// Ekspor adalah keluarnya data sensitif: bila tidak tercatat, tidak diserahkan.
	err = h.uc.Write(audit.RequestContext(c), domain.Entry{
		AktorID: c.GetString("user_id"), Aksi: "EKSPOR_DATA", Entitas: "audit_log", EntitasID: "export",
		Hasil:    domain.HasilBerhasil,
		Metadata: map[string]any{"baris": n, "terpotong": truncated, "query": c.Request.URL.RawQuery},
	})
	if err != nil {
		response.InternalError(c, "Gagal mencatat ekspor audit")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="audit-trail-%s.csv"`, time.Now().Format(time.DateOnly)))
	c.Header("X-Export-Rows", fmt.Sprint(n))
	if truncated {
		c.Header("X-Export-Truncated", "true")
	}
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

func csvRecord(row domain.AuditLog) []string {
	return []string{
		row.Waktu.UTC().Format(time.RFC3339), safeCell(row.AktorNama), safeCell(row.AktorEmail), safeCell(row.AktorPeran),
		safeCell(&row.Aksi), safeCell(&row.Entitas), safeCell(&row.EntitasID), row.Hasil,
		safeCell(row.IP), safeCell(row.UserAgent), safeCell(row.CorrelationID),
		safeCell(row.Sebelum), safeCell(row.Sesudah), safeCell(row.Metadata),
	}
}

// safeCell mencegah injeksi rumus spreadsheet: sel yang diawali = + - @ (atau
// tab/CR) diberi awalan apostrof, karena nilai seperti entitas_id berasal dari input pengguna.
func safeCell(s *string) string {
	if s == nil || *s == "" {
		return ""
	}
	switch (*s)[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + *s
	}
	return *s
}

func toFilter(req dto.ListAuditRequest) (domain.Filter, error) {
	filter := domain.Filter{
		Aktor:   strings.TrimSpace(req.Aktor),
		Aksi:    strings.TrimSpace(req.Aksi),
		Entitas: strings.TrimSpace(req.Entitas),
		Hasil:   req.Hasil,

		EntitasID:     strings.TrimSpace(req.EntitasID),
		Peran:         strings.TrimSpace(req.Peran),
		CorrelationID: strings.TrimSpace(req.CorrelationID),
		Q:             strings.TrimSpace(req.Q),
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
