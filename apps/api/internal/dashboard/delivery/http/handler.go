package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/internal/dashboard/dto"
	"github.com/garda-inflasi/api/internal/dashboard/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// cakupan menurunkan cakupan data pemanggil (02-B) dari peran dan kabupaten tugasnya di basis data. Gagal-tutup: bila
// cakupan tidak dapat dipastikan, request dijawab 500, bukan diteruskan dengan cakupan yang lebih luas. Semua endpoint
// /v1/dashboard/operasional/* wajib memanggilnya dan meneruskan hasilnya ke usecase.
func (h *Handler) cakupan(c *gin.Context) (domain.Cakupan, bool) {
	cakupan, err := h.uc.Cakupan(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		response.InternalError(c, "Gagal menentukan cakupan data")
		return domain.Cakupan{}, false
	}
	return cakupan, true
}

// filter membaca query filter KPI dan heatmap. Isiannya baru diperiksa usecase.
func filter(c *gin.Context) (domain.PermintaanFilter, bool) {
	var req dto.FilterRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Parameter tidak valid: "+err.Error())
		return domain.PermintaanFilter{}, false
	}
	return req.Permintaan(), true
}

// saringan membaca query kabupaten_kode dan komoditas_kode ringkasan operasional (stok, produsen, logistik). Isiannya
// baru diperiksa usecase.
func saringan(c *gin.Context) (domain.PermintaanSaringan, bool) {
	var req dto.SaringanRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Parameter tidak valid: "+err.Error())
		return domain.PermintaanSaringan{}, false
	}
	return req.Permintaan(), true
}

// GET /v1/dashboard/operasional/kpi
//
// Query opsional kabupaten_kode, komoditas_kode, dari, sampai. Hanya data dalam cakupan peran pemanggil: kabupaten
// tugas untuk Koordinator dan Agregator, seluruh Aceh untuk peran tingkat provinsi.
func (h *Handler) GetKpiOperasional(c *gin.Context) {
	req, ok := filter(c)
	if !ok {
		return
	}
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	items, err := h.uc.GetKpiOperasional(c.Request.Context(), cakupan, req)
	if err != nil {
		if errors.Is(err, usecase.ErrValidasi) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal mengambil data KPI")
		return
	}

	response.Success(c, dto.ToKpiResponses(items))
}

// GET /v1/dashboard/operasional/heatmap
//
// Query opsional kabupaten_kode, komoditas_kode, dari, sampai (tanggal acuan). Hanya data dalam cakupan peran pemanggil.
func (h *Handler) GetHeatmap(c *gin.Context) {
	req, ok := filter(c)
	if !ok {
		return
	}
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	data, err := h.uc.GetHeatmap(c.Request.Context(), cakupan, req)
	if err != nil {
		if errors.Is(err, usecase.ErrValidasi) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal mengambil data heatmap")
		return
	}

	response.Success(c, dto.ToHeatmapResponse(data))
}

// GET /v1/dashboard/operasional/aktivitas-lapangan
//
// Hanya kabupaten dalam cakupan peran pemanggil.
func (h *Handler) GetAktivitasLapangan(c *gin.Context) {
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	items, err := h.uc.GetAktivitasLapangan(c.Request.Context(), cakupan)
	if err != nil {
		response.InternalError(c, "Gagal mengambil aktivitas lapangan")
		return
	}

	response.Success(c, dto.ToAktivitasLapanganResponses(items))
}

// GET /v1/dashboard/operasional/pembanding-harga
//
// Hanya kabupaten dalam cakupan peran pemanggil.
func (h *Handler) GetPembandingHarga(c *gin.Context) {
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	p, err := h.uc.GetPembandingHarga(c.Request.Context(), cakupan)
	if err != nil {
		response.InternalError(c, "Gagal mengambil pembanding harga")
		return
	}

	response.Success(c, dto.ToPembandingHargaResponse(p))
}

// GET /v1/dashboard/operasional/cuaca
//
// Hanya kabupaten dalam cakupan peran pemanggil.
func (h *Handler) GetCuaca(c *gin.Context) {
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	items, err := h.uc.GetCuaca(c.Request.Context(), cakupan)
	if err != nil {
		response.InternalError(c, "Gagal mengambil cuaca")
		return
	}

	response.Success(c, dto.ToCuacaResponses(items))
}

// GET /v1/dashboard/operasional/aktivitas-petugas
//
// Query opsional kabupaten_kode. Hanya petugas yang kabupaten tugasnya dalam cakupan peran pemanggil. Memerlukan izin
// harga.lihat di samping dashboard.operasional (lihat routes.go): nama dan ID petugas hanya boleh dilihat yang boleh
// melihat entri harga, yang juga memuatnya.
func (h *Handler) GetAktivitasPetugas(c *gin.Context) {
	var req dto.KabupatenRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Parameter tidak valid: "+err.Error())
		return
	}
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	hasil, err := h.uc.GetAktivitasPetugas(c.Request.Context(), cakupan, req.KabupatenKode)
	if err != nil {
		if errors.Is(err, usecase.ErrValidasi) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal mengambil aktivitas petugas")
		return
	}

	response.Success(c, dto.ToAktivitasPetugasResponse(hasil))
}

// GET /v1/dashboard/operasional/stok
//
// Query opsional kabupaten_kode dan komoditas_kode. Hanya fasilitas di kabupaten dalam cakupan peran pemanggil.
func (h *Handler) GetStok(c *gin.Context) {
	req, ok := saringan(c)
	if !ok {
		return
	}
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	stok, err := h.uc.GetStok(c.Request.Context(), cakupan, req)
	if err != nil {
		if errors.Is(err, usecase.ErrValidasi) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal mengambil stok")
		return
	}

	response.Success(c, dto.ToStokResponse(stok))
}

// GET /v1/dashboard/operasional/produsen
//
// Query opsional kabupaten_kode dan komoditas_kode. Hanya kabupaten dalam cakupan peran pemanggil.
func (h *Handler) GetProdusen(c *gin.Context) {
	req, ok := saringan(c)
	if !ok {
		return
	}
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	produsen, err := h.uc.GetProdusen(c.Request.Context(), cakupan, req)
	if err != nil {
		if errors.Is(err, usecase.ErrValidasi) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal mengambil agregasi produsen")
		return
	}

	response.Success(c, dto.ToProdusenResponse(produsen))
}

// GET /v1/dashboard/operasional/logistik
//
// Query opsional kabupaten_kode dan komoditas_kode. Pergerakan dan rute yang salah satu ujungnya di kabupaten dalam
// cakupan peran pemanggil.
func (h *Handler) GetLogistik(c *gin.Context) {
	req, ok := saringan(c)
	if !ok {
		return
	}
	cakupan, ok := h.cakupan(c)
	if !ok {
		return
	}
	logistik, err := h.uc.GetLogistik(c.Request.Context(), cakupan, req)
	if err != nil {
		if errors.Is(err, usecase.ErrValidasi) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal mengambil pergerakan logistik")
		return
	}

	response.Success(c, dto.ToLogistikResponse(logistik))
}
