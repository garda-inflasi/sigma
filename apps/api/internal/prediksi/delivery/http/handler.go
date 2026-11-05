package http

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"github.com/garda-inflasi/api/internal/prediksi/dto"
	"github.com/garda-inflasi/api/internal/prediksi/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// bindQuery membaca kabupaten dan horizon, memakai bawaan bila kosong. Mengembalikan
// false setelah menjawab 400 bila ada yang tidak valid.
func bindQuery(c *gin.Context) (kabupaten string, horizon int, ok bool) {
	var q dto.ProyeksiQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return "", 0, false
	}
	kabupaten, horizon = q.Kabupaten, usecase.DefaultHorizon
	if kabupaten == "" {
		kabupaten = usecase.DefaultKabupaten
	}
	if q.Horizon != nil {
		horizon = *q.Horizon
	}
	return kabupaten, horizon, true
}

// GET /v1/prediksi?kabupaten=1171&horizon=14
func (h *Handler) List(c *gin.Context) {
	kabupaten, horizon, ok := bindQuery(c)
	if !ok {
		return
	}

	items, err := h.uc.Ringkasan(c.Request.Context(), kabupaten, horizon)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data proyeksi")
		return
	}
	response.Success(c, dto.ToRingkasan(items))
}

// GET /v1/prediksi/registri
func (h *Handler) Registri(c *gin.Context) {
	items, err := h.uc.Model(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil registri model")
		return
	}
	response.Success(c, dto.ToModels(items))
}

// GET /v1/prediksi/:kode?kabupaten=1171&horizon=14
func (h *Handler) GetByKode(c *gin.Context) {
	detail, ok := h.detail(c)
	if !ok {
		return
	}
	response.Success(c, dto.ToProyeksi(*detail))
}

// csvHeader adalah kolom ekspor proyeksi; urutannya sama dengan baris yang ditulis Export.
var csvHeader = []string{
	"komoditas_kode", "komoditas_nama", "kabupaten_kode", "horizon_hari", "tanggal",
	"aktual", "proyeksi", "bawah", "atas", "model_versi", "sumber",
}

// GET /v1/prediksi/:kode/export?kabupaten=&horizon=
// Seri grafik satu komoditas (riwayat aktual, titik sambung, jalur proyeksi) sebagai CSV
// (titik koma, BOM UTF-8, seperti ekspor lain). Sel kosong berarti titik itu tidak punya nilai tersebut.
func (h *Handler) Export(c *gin.Context) {
	detail, ok := h.detail(c)
	if !ok {
		return
	}
	p := dto.ToProyeksi(*detail)

	angka := func(v *int) string {
		if v == nil {
			return ""
		}
		return strconv.Itoa(*v)
	}
	var buf bytes.Buffer
	buf.WriteString("\ufeff")
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	_ = w.Write(csvHeader)
	for _, t := range p.Seri {
		_ = w.Write([]string{
			p.KomoditasKode, p.KomoditasNama, p.KabupatenKode, strconv.Itoa(p.HorizonHari), t.Tanggal,
			angka(t.Aktual), angka(t.Proyeksi), angka(t.Bawah), angka(t.Atas), p.Model.Versi, p.Sumber,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		response.InternalError(c, "Gagal menyusun berkas proyeksi")
		return
	}

	c.Header("Content-Disposition", `attachment; filename="proyeksi_`+p.KomoditasKode+`.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

// detail memuat proyeksi lengkap untuk :kode; false bila sudah dijawab dengan galat.
func (h *Handler) detail(c *gin.Context) (*domain.Detail, bool) {
	var req dto.GetProyeksiRequest
	if err := c.ShouldBindUri(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return nil, false
	}
	kabupaten, horizon, ok := bindQuery(c)
	if !ok {
		return nil, false
	}

	detail, err := h.uc.Detail(c.Request.Context(), req.Kode, kabupaten, horizon)
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(c, "Proyeksi tidak ditemukan")
		return nil, false
	case err != nil:
		response.InternalError(c, "Gagal mengambil proyeksi")
		return nil, false
	}
	return detail, true
}
