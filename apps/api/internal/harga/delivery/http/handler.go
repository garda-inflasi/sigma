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
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// totalCountHeader membawa jumlah seluruh hasil filter untuk klien yang memaginasi.
const totalCountHeader = "X-Total-Count"

const dateLayout = "2006-01-02"

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// GET /v1/harga
func (h *Handler) ListHarga(c *gin.Context) {
	h.list(c, "")
}

// GET /v1/harga/saya
func (h *Handler) ListSaya(c *gin.Context) {
	h.list(c, c.GetString("user_id"))
}

// list menjawab daftar entri; petugasID non-kosong membatasi ke entri petugas itu.
func (h *Handler) list(c *gin.Context, petugasID string) {
	var req dto.ListHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	filter := domain.ListFilter{
		Status: req.Status, KabupatenKode: req.Wilayah, KomoditasKode: req.Komoditas,
		PasarID: req.PasarID, PetugasID: petugasID, Urut: req.Urut, Page: req.Page, Limit: req.Limit,
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
func (h *Handler) Ringkasan(c *gin.Context) {
	var req dto.RingkasanHargaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	r, err := h.uc.Ringkasan(c.Request.Context(), req.Hari)
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
func (h *Handler) GetHargaByID(c *gin.Context) {
	entri, err := h.uc.GetHargaByID(c.Request.Context(), c.Param("id"))
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
// ajukan harga.input, aksi lain harga.validasi.
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

	hasil, err := h.uc.BulkAction(audit.RequestContext(c), req.IDs, req.Aksi, c.GetString("user_id"), req.Alasan)
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
// harga.lihat membuka semua entri; petugas dengan harga.input hanya entrinya sendiri.
func (h *Handler) Riwayat(c *gin.Context) {
	items, err := h.uc.Riwayat(c.Request.Context(), c.Param("id"), c.GetString("user_id"), middleware.HasPermission(c, "harga.lihat"))
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
