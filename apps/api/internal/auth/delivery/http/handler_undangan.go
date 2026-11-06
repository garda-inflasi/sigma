package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// jawabGalatAkunMenunggu menjawab galat yang dibagi jalur PATCH/reset-password dan undangan: status akun
// menunggu aktivasi (409) dan masukan yang salah (400). true bila galat sudah dijawab.
func jawabGalatAkunMenunggu(c *gin.Context, err error) bool {
	var validasi *usecase.GalatValidasi
	switch {
	case errors.Is(err, usecase.ErrAkunMenunggu):
		response.Conflict(c, err.Error())
	case errors.As(err, &validasi):
		response.BadRequest(c, validasi.Pesan)
	default:
		return false
	}
	return true
}

func undanganResponse(h *usecase.HasilUndangan) dto.UndanganResponse {
	return dto.UndanganResponse{
		User:          dto.ToUserResponse(h.User),
		BerlakuSampai: h.BerlakuSampai.UTC(),
		EmailTerkirim: h.EmailTerkirim,
	}
}

// POST /v1/users/undang
// Membuat akun berstatus MENUNGGU tanpa kata sandi dan mengirim tautan aktivasi ke emailnya (pengguna.kelola).
func (h *Handler) UndangPengguna(c *gin.Context) {
	var req dto.UndangPenggunaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	hasil, err := h.uc.UndangPengguna(audit.RequestContext(c), usecase.UndangInput{
		Nama: req.Nama, Email: req.Email, Peran: req.Peran, Lembaga: req.Lembaga, KabupatenKode: req.KabupatenKode,
	}, c.GetString("user_id"))
	if err != nil {
		if errors.Is(err, usecase.ErrEmailExists) {
			response.Conflict(c, "Email sudah terdaftar")
			return
		}
		if jawabGalatAkunMenunggu(c, err) {
			return
		}
		response.InternalError(c, "Gagal mengundang pengguna")
		return
	}

	response.Created(c, undanganResponse(hasil))
}

// POST /v1/users/:id/undang-ulang
// Mengganti undangan akun yang masih menunggu aktivasi: tautan lama hangus, tautan baru dikirim (pengguna.kelola).
func (h *Handler) UndangUlang(c *gin.Context) {
	hasil, err := h.uc.KirimUlangUndangan(audit.RequestContext(c), c.Param("id"), c.GetString("user_id"))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrUserNotFound):
			response.NotFound(c, "Pengguna tidak ditemukan")
		case errors.Is(err, usecase.ErrBukanMenunggu):
			response.Conflict(c, "Akun ini tidak sedang menunggu aktivasi, jadi tidak perlu diundang ulang.")
		default:
			response.InternalError(c, "Gagal mengirim ulang undangan")
		}
		return
	}

	response.Success(c, undanganResponse(hasil))
}

// POST /v1/auth/aktivasi
// Publik: menetapkan kata sandi pertama dari tautan undangan. Tidak menerbitkan sesi; pengguna masuk seperti biasa.
func (h *Handler) AktivasiAkun(c *gin.Context) {
	var req dto.AktivasiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if err := h.uc.AktifkanAkun(audit.RequestContext(c), req.Token, req.Password); err != nil {
		switch {
		case errors.Is(err, usecase.ErrUndanganKedaluwarsa):
			response.Unauthorized(c, "Tautan undangan sudah kedaluwarsa. Minta administrator mengirim undangan baru.")
		case errors.Is(err, usecase.ErrUndanganInvalid):
			response.Unauthorized(c, "Tautan undangan tidak valid atau sudah dipakai. Minta administrator mengirim undangan baru.")
		case errors.Is(err, usecase.ErrWeakPassword):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "Gagal mengaktifkan akun")
		}
		return
	}

	response.Success(c, nil)
}

// POST /v1/users/impor
// Mengurai dan memeriksa CSV pengguna; dry_run=false membuat akun undangan untuk baris yang sah (pengguna.kelola).
func (h *Handler) ImporPengguna(c *gin.Context) {
	// JSON membungkus CSV (kutip dan baris baru di-escape), jadi batas badan sedikit di atas batas teks.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(usecase.ImporMaksByte)*2+4096)
	var req dto.ImporPenggunaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var terlalu *http.MaxBytesError
		if errors.As(err, &terlalu) {
			response.Error(c, http.StatusRequestEntityTooLarge, "413", "Berkas terlalu besar.")
			return
		}
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	hasil, err := h.uc.ImporPengguna(audit.RequestContext(c), req.CSV, *req.DryRun, c.GetString("user_id"))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrImporTerlaluBesar):
			response.Error(c, http.StatusRequestEntityTooLarge, "413", fmt.Sprintf("Berkas terlalu besar (maksimal %d KB).", usecase.ImporMaksByte/1024))
		case errors.Is(err, usecase.ErrImporBentrok):
			response.Conflict(c, "Ada email yang baru saja didaftarkan pihak lain. Tidak ada akun yang dibuat; periksa ulang berkas lalu impor lagi.")
		default:
			if jawabGalatAkunMenunggu(c, err) {
				return
			}
			response.InternalError(c, "Gagal mengimpor pengguna")
		}
		return
	}

	response.Success(c, imporResponse(hasil))
}

func imporResponse(h *usecase.HasilImpor) dto.ImporPenggunaResponse {
	baris := make([]dto.ImporBarisResponse, len(h.Baris))
	for i, b := range h.Baris {
		galat := b.Galat
		if galat == nil {
			galat = []string{}
		}
		baris[i] = dto.ImporBarisResponse{
			Baris: b.Baris, Nama: b.Nama, Email: b.Email, Peran: b.Peran, Lembaga: b.Lembaga,
			KabupatenKode: b.KabupatenKode, Status: b.Status, Galat: galat, ID: b.ID,
		}
	}
	return dto.ImporPenggunaResponse{
		DryRun: h.DryRun, Total: h.Total, Valid: h.Valid, Galat: h.Galat, Dibuat: h.Dibuat,
		EmailAktif: h.EmailAktif, Baris: baris,
	}
}

// GET /v1/users/ekspor
// CSV pengguna (titik koma, UTF-8 BOM) dengan filter daftar yang sama dan, opsional, hanya id terpilih
// (pengguna.kelola). Tercatat sebagai EKSPOR_DATA sebelum berkas diserahkan.
func (h *Handler) EksporPengguna(c *gin.Context) {
	var req dto.EksporPenggunaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	filter := req.Filter()
	if strings.TrimSpace(req.IDs) != "" && len(filter.IDs) == 0 {
		// Pilihan yang ternyata kosong tidak boleh berubah diam-diam menjadi "semua pengguna".
		response.BadRequest(c, "Daftar id pengguna tidak valid.")
		return
	}

	hasil, err := h.uc.EksporPengguna(audit.RequestContext(c), filter, c.GetString("user_id"))
	if err != nil {
		if jawabGalatAkunMenunggu(c, err) {
			return
		}
		response.InternalError(c, "Gagal mengekspor pengguna")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="pengguna-%s.csv"`, time.Now().Format(time.DateOnly)))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Export-Rows", fmt.Sprint(hasil.Baris))
	if hasil.Terpotong {
		c.Header("X-Export-Truncated", "true")
	}
	c.Data(http.StatusOK, "text/csv; charset=utf-8", hasil.CSV)
}
