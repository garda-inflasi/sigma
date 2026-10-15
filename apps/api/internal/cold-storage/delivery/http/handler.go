package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/cold-storage/dto"
	"github.com/garda-inflasi/api/internal/cold-storage/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GetAll handles GET /v1/cold-storage
func (h *Handler) GetAll(c *gin.Context) {
	fasilitas, stok, err := h.uc.GetAll(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data cold storage")
		return
	}

	fasilitasResp := make([]dto.FasilitasColdStorageResponse, len(fasilitas))
	for i, f := range fasilitas {
		fasilitasResp[i] = dto.FasilitasColdStorageResponse{
			ID:               f.ID,
			Nama:             f.Nama,
			Lokasi:           f.Lokasi,
			Kapasitas:        f.Kapasitas,
			KapasitasTerpakai: f.KapasitasTerpakai,
			Status:           f.Status,
		}
	}

	stokResp := make([]dto.StokColdStorageResponse, len(stok))
	for i, s := range stok {
		stokResp[i] = dto.StokColdStorageResponse{
			ID:            s.ID,
			FasilitasID:   s.FasilitasID,
			KomoditasKode: s.KomoditasKode,
			Jumlah:        s.Jumlah,
			Satuan:        s.Satuan,
			MasukPada:     s.MasukPada,
			KeluarPada:    s.KeluarPada,
		}
	}

	response.Success(c, dto.ColdStorageResponse{
		Fasilitas: fasilitasResp,
		Stok:      stokResp,
	})
}

// Health handles GET /v1/cold-storage/health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
