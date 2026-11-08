package http

import (
	"errors"
	stdhttp "net/http"
	"strconv"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/master/domain"
	"github.com/garda-inflasi/api/internal/master/dto"
	"github.com/garda-inflasi/api/internal/master/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// Kode galat 409 pada penghapusan master.
const (
	kodeHapusAdaUsulan = "MASTER_ADA_USULAN"
	kodeHapusDirujuk   = "MASTER_DIRUJUK"
)

// HandlerReferensi melayani data referensi master: kecamatan, satuan, pedagang, detail, riwayat versi, dan hapus.
type HandlerReferensi struct {
	uc *usecase.UseCaseReferensi
	// usulan (opsional) melengkapi detail komoditas/pasar dengan usulan yang menunggu.
	usulan *usecase.UseCasePerubahan
}

// DenganUsulan melengkapi detail komoditas dan pasar dengan usulan perubahan yang menunggu.
func (h *HandlerReferensi) DenganUsulan(uc *usecase.UseCasePerubahan) *HandlerReferensi {
	h.usulan = uc
	return h
}

// usulanMenunggu mengembalikan usulan MENUNGGU satu entitas (paling banyak satu, dijaga indeks unik), atau nil.
func (h *HandlerReferensi) usulanMenunggu(c *gin.Context, entitas, kode string) (*dto.PerubahanResponse, error) {
	if h.usulan == nil {
		return nil, nil
	}
	items, _, err := h.usulan.List(c.Request.Context(), domain.FilterPerubahan{
		Status: domain.UsulanMenunggu, Entitas: entitas, EntitasID: kode, Limit: 1,
	})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	r, err := dto.ToPerubahanResponse(items[0])
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func NewReferensi(uc *usecase.UseCaseReferensi) *HandlerReferensi {
	return &HandlerReferensi{uc: uc}
}

// GET /v1/master/kecamatan?kabupaten_kode=&q=&page=&limit=
func (h *HandlerReferensi) ListKecamatan(c *gin.Context) {
	var req dto.ListKecamatanRequest
	if !bindQueryReferensi(c, &req) {
		return
	}
	items, total, err := h.uc.ListKecamatan(c.Request.Context(), req.KabupatenKode, queryDaftarReferensi(req.ListRequest))
	if err != nil {
		response.InternalError(c, "Gagal mengambil data kecamatan")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToKecamatanRows(items))
}

// GET /v1/master/kecamatan/:kode
func (h *HandlerReferensi) GetKecamatan(c *gin.Context) {
	item, err := h.uc.GetKecamatan(c.Request.Context(), c.Param("kode"))
	if err != nil {
		h.galatBaca(c, err, "Kecamatan tidak ditemukan", "Gagal mengambil kecamatan")
		return
	}
	response.Success(c, dto.ToKecamatanRow(item))
}

// GET /v1/master/satuan?q=&page=&limit=
func (h *HandlerReferensi) ListSatuan(c *gin.Context) {
	var req dto.ListRequest
	if !bindQueryReferensi(c, &req) {
		return
	}
	items, total, err := h.uc.ListSatuan(c.Request.Context(), queryDaftarReferensi(req))
	if err != nil {
		response.InternalError(c, "Gagal mengambil data satuan")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToSatuanRows(items))
}

// GET /v1/master/pedagang?pasar_id=&q=&aktif=true|false&page=&limit=
func (h *HandlerReferensi) ListPedagang(c *gin.Context) {
	var req dto.ListPedagangRequest
	if !bindQueryReferensi(c, &req) {
		return
	}
	filter := domain.PedagangFilter{PasarID: req.PasarID}
	if req.Aktif != "" {
		aktif := req.Aktif == "true"
		filter.Aktif = &aktif
	}
	items, total, err := h.uc.ListPedagang(c.Request.Context(), filter, queryDaftarReferensi(req.ListRequest))
	if err != nil {
		response.InternalError(c, "Gagal mengambil data pedagang")
		return
	}
	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToPedagangRows(items))
}

// GET /v1/master/komoditas/:kode
func (h *HandlerReferensi) GetKomoditas(c *gin.Context) {
	item, err := h.uc.GetKomoditas(c.Request.Context(), c.Param("kode"))
	if err != nil {
		h.galatBaca(c, err, "Komoditas tidak ditemukan", "Gagal mengambil komoditas")
		return
	}
	res := dto.ToKomoditasDetail(item)
	if res.PerubahanMenunggu, err = h.usulanMenunggu(c, domain.EntitasKomoditas, item.Komoditas.Kode); err != nil {
		response.InternalError(c, "Gagal mengambil usulan komoditas")
		return
	}
	response.Success(c, res)
}

// GET /v1/master/komoditas/:kode/versi?pada=
//
// Tanpa pada: seluruh riwayat (terbaru dulu). Dengan pada: satu versi yang berlaku pada waktu itu.
func (h *HandlerReferensi) VersiKomoditas(c *gin.Context) {
	var req dto.VersiKomoditasRequest
	if !bindQueryReferensi(c, &req) {
		return
	}
	kode := c.Param("kode")
	if req.Pada == "" {
		riwayat, err := h.uc.RiwayatVersi(c.Request.Context(), kode)
		if err != nil {
			h.galatBaca(c, err, "Komoditas tidak ditemukan", "Gagal mengambil riwayat versi komoditas")
			return
		}
		response.Success(c, dto.ToKomoditasVersiRows(riwayat))
		return
	}

	pada, err := usecase.ParsePada(req.Pada)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	versi, err := h.uc.VersiPada(c.Request.Context(), kode, pada)
	if err != nil {
		h.galatBaca(c, err, "Komoditas tidak ditemukan", "Gagal mengambil versi komoditas")
		return
	}
	response.Success(c, dto.ToKomoditasVersiRow(versi))
}

// GET /v1/master/wilayah/:kode
func (h *HandlerReferensi) GetWilayah(c *gin.Context) {
	item, err := h.uc.GetWilayah(c.Request.Context(), c.Param("kode"))
	if err != nil {
		h.galatBaca(c, err, "Wilayah tidak ditemukan", "Gagal mengambil wilayah")
		return
	}
	response.Success(c, dto.ToWilayahDetail(item))
}

// GET /v1/master/pasar/:kode (nilai pasar.id, mis. PSR-1171-01)
func (h *HandlerReferensi) GetPasar(c *gin.Context) {
	item, err := h.uc.GetPasar(c.Request.Context(), c.Param("kode"))
	if err != nil {
		h.galatBaca(c, err, "Pasar tidak ditemukan", "Gagal mengambil pasar")
		return
	}
	res := dto.ToPasarDetail(item)
	if res.PerubahanMenunggu, err = h.usulanMenunggu(c, domain.EntitasPasar, res.Kode); err != nil {
		response.InternalError(c, "Gagal mengambil usulan pasar")
		return
	}
	response.Success(c, res)
}

// DELETE /v1/master/komoditas/:kode (rute mensyaratkan master.setujui)
func (h *HandlerReferensi) HapusKomoditas(c *gin.Context) {
	err := h.uc.HapusKomoditas(audit.RequestContext(c), c.Param("kode"), c.GetString("user_id"))
	h.jawabHapus(c, err, "Komoditas tidak ditemukan", pesanHapusKomoditas)
}

// DELETE /v1/master/pasar/:kode (rute mensyaratkan master.setujui)
func (h *HandlerReferensi) HapusPasar(c *gin.Context) {
	err := h.uc.HapusPasar(audit.RequestContext(c), c.Param("kode"), c.GetString("user_id"))
	h.jawabHapus(c, err, "Pasar tidak ditemukan", pesanHapusPasar)
}

// Pesan 409 per entitas: ADA_USULAN meminta usulannya diselesaikan, DIRUJUK menyarankan jalan yang benar.
var (
	pesanHapusKomoditas = pesanHapus{
		adaUsulan: "Komoditas masih punya usulan perubahan yang menunggu; setujui atau tolak usulannya dulu",
		dirujuk:   "Komoditas sudah dirujuk data lain (harga, tugas, batas harga) sehingga tidak bisa dihapus; arsipkan komoditas lewat usulan perubahan",
	}
	pesanHapusPasar = pesanHapus{
		adaUsulan: "Pasar masih punya usulan perubahan yang menunggu; setujui atau tolak usulannya dulu",
		dirujuk:   "Pasar sudah dirujuk data lain (harga, tugas, pedagang) sehingga tidak bisa dihapus; nonaktifkan pasar (aktif = false) lewat usulan perubahan",
	}
)

type pesanHapus struct{ adaUsulan, dirujuk string }

func (h *HandlerReferensi) jawabHapus(c *gin.Context, err error, tidakAda string, pesan pesanHapus) {
	switch {
	case err == nil:
		response.NoContent(c)
	case errors.Is(err, domain.ErrTidakDitemukan):
		response.NotFound(c, tidakAda)
	case errors.Is(err, domain.ErrAdaUsulan):
		response.Error(c, stdhttp.StatusConflict, kodeHapusAdaUsulan, pesan.adaUsulan)
	case errors.Is(err, domain.ErrDirujuk):
		response.Error(c, stdhttp.StatusConflict, kodeHapusDirujuk, pesan.dirujuk)
	default:
		response.InternalError(c, "Gagal menghapus data master")
	}
}

// galatBaca memetakan galat baca: tidak ditemukan menjadi 404 tanpa membocorkan sebabnya, sisanya 500.
func (h *HandlerReferensi) galatBaca(c *gin.Context, err error, tidakAda, gagal string) {
	if errors.Is(err, domain.ErrTidakDitemukan) {
		response.NotFound(c, tidakAda)
		return
	}
	response.InternalError(c, gagal)
}

// bindQueryReferensi mengikat query string; ok false berarti respons 400 sudah dikirim.
func bindQueryReferensi(c *gin.Context, req any) bool {
	if err := c.ShouldBindQuery(req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return false
	}
	return true
}

func queryDaftarReferensi(req dto.ListRequest) domain.ListQuery {
	return domain.ListQuery{Search: req.Q, Page: req.Page, Limit: req.Limit}
}
