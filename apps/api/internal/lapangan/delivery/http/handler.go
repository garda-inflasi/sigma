package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/lapangan/dto"
	"github.com/garda-inflasi/api/internal/lapangan/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// maksKodeTugas adalah panjang kolom kode (varchar(64)): rujukan lebih panjang pasti bukan kode yang ada.
const maksKodeTugas = 64

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// pemanggil membaca identitas dari token; tugas.kelola membuka tugas petugas lain.
func pemanggil(c *gin.Context) usecase.Pemanggil {
	return usecase.Pemanggil{ID: c.GetString("user_id"), Semua: middleware.HasPermission(c, "tugas.kelola")}
}

// GET /v1/lapangan/tugas/saya
func (h *Handler) ListSaya(c *gin.Context) {
	items, err := h.uc.ListSaya(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		response.InternalError(c, "Gagal mengambil data tugas lapangan")
		return
	}

	res := make([]dto.TugasLapanganResponse, len(items))
	for i, item := range items {
		res[i] = dto.ToTugasLapanganResponse(item)
	}
	response.Success(c, res)
}

// GET /v1/lapangan/tugas/:id
//
// :id adalah ID BIGINT atau kode lama tugas (TGS-<slug>-<n>): tautan dan antrean PWA yang dibuat sebelum migrasi
// 000111 tetap berlaku. Teks kosong atau lebih dari 64 karakter dijawab 404 tanpa menyentuh basis data.
func (h *Handler) Get(c *gin.Context) {
	ref, ok := idx.ParamIDAtauKode(c, "id", maksKodeTugas, "Tugas tidak ditemukan")
	if !ok {
		return
	}
	item, err := h.uc.GetDetail(c.Request.Context(), ref, pemanggil(c))
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(c, "Tugas tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil data tugas lapangan")
		return
	}
	response.Success(c, dto.ToTugasDetailResponse(*item))
}

// POST /v1/lapangan/tugas/bulk
//
// ids berisi ID BIGINT (string angka) atau kode lama (TGS-...), supaya klien yang masih memegang ID lama tetap
// dilayani. Rujukan lebih panjang dari 64 karakter menolak seluruh permintaan dengan 400.
func (h *Handler) Bulk(c *gin.Context) {
	var req dto.BulkTugasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	refs, err := rujukanDaftar(req.IDs)
	if err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	hasil, err := h.uc.BulkAction(audit.RequestContext(c), refs, req.Aksi, pemanggil(c))
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

	response.Success(c, dto.BulkTugasResponse{Berhasil: hasil.Berhasil, Gagal: hasil.Gagal})
}

// GET /v1/lapangan/acuan
//
// Mengirim ETag dari versi data; permintaan dengan If-None-Match yang sama dijawab 304
// tanpa isi sehingga unduhan ulang di perangkat murah.
func (h *Handler) Acuan(c *gin.Context) {
	data, err := h.uc.Acuan(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Gagal mengambil data acuan")
		return
	}

	etag := `"` + data.Versi + `"`
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	response.Success(c, dto.ToAcuanResponse(data))
}

// rujukanDaftar membaca daftar id aksi massal: bilangan bulat positif menjadi ID BIGINT, teks lain (kode lama
// seperti TGS-mahasiswa_1-1) menjadi rujukan kode. Entri kosong dibuang.
func rujukanDaftar(ids []string) ([]idx.RujukanID, error) {
	refs := make([]idx.RujukanID, 0, len(ids))
	for _, teks := range ids {
		teks = strings.TrimSpace(teks)
		if teks == "" {
			continue
		}
		if id, err := idx.Parse(teks); err == nil {
			refs = append(refs, idx.RujukanID{ID: id})
			continue
		}
		if len(teks) > maksKodeTugas {
			return nil, fmt.Errorf("ids: rujukan lebih dari %d karakter", maksKodeTugas)
		}
		refs = append(refs, idx.RujukanID{Kode: teks})
	}
	return refs, nil
}
