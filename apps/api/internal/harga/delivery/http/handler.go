package http

import (
	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/harga/dto"
	"github.com/garda-inflasi/api/internal/harga/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/harga
func (h *Handler) ListHarga(c *gin.Context) {
	var req dto.ListHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 500
	}

	entries, err := h.uc.ListHarga(c.Request.Context(), limit, req.Status)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data harga")
		return
	}

	items := make([]dto.EntriHargaResponse, len(entries))
	for i, e := range entries {
		items[i] = dto.EntriHargaResponse{
			ID:            e.ID,
			KomoditasKode: e.KomoditasKode,
			PasarID:       e.PasarID,
			Harga:         e.Harga,
			Satuan:        e.Satuan,
			Sumber:        e.Sumber,
			Periode:       e.Periode,
			Status:        e.Status,
			SkorAnomali:   e.SkorAnomali,
			DibuatPada:    e.DibuatPada,
			DiupdatePada:  e.DiupdatePada,
		}
		if e.PetugasID != nil {
			items[i].PetugasID = e.PetugasID
		}
	}

	response.Success(c, items)
}

// GET /v1/harga/tren
func (h *Handler) GetTren(c *gin.Context) {
	var req dto.TrenHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	tren, err := h.uc.GetTren(c.Request.Context(), req.Komoditas, req.Wilayah)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data tren")
		return
	}

	items := make([]dto.TrenHargaResponse, len(tren))
	for i, t := range tren {
		items[i] = dto.TrenHargaResponse{
			Periode:     t.Periode,
			HargaRerata: t.HargaRerata,
			HargaMin:    t.HargaMin,
			HargaMax:    t.HargaMax,
			JumlahEntri: t.JumlahEntri,
		}
	}

	response.Success(c, items)
}

// POST /v1/harga
func (h *Handler) CreateHarga(c *gin.Context) {
	var req dto.CreateHargaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	petugasID := c.GetString("user_id")

	entri := &domain.EntriHarga{
		KomoditasKode: req.KomoditasKode,
		PasarID:       req.PasarID,
		Harga:         req.Harga,
		Satuan:        req.Satuan,
		Periode:       req.Periode,
		Sumber:        req.Sumber,
		PetugasID:     &petugasID,
	}

	if err := h.uc.CreateHarga(c.Request.Context(), entri); err != nil {
		response.InternalError(c, "Gagal menyimpan data harga")
		return
	}

	response.Created(c, dto.CreateHargaResponse{ID: entri.ID})
}

// GET /v1/harga/:id
func (h *Handler) GetHargaByID(c *gin.Context) {
	id := c.Param("id")
	entri, err := h.uc.GetHargaByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "Entri harga tidak ditemukan")
		return
	}

	resp := dto.HargaDetailResponse{
		ID:            entri.ID,
		KomoditasKode: entri.KomoditasKode,
		PasarID:       entri.PasarID,
		Harga:         entri.Harga,
		Satuan:        entri.Satuan,
		Kualitas:      entri.Kualitas,
		Sumber:        entri.Sumber,
		Status:        entri.Status,
		Periode:       entri.Periode,
		DeviasiPct:    entri.DeviasiPct,
		SkorAnomali:   entri.SkorAnomali,
		DalamGeofence: entri.DalamGeofence,
		DibuatPada:    entri.DibuatPada,
		DiupdatePada:  entri.DiupdatePada,
	}
	if entri.Lat != nil {
		resp.Lat = entri.Lat
	}
	if entri.Lng != nil {
		resp.Lng = entri.Lng
	}
	if entri.FotoURL != nil {
		resp.FotoURL = entri.FotoURL
	}
	if entri.Catatan != nil {
		resp.Catatan = entri.Catatan
	}
	if entri.PetugasID != nil {
		resp.PetugasID = entri.PetugasID
	}
	if entri.DivalidasiOleh != nil {
		resp.DivalidasiOleh = entri.DivalidasiOleh
	}
	if entri.DivalidasiPada != nil {
		resp.DivalidasiPada = entri.DivalidasiPada
	}
	if entri.DipublikasiOleh != nil {
		resp.DipublikasiOleh = entri.DipublikasiOleh
	}
	if entri.DipublikasiPada != nil {
		resp.DipublikasiPada = entri.DipublikasiPada
	}

	response.Success(c, resp)
}

// POST /v1/harga/bulk
func (h *Handler) BulkHarga(c *gin.Context) {
	var req dto.BulkHargaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	userID := c.GetString("user_id")
	count, err := h.uc.BulkAction(c.Request.Context(), req.IDs, req.Action, userID)
	if err != nil {
		switch err {
		case usecase.ErrInvalidAction:
			response.BadRequest(c, "Aksi tidak valid")
		default:
			response.InternalError(c, "Gagal memproses aksi massal")
		}
		return
	}

	response.Success(c, dto.BulkHargaResponse{
		Berhasil: int(count),
	})
}
