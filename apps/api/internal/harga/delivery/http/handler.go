package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/harga/domain"
	"github.com/garda-inflasi/api/internal/harga/dto"
	"github.com/garda-inflasi/api/internal/harga/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// totalCountHeader membawa jumlah seluruh hasil filter untuk klien yang memaginasi.
const totalCountHeader = "X-Total-Count"

const dateLayout = "2006-01-02"

// maksKodeEntri adalah panjang kolom kode (varchar(64)): rujukan lebih panjang pasti bukan kode yang ada.
const maksKodeEntri = 64

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// cakupanBaca menurunkan cakupan baris pemanggil untuk membuka entri (02-B). Pemegang harga.lihat mengikuti perannya
// (kabupaten tugas, atau seluruh Aceh); tanpa harga.lihat hanya entri yang ia catat. Gagal-tutup: bila cakupan tidak
// dapat dipastikan, request dijawab 500, bukan diteruskan dengan cakupan yang lebih luas.
func (h *Handler) cakupanBaca(c *gin.Context) (domain.Cakupan, bool) {
	cakupan, err := h.uc.CakupanBaca(c.Request.Context(), c.GetString("user_id"), middleware.HasPermission(c, "harga.lihat"))
	if err != nil {
		response.InternalError(c, "Gagal menentukan cakupan data")
		return domain.Cakupan{}, false
	}
	return cakupan, true
}

// cakupanPeran adalah cakupan menurut peran saja, untuk aksi yang izinnya diperiksa per aksi (bulk) atau yang
// dilindungi harga.lihat di rute (daftar, ringkasan).
func (h *Handler) cakupanPeran(c *gin.Context) (domain.Cakupan, bool) {
	cakupan, err := h.uc.Cakupan(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		response.InternalError(c, "Gagal menentukan cakupan data")
		return domain.Cakupan{}, false
	}
	return cakupan, true
}

// GET /v1/harga
//
// Hanya entri dalam cakupan peran pemanggil: kabupaten tugas untuk Koordinator dan Agregator (ditambah entri yang ia
// catat sendiri), seluruh Aceh untuk peran tingkat provinsi.
func (h *Handler) ListHarga(c *gin.Context) {
	cakupan, ok := h.cakupanPeran(c)
	if !ok {
		return
	}
	h.list(c, "", cakupan)
}

// GET /v1/harga/saya
func (h *Handler) ListSaya(c *gin.Context) {
	userID := c.GetString("user_id")
	h.list(c, userID, domain.CakupanMilik(userID))
}

// list menjawab daftar entri dalam cakupan; petugasID non-kosong membatasi lagi ke entri petugas itu.
func (h *Handler) list(c *gin.Context, petugasID string, cakupan domain.Cakupan) {
	var req dto.ListHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	filter := domain.ListFilter{
		Status: req.Status, KabupatenKode: req.Wilayah, KomoditasKode: req.Komoditas,
		PasarID: req.PasarID, PetugasID: petugasID, Urut: req.Urut, Page: req.Page, Limit: req.Limit,
		Cakupan: cakupan,
	}
	if req.Dari != "" {
		dari, _ := time.ParseInLocation(dateLayout, req.Dari, domain.WIB)
		filter.Dari = &dari
	}
	if req.Sampai != "" {
		sampai, _ := time.ParseInLocation(dateLayout, req.Sampai, domain.WIB)
		sampai = sampai.AddDate(0, 0, 1)
		filter.Sampai = &sampai
	}

	items, total, err := h.uc.ListHarga(c.Request.Context(), filter)
	if err != nil {
		if errors.Is(err, usecase.ErrValidasi) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal mengambil data harga")
		return
	}

	c.Header(totalCountHeader, strconv.FormatInt(total, 10))
	response.Success(c, dto.ToEntriHargaResponses(items))
}

// GET /v1/harga/ringkasan
//
// Hitungan entri per status hanya dari entri dalam cakupan pemanggil (sama dengan GET /v1/harga).
func (h *Handler) Ringkasan(c *gin.Context) {
	var req dto.RingkasanHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	cakupan, ok := h.cakupanPeran(c)
	if !ok {
		return
	}
	r, err := h.uc.Ringkasan(c.Request.Context(), req.Hari, cakupan)
	if err != nil {
		response.InternalError(c, "Gagal mengambil ringkasan harga")
		return
	}
	response.Success(c, dto.RingkasanHargaResponse{
		Draf: r.Draf, Diajukan: r.Diajukan, Divalidasi: r.Divalidasi,
		Dipublikasi: r.Dipublikasi, Ditolak: r.Ditolak, Anomali: r.Anomali,
	})
}

