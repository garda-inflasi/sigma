package http

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/garda-inflasi/api/internal/audit"

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

// csvHeader adalah kolom laporan KPI; urutannya sama dengan csvRecord.
var csvHeader = []string{
	"kelompok", "kode", "nama", "satuan", "arah_baik", "otomatis", "periode_nilai", "nilai",
	"target", "ambang_waspada", "baseline", "status",
}

// GET /v1/kpi/export?periode=YYYY-MM&wilayah=
// Laporan KPI satu periode sebagai CSV (titik koma, BOM UTF-8, seperti ekspor lain).
// Ekspor dicatat di audit trail; bila pencatatan gagal, berkas tidak diserahkan.
func (h *Handler) Export(c *gin.Context) {
	periode, wilayah, ok := h.bindQuery(c)
	if !ok {
		return
	}
	items, err := h.uc.Daftar(c.Request.Context(), periode, wilayah)
	if err != nil {
		h.log.Error("kpi: gagal menyusun laporan", map[string]interface{}{"error": err.Error()})
		response.InternalError(c, "Gagal menyusun laporan KPI")
		return
	}

	var buf bytes.Buffer
	buf.WriteString("\ufeff")
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	_ = w.Write(csvHeader)
	for _, ind := range items {
		_ = w.Write(csvRecord(ind))
	}
	w.Flush()
	if err := w.Error(); err != nil {
		h.log.Error("kpi: gagal menulis csv", map[string]interface{}{"error": err.Error()})
		response.InternalError(c, "Gagal menyusun laporan KPI")
		return
	}

	if err := h.uc.CatatEkspor(audit.RequestContext(c), c.GetString("user_id"), periode, wilayah, len(items)); err != nil {
		h.log.Error("kpi: gagal mencatat ekspor", map[string]interface{}{"error": err.Error()})
		response.InternalError(c, "Gagal mencatat ekspor laporan KPI")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="laporan-kpi-%s.csv"`, usecase.AwalBulan(periode).Format("2006-01")))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

func csvRecord(i domain.Indikator) []string {
	angka := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(*v, 'f', -1, 64)
	}
	var periodeNilai, nilai, target, ambang, baseline string
	if i.Nilai != nil {
		periodeNilai, nilai = i.Nilai.Periode.Format("2006-01"), angka(&i.Nilai.Nilai)
	}
	if i.Target != nil {
		target, ambang, baseline = angka(&i.Target.Target), angka(i.Target.AmbangWaspada), angka(i.Target.Baseline)
	}
	return []string{
		i.Definisi.Kelompok, i.Definisi.Kode, i.Definisi.Nama, i.Definisi.Satuan, i.Definisi.ArahBaik,
		strconv.FormatBool(i.Otomatis), periodeNilai, nilai, target, ambang, baseline, i.Status,
	}
}
