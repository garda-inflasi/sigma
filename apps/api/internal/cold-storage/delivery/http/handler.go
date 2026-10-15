package http

import (
	"net/http"

	"github.com/garda-inflasi/api/internal/cold-storage/dto"
	"github.com/garda-inflasi/api/internal/cold-storage/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
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
			ID:            f.ID,
			Nama:          f.Nama,
			KabupatenKode: f.KabupatenKode,
			Lat:           f.Lat,
			Lng:           f.Lng,
			KapasitasTon:  f.KapasitasTon,
			SuhuTargetC:   f.SuhuTargetC,
			Status:        f.Status,
		}
	}

	stokResp := make([]dto.StokColdStorageResponse, len(stok))
	for i, s := range stok {
		stokResp[i] = dto.StokColdStorageResponse{
			ID:             s.ID,
			ColdStorageID:  s.ColdStorageID,
			KomoditasKode:  s.KomoditasKode,
			VolumeTon:      s.VolumeTon,
			MasukPada:      s.MasukPada,
			Pemilik:        s.Pemilik,
			RisikoFoodLoss: s.RisikoFoodLoss,
		}
	}

	response.Success(c, dto.ColdStorageResponse{
		Fasilitas: fasilitasResp,
		Stok:      stokResp,
	})
}

// Bulk handles POST /v1/cold-storage/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkColdStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	count, err := h.uc.BulkAction(c.Request.Context(), req.IDs, req.Action)
	if err != nil {
		switch err {
		case usecase.ErrInvalidAction:
			response.BadRequest(c, "Aksi tidak valid")
		default:
			response.InternalError(c, "Gagal memproses aksi massal")
		}
		return
	}

	response.Success(c, dto.BulkColdStorageResponse{Berhasil: int(count)})
}

// Health handles GET /v1/cold-storage/health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