// GET /v1/harga/tren
func (h *Handler) GetTren(c *gin.Context) {
	var req dto.TrenHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	tren, err := h.uc.GetTren(c.Request.Context(), req.Komoditas, req.Wilayah, req.Hari, req.Interval)
	if err != nil {
		if errors.Is(err, usecase.ErrValidasi) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal mengambil data tren")
		return
	}

	items := make([]dto.TrenHargaResponse, len(tren))
	for i, t := range tren {
		items[i] = dto.TrenHargaResponse{Tanggal: t.Tanggal, Produsen: t.Produsen, Konsumen: t.Konsumen}
	}
	response.Success(c, items)
}

// idempotencyKeyHeader membawa kunci yang membuat kiriman ulang aman; maksimal sepanjang kolom basis data.
const (
	idempotencyKeyHeader = "Idempotency-Key"
	maxIdempotencyKey    = 255
	replayedHeader       = "Idempotent-Replayed"
)

// POST /v1/harga
//
// Header Idempotency-Key bersifat opsional di sini.
func (h *Handler) CreateHarga(c *gin.Context) {
	h.create(c, false)
}

// POST /v1/lapangan/harga
//
// Dipakai sync engine PWA: Idempotency-Key wajib, kiriman ulang mengembalikan
// entri semula, dan 409 memuat versi server dari harga yang bentrok.
func (h *Handler) KirimLapangan(c *gin.Context) {
	h.create(c, true)
}

func (h *Handler) create(c *gin.Context, kunciWajib bool) {
	var req dto.CreateHargaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	kunci := strings.TrimSpace(c.GetHeader(idempotencyKeyHeader))
	if kunciWajib && kunci == "" {
		response.BadRequest(c, "Header "+idempotencyKeyHeader+" wajib diisi")
		return
	}
	if len(kunci) > maxIdempotencyKey {
		response.BadRequest(c, "Header "+idempotencyKeyHeader+" terlalu panjang")
		return
	}

	hasil, err := h.uc.CreateHarga(audit.RequestContext(c), usecase.CreateInput{
		KomoditasKode: req.KomoditasKode, PasarID: req.PasarID, Tipe: req.Tipe, Harga: req.Harga,
		Kualitas: req.Kualitas, KodePedagang: req.KodePedagang, Catatan: req.Catatan, DicatatPada: req.DicatatPada,
		Lat: req.Lat, Lng: req.Lng, IdempotencyKey: kunci, Pertahankan: req.Pertahankan,
	}, c.GetString("user_id"))
	if err != nil {
		var bentrok *usecase.BentrokError
		switch {
		case errors.As(err, &bentrok):
			response.ErrorWithData(c, http.StatusConflict, "409", bentrok.Error(), dto.BentrokResponse{VersiServer: dto.VersiServerResponse{
				ID: bentrok.Server.ID, Harga: bentrok.Server.Harga, DicatatPada: bentrok.Server.DicatatPada, Petugas: bentrok.Server.PetugasNama,
			}})
		case errors.Is(err, usecase.ErrValidasi):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "Gagal menyimpan data harga")
		}
		return
	}

	body := dto.CreateHargaResponse{ID: hasil.Entri.ID, Status: hasil.Entri.Status}
	if hasil.Diulang {
		c.Header(replayedHeader, "true")
		response.Success(c, body)
		return
	}
	response.Created(c, body)
}

// GET /v1/harga/:id
//
// :id adalah ID BIGINT atau kode lama entri (HRG-D00-00-01-K, UUID dari aplikasi lama): tautan lama tetap berlaku.
// Teks kosong atau lebih dari 64 karakter dijawab 404 tanpa menyentuh basis data. Entri di luar cakupan pemanggil
// (02-B) juga dijawab 404 dengan pesan yang sama dengan entri yang tidak ada.
func (h *Handler) GetHargaByID(c *gin.Context) {
	ref, ok := idx.ParamIDAtauKode(c, "id", maksKodeEntri, "Entri harga tidak ditemukan")
	if !ok {
		return
	}
	cakupan, ok := h.cakupanBaca(c)
	if !ok {
		return
	}
	entri, err := h.uc.GetHargaByID(c.Request.Context(), ref, cakupan)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Entri harga tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil data harga")
		return
	}
	response.Success(c, dto.ToHargaDetailResponse(*entri))
}

