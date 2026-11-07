package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/internal/master/dto"
	"github.com/garda-inflasi/api/internal/master/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// Kode amplop galat alur usulan, dibaca web untuk pesan yang tepat.
const (
	KodeTanpaPerubahan    = "TANPA_PERUBAHAN"
	KodeUsulanMenunggu    = "USULAN_MENUNGGU"
	KodeKodeSudahAda      = "KODE_SUDAH_ADA"
	KodeStatusTidakSesuai = "STATUS_TIDAK_SESUAI"
	KodeBukanMenunggu     = "BUKAN_MENUNGGU"
	KodeUsulanKedaluwarsa = "USULAN_KEDALUWARSA"
	KodePemisahanTugas    = "PEMISAHAN_TUGAS"
	KodeBukanPengaju      = "BUKAN_PENGAJU"
)

// HandlerPerubahan melayani antrean usulan perubahan master data (/v1/master/perubahan).
type HandlerPerubahan struct {
	uc *usecase.UseCasePerubahan
}

func NewPerubahan(uc *usecase.UseCasePerubahan) *HandlerPerubahan {
	return &HandlerPerubahan{uc: uc}
}

// POST /v1/master/perubahan (master.ajukan)
func (h *HandlerPerubahan) Ajukan(c *gin.Context) {
	var req dto.AjukanPerubahanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	v, err := h.uc.Ajukan(audit.RequestContext(c), usecase.AjukanInput{
		Entitas: req.Entitas, Jenis: req.Jenis, EntitasID: req.EntitasID, Perubahan: req.Perubahan, Alasan: req.Alasan,
	}, c.GetString("user_id"))
	if err != nil {
		h.galat(c, err, "Gagal menyimpan usulan")
		return
	}
	h.kirim(c, http.StatusCreated, v)
}

// GET /v1/master/perubahan?status=&entitas=&entitas_id=&page=&limit=
func (h *HandlerPerubahan) List(c *gin.Context) {
	var req dto.ListPerubahanRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	items, total, err := h.uc.List(c.Request.Context(), domain.FilterPerubahan{
		Status: req.Status, Entitas: req.Entitas, EntitasID: req.EntitasID, Page: req.Page, Limit: req.Limit,
	})
	if err != nil {
		h.galat(c, err, "Gagal mengambil usulan")
		return
	}
	rows, err := dto.ToPerubahanResponses(items)
	if err != nil {
		response.InternalError(c, "Gagal menyusun usulan")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, rows)
}

// GET /v1/master/perubahan/:id
func (h *HandlerPerubahan) Get(c *gin.Context) {
	id, ok := idx.ParamID(c, "id", "Usulan tidak ditemukan")
	if !ok {
		return
	}
	v, err := h.uc.Get(c.Request.Context(), id)
	if err != nil {
		h.galat(c, err, "Gagal mengambil usulan")
		return
	}
	h.kirim(c, http.StatusOK, v)
}

// POST /v1/master/perubahan/:id/setujui (master.setujui)
func (h *HandlerPerubahan) Setujui(c *gin.Context) { h.putuskan(c, domain.KeputusanSetujui) }

// POST /v1/master/perubahan/:id/tolak (master.setujui), body {alasan}
func (h *HandlerPerubahan) Tolak(c *gin.Context) { h.putuskan(c, domain.KeputusanTolak) }

// POST /v1/master/perubahan/:id/batalkan (master.ajukan, hanya pengaju), body {alasan?}
func (h *HandlerPerubahan) Batalkan(c *gin.Context) { h.putuskan(c, domain.KeputusanBatalkan) }

func (h *HandlerPerubahan) putuskan(c *gin.Context, aksi string) {
	id, ok := idx.ParamID(c, "id", "Usulan tidak ditemukan")
	if !ok {
		return
	}
	var req dto.KeputusanRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Validasi gagal: "+err.Error())
			return
		}
	}
	v, err := h.uc.Putuskan(audit.RequestContext(c), id, aksi, c.GetString("user_id"), req.Alasan)
	if err != nil {
		h.galat(c, err, "Gagal memutuskan usulan")
		return
	}
	h.kirim(c, http.StatusOK, v)
}

func (h *HandlerPerubahan) kirim(c *gin.Context, status int, v domain.PerubahanView) {
	r, err := dto.ToPerubahanResponse(v)
	if err != nil {
		response.InternalError(c, "Gagal menyusun usulan")
		return
	}
	if status == http.StatusCreated {
		response.Created(c, r)
		return
	}
	response.Success(c, r)
}

// galat memetakan galat domain ke status HTTP dan kode amplop.
func (h *HandlerPerubahan) galat(c *gin.Context, err error, pesanUmum string) {
	switch {
	case errors.Is(err, domain.ErrTanpaPerubahan):
		response.Error(c, http.StatusBadRequest, KodeTanpaPerubahan, "Usulan tidak mengubah apa pun: semua nilai sama dengan data sekarang")
	case errors.Is(err, domain.ErrValidasi):
		response.BadRequest(c, err.Error())
	case errors.Is(err, domain.ErrEntitasTidakAda):
		response.NotFound(c, "Data master tidak ditemukan")
	case errors.Is(err, domain.ErrUsulanTidakAda):
		response.NotFound(c, "Usulan tidak ditemukan")
	case errors.Is(err, domain.ErrUsulanMenunggu):
		response.Error(c, http.StatusConflict, KodeUsulanMenunggu, "Data ini sudah punya usulan yang menunggu persetujuan; putuskan atau batalkan usulan itu dulu")
	case errors.Is(err, domain.ErrKodeSudahAda):
		response.Error(c, http.StatusConflict, KodeKodeSudahAda, "Kode sudah dipakai data lain")
	case errors.Is(err, domain.ErrStatusTidakSesuai):
		response.Error(c, http.StatusConflict, KodeStatusTidakSesuai, "Status data tidak mengizinkan perubahan (mis. komoditas sudah diarsipkan)")
	case errors.Is(err, domain.ErrBukanMenunggu):
		response.Error(c, http.StatusConflict, KodeBukanMenunggu, "Usulan sudah diputuskan; muat ulang daftar")
	case errors.Is(err, domain.ErrKedaluwarsa):
		response.Error(c, http.StatusConflict, KodeUsulanKedaluwarsa, "Data berubah sejak usulan diajukan; tolak usulan ini lalu ajukan ulang")
	case errors.Is(err, domain.ErrPemisahanTugas):
		response.Error(c, http.StatusForbidden, KodePemisahanTugas, "Pengaju tidak boleh menyetujui atau menolak usulannya sendiri")
	case errors.Is(err, domain.ErrBukanPengaju):
		response.Error(c, http.StatusForbidden, KodeBukanPengaju, "Hanya pengaju yang boleh membatalkan usulan")
	default:
		response.InternalError(c, pesanUmum)
	}
}
