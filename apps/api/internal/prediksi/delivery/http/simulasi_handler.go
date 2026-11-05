package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/prediksi/dto"
	"github.com/garda-inflasi/api/internal/prediksi/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// SimulasiHandler melayani simulasi skenario what-if.
type SimulasiHandler struct {
	uc *usecase.Simulasi
}

func NewSimulasi(uc *usecase.Simulasi) *SimulasiHandler {
	return &SimulasiHandler{uc: uc}
}

// POST /v1/prediksi/simulasi
func (h *SimulasiHandler) Hitung(c *gin.Context) {
	var req dto.SimulasiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	p := usecase.ParameterSimulasi{
		KomoditasKode: req.KomoditasKode, KabupatenKode: req.KabupatenKode, HorizonHari: usecase.DefaultHorizon,
		HujanPct: usecase.CurahHujanNormal,
	}
	if p.KabupatenKode == "" {
		p.KabupatenKode = usecase.DefaultKabupaten
	}
	if req.HorizonHari != nil {
		p.HorizonHari = *req.HorizonHari
	}
	if req.HujanPct != nil {
		p.HujanPct = *req.HujanPct
	}
	if req.PasokanPct != nil {
		p.PasokanPct = *req.PasokanPct
	}
	if req.SubsidiPct != nil {
		p.SubsidiPct = *req.SubsidiPct
	}

	hasil, err := h.uc.Hitung(audit.RequestContext(c), c.GetString("user_id"), p)
	switch {
	case errors.Is(err, usecase.ErrProyeksiTidakAda):
		response.NotFound(c, "Proyeksi model produksi tidak ditemukan untuk skenario ini")
	case err != nil:
		response.InternalError(c, "Gagal menghitung simulasi")
	default:
		response.Created(c, hasil)
	}
}
