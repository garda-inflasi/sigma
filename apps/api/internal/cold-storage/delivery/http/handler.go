package http

import (
	"errors"
	"net/http"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"github.com/garda-inflasi/api/internal/cold-storage/dto"
	"github.com/garda-inflasi/api/internal/cold-storage/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// bulkPermission adalah izin yang dibutuhkan tiap aksi massal. Rute hanya
// memastikan pemanggil memegang salah satunya; kecocokan dengan aksi diperiksa
// di handler, selaras dengan matriks izin antarmuka.
var bulkPermission = map[domain.BulkAction]string{
	domain.BulkRencanakanDistribusi: "logistik.kelola",
	domain.BulkRekomendasiOlahan:    "coldstorage.kelola",
	domain.BulkTandaiPrioritasJual:  "coldstorage.kelola",
	domain.BulkBuang:                "coldstorage.kelola",
}

type Handler struct {
	uc  *usecase.UseCase
	ops *usecase.Operasi
}

func New(uc *usecase.UseCase, ops *usecase.Operasi) *Handler {
	return &Handler{uc: uc, ops: ops}
}

// GetAll handles GET /v1/cold-storage
func (h *Handler) GetAll(c *gin.Context) {
	var req dto.ColdStorageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	overview, err := h.uc.GetAll(c.Request.Context(), domain.StokQuery{
		ColdStorageID: req.ColdStorageID,
		Search:        req.Q,
		Sort:          domain.StokSort(req.Sort),
		Desc:          req.Order == "desc",
		Page:          req.Page,
		Limit:         req.Limit,
	})
	if err != nil {
		response.InternalError(c, "Gagal mengambil data cold storage")
		return
	}

	fasilitasResp := make([]dto.FasilitasColdStorageResponse, len(overview.Fasilitas))
	for i, f := range overview.Fasilitas {
		fasilitasResp[i] = dto.ToFasilitasResponse(f)
	}
	stokResp := make([]dto.StokColdStorageResponse, len(overview.Stok))
	for i, st := range overview.Stok {
		stokResp[i] = dto.ToStokResponse(st)
	}

	response.Success(c, dto.ColdStorageResponse{
		Fasilitas: fasilitasResp,
		Stok:      stokResp,
		Meta: dto.PageMetaResponse{
			Page:           overview.Page,
			Limit:          overview.Limit,
			Total:          overview.Total,
			TotalPages:     overview.TotalPages,
			VolumeHasilTon: overview.TotalVolumeTon,
		},
		Ringkasan: dto.RingkasanColdStorageResponse{
			TotalLot:           overview.Ringkasan.TotalLot,
			TotalVolumeTon:     overview.Ringkasan.TotalVolumeTon,
			RisikoTinggiLot:    overview.Ringkasan.RisikoTinggiLot,
			RisikoTinggiTon:    overview.Ringkasan.RisikoTinggiTon,
			AmbangRisikoTinggi: domain.RisikoTinggiAmbang,
		},
	})
}

// Bulk handles POST /v1/cold-storage/bulk
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkColdStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if perm := bulkPermission[domain.BulkAction(req.Action)]; perm != "" && !middleware.HasPermission(c, perm) {
		response.Forbidden(c, "Akses ditolak")
		return
	}

	in := usecase.BulkInput{Action: req.Action, IDs: req.IDs, ActorID: c.GetString("user_id")}
	var (
		out usecase.BulkOutcome
		err error
	)
	if usecase.IsMutatingBulk(req.Action) {
		out, err = h.ops.BulkMutasi(audit.RequestContext(c), in)
	} else {
		out, err = h.uc.BulkAction(audit.RequestContext(c), in)
	}
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidAction):
			response.BadRequest(c, "Aksi tidak valid")
		case errors.Is(err, usecase.ErrNoIDs):
			response.BadRequest(c, "Pilih minimal satu lot stok")
		case errors.Is(err, usecase.ErrTooManyIDs):
			response.BadRequest(c, "Terlalu banyak lot dipilih dalam satu aksi")
		default:
			response.InternalError(c, "Gagal memproses aksi massal")
		}
		return
	}

	response.Success(c, dto.ToBulkResponse(out.Berhasil, out.Gagal, out.VolumeTon, out.Baris))
}

// Health handles GET /v1/cold-storage/health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