// POST /v1/harga/bulk
//
// Izin diperiksa per aksi (domain.BulkAction.Izin): publikasi memerlukan harga.publikasi,
// ajukan harga.input, aksi lain harga.validasi. ids adalah daftar ID BIGINT (string angka atau angka JSON);
// satu ID yang salah bentuk (mis. kode lama HRG-...) menolak seluruh permintaan dengan 400. Entri di luar cakupan
// aktor (02-B) dilewati dan dilaporkan tidak_ditemukan di rincian, sama dengan ID yang tidak ada.
func (h *Handler) BulkHarga(c *gin.Context) {
	var req dto.BulkHargaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if izin := domain.BulkAction(req.Aksi).Izin(); !middleware.HasPermission(c, izin) {
		response.Forbidden(c, "Anda tidak memiliki izin "+izin)
		return
	}

	ids, err := idx.ParseDaftar(req.IDs)
	if err != nil {
		response.BadRequest(c, "Validasi gagal: ids "+err.Error())
		return
	}

	cakupan, ok := h.cakupanPeran(c)
	if !ok {
		return
	}
	hasil, err := h.uc.BulkAction(audit.RequestContext(c), ids, req.Aksi, c.GetString("user_id"), req.Alasan, cakupan)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidAction):
			response.BadRequest(c, "Aksi tidak valid")
		case errors.Is(err, usecase.ErrValidasi):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "Gagal memproses aksi massal")
		}
		return
	}

	rincian := make([]dto.RincianBulkResponse, len(hasil.Rincian))
	for i, r := range hasil.Rincian {
		rincian[i] = dto.RincianBulkResponse{ID: r.ID, Sebab: r.Sebab}
	}
	response.Success(c, dto.BulkHargaResponse{Berhasil: hasil.Berhasil, Gagal: hasil.Gagal, Rincian: rincian})
}

// GET /v1/harga/:id/riwayat
//
// Pemegang harga.lihat membuka entri dalam cakupan perannya (02-B); petugas dengan harga.input saja hanya entrinya
// sendiri. Entri di luar cakupan dijawab 404, sama dengan entri yang tidak ada. :id seperti GET /v1/harga/:id
// (ID BIGINT atau kode lama).
func (h *Handler) Riwayat(c *gin.Context) {
	ref, ok := idx.ParamIDAtauKode(c, "id", maksKodeEntri, "Entri harga tidak ditemukan")
	if !ok {
		return
	}
	cakupan, ok := h.cakupanBaca(c)
	if !ok {
		return
	}
	items, err := h.uc.Riwayat(c.Request.Context(), ref, cakupan)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Entri harga tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil riwayat status")
		return
	}
	response.Success(c, dto.ToRiwayatStatusResponses(items))
}

// GET /v1/harga/:id/versi
//
// Seluruh versi isi entri (siapa, kapan, aksi, kolom yang berubah, salinan baris), yang terlama lebih dulu.
// Akses dan :id sama dengan GET /v1/harga/:id/riwayat.
func (h *Handler) Versi(c *gin.Context) {
	ref, ok := idx.ParamIDAtauKode(c, "id", maksKodeEntri, "Entri harga tidak ditemukan")
	if !ok {
		return
	}
	cakupan, ok := h.cakupanBaca(c)
	if !ok {
		return
	}
	items, err := h.uc.Versi(c.Request.Context(), ref, cakupan)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Entri harga tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil versi entri")
		return
	}
	response.Success(c, dto.ToVersiEntriResponses(items))
}

// GET /v1/harga/:id/lineage
//
// Jejak asal entri (PRD §1.4): pencatat, validasi otomatis, validasi, publikasi, penolakan, versi, riwayat status,
// dan rujukan audit trail. Akses dan :id sama dengan GET /v1/harga/:id/riwayat.
func (h *Handler) Lineage(c *gin.Context) {
	ref, ok := idx.ParamIDAtauKode(c, "id", maksKodeEntri, "Entri harga tidak ditemukan")
	if !ok {
		return
	}
	cakupan, ok := h.cakupanBaca(c)
	if !ok {
		return
	}
	lineage, err := h.uc.Lineage(c.Request.Context(), ref, cakupan)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Entri harga tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil lineage entri")
		return
	}
	response.Success(c, dto.ToLineageResponse(lineage))
}
