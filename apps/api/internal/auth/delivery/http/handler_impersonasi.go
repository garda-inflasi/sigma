package http

import (
	"errors"
	"net/http"

	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/garda-inflasi/api/pkg/token"
	"github.com/gin-gonic/gin"
)

// Impersonasi peran dan akun (docs/impersonasi.md). Hak ditentukan usecase dari catatan di basis data; rute kebijakan dan
// riwayat juga dijaga izin impersonasi.kelola dan peran ADMIN_SISTEM di routes.

// jawabGalatImpersonasi memetakan galat usecase impersonasi ke jawaban HTTP; false bila galatnya bukan galat impersonasi
// (pemanggil menjawab sendiri).
func jawabGalatImpersonasi(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, usecase.ErrImpersonasiNonaktif):
		response.Error(c, http.StatusForbidden, dto.KodeImpersonasiNonaktif, "Impersonasi sedang dinonaktifkan oleh kebijakan")
	case errors.Is(err, usecase.ErrImpersonasiDitolak):
		response.Error(c, http.StatusForbidden, dto.KodeImpersonasiDitolak, "Anda tidak berhak melakukan impersonasi ke akun ini")
	case errors.Is(err, usecase.ErrAlasanImpersonasiWajib):
		response.Error(c, http.StatusBadRequest, dto.KodeAlasanWajib, "Alasan impersonasi wajib diisi")
	case errors.Is(err, usecase.ErrAlasanTerlaluPanjang):
		response.BadRequest(c, err.Error())
	case errors.Is(err, usecase.ErrTargetWajib):
		response.BadRequest(c, err.Error())
	case errors.Is(err, usecase.ErrTargetTidakAda), errors.Is(err, usecase.ErrRoleNotFound):
		response.Error(c, http.StatusNotFound, dto.KodeTargetTidakAda, "Akun sasaran tidak ditemukan atau tidak aktif")
	case errors.Is(err, usecase.ErrTargetAmbigu):
		response.Error(c, http.StatusConflict, dto.KodeTargetAmbigu, "Ada lebih dari satu akun berperan ini, pilih salah satu akun")
	case errors.Is(err, usecase.ErrTidakAdaImpersonasi):
		response.Error(c, http.StatusBadRequest, dto.KodeTidakAdaImpersonasi, "Sesi ini bukan sesi impersonasi")
	case errors.Is(err, token.ErrImpersonationEnded):
		response.Error(c, http.StatusUnauthorized, middleware.ImpersonationEndedCode, middleware.ImpersonationEndedMessage)
	case errors.Is(err, usecase.ErrInvalidToken):
		response.Unauthorized(c, "Sesi tidak valid, silakan masuk kembali")
	case errors.Is(err, usecase.ErrImpersonasiTidakDitemukan):
		response.NotFound(c, "Sesi impersonasi tidak ditemukan atau sudah berakhir")
	default:
		return false
	}
	return true
}

// GET /v1/auth/impersonation/targets
// Siapa yang dapat di-impersonate oleh pemanggil. Tidak berhak bukan galat: boleh=false dan daftar kosong.
func (h *Handler) SasaranImpersonasi(c *gin.Context) {
	daftar, err := h.uc.DaftarSasaranImpersonasi(requestContext(c), c.GetString("user_id"))
	if err != nil {
		if !jawabGalatImpersonasi(c, err) {
			response.InternalError(c, "Gagal mengambil daftar sasaran impersonasi")
		}
		return
	}
	response.Success(c, dto.ToSasaranImpersonasiResponse(*daftar))
}

// POST /v1/auth/impersonation/start
// Memulai impersonasi ke satu akun (target_pengguna_id) atau satu peran (target_peran, harus tepat satu akun). Bila
// pemanggil sedang impersonasi, ini mengganti sasarannya; hak dievaluasi terhadap pelaku asli.
func (h *Handler) MulaiImpersonasi(c *gin.Context) {
	var req dto.MulaiImpersonasiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	hasil, err := h.uc.MulaiImpersonasi(requestContext(c), c.GetString("user_id"), usecase.MulaiImpersonasiInput{
		TargetPenggunaID: req.TargetPenggunaID, TargetPeran: req.TargetPeran, Alasan: req.Alasan,
	}, c.ClientIP())
	if err != nil {
		if !jawabGalatImpersonasi(c, err) {
			response.InternalError(c, "Gagal memulai impersonasi")
		}
		return
	}
	response.Success(c, dto.MulaiImpersonasiResponse{
		AccessToken: hasil.AccessToken, RefreshToken: hasil.RefreshToken,
		User: dto.ToUserResponse(hasil.User), Impersonasi: dto.ToImpersonasiMetaResponse(hasil.Meta),
	})
}

// POST /v1/auth/impersonation/stop
// Kembali ke akun asli. Sah bagi sesi impersonasi apa pun (tanpa izin khusus); pelaku asli ditentukan dari catatan di
// server yang cocok dengan sesi token, bukan dari header atau body.
func (h *Handler) HentikanImpersonasi(c *gin.Context) {
	hasil, err := h.uc.HentikanImpersonasi(requestContext(c), c.ClientIP())
	if err != nil {
		if !jawabGalatImpersonasi(c, err) {
			response.InternalError(c, "Gagal kembali ke akun asli")
		}
		return
	}
	response.Success(c, dto.HentiImpersonasiResponse{
		AccessToken: hasil.AccessToken, RefreshToken: hasil.RefreshToken, User: dto.ToUserResponse(hasil.User),
	})
}

// GET /v1/admin/impersonation/policy
func (h *Handler) KebijakanImpersonasi(c *gin.Context) {
	k, err := h.uc.GetKebijakanImpersonasi(requestContext(c), c.GetString("user_id"))
	if err != nil {
		if !jawabGalatImpersonasi(c, err) {
			response.InternalError(c, "Gagal mengambil kebijakan impersonasi")
		}
		return
	}
	response.Success(c, dto.ToKebijakanResponse(*k))
}

// PUT /v1/admin/impersonation/policy
// Mengganti seluruh kebijakan dengan kunci optimistis versi; versi usang dijawab 409 beserta kebijakan terkini.
func (h *Handler) SimpanKebijakanImpersonasi(c *gin.Context) {
	var req dto.KebijakanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	k, err := h.uc.SimpanKebijakanImpersonasi(requestContext(c), c.GetString("user_id"), req.ToInput())
	if err != nil {
		var konflik *usecase.KonflikKebijakan
		switch {
		case errors.As(err, &konflik):
			response.ErrorWithData(c, http.StatusConflict, dto.KodeVersiKebijakanBerubah,
				"Kebijakan sudah diubah admin lain. Muat ulang lalu ulangi perubahan Anda.", dto.ToKebijakanResponse(konflik.Saat))
		case errors.Is(err, usecase.ErrKebijakanTidakValid):
			response.Error(c, http.StatusBadRequest, dto.KodeKebijakanTidakValid, err.Error())
		case jawabGalatImpersonasi(c, err):
		default:
			response.InternalError(c, "Gagal menyimpan kebijakan impersonasi")
		}
		return
	}
	response.Success(c, dto.ToKebijakanResponse(*k))
}

// GET /v1/admin/impersonation/riwayat?status=semua|aktif&limit=&cursor=
func (h *Handler) RiwayatImpersonasi(c *gin.Context) {
	var req dto.RiwayatImpersonasiRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}
	page, err := h.uc.ListRiwayatImpersonasi(requestContext(c), c.GetString("user_id"), req.Status, req.Limit, req.Cursor)
	switch {
	case err == nil:
		items := make([]dto.RiwayatImpersonasiResponse, len(page.Items))
		for i, r := range page.Items {
			items[i] = dto.ToRiwayatImpersonasiResponse(r)
		}
		response.Success(c, dto.RiwayatImpersonasiPageResponse{Items: items, NextCursor: page.NextCursor})
	case errors.Is(err, usecase.ErrRiwayatSesiTidakValid):
		response.BadRequest(c, "Status atau cursor tidak valid")
	case jawabGalatImpersonasi(c, err):
	default:
		response.InternalError(c, "Gagal mengambil riwayat impersonasi")
	}
}

// POST /v1/admin/impersonation/riwayat/:id/akhiri
// Mengakhiri paksa impersonasi yang masih hidup; tokennya mati seketika.
func (h *Handler) AkhiriImpersonasi(c *gin.Context) {
	id, ok := idx.ParamUUID(c, "id", "Sesi impersonasi tidak ditemukan atau sudah berakhir")
	if !ok {
		return
	}
	if err := h.uc.AkhiriImpersonasiPaksa(requestContext(c), c.GetString("user_id"), id); err != nil {
		if !jawabGalatImpersonasi(c, err) {
			response.InternalError(c, "Gagal mengakhiri impersonasi")
		}
		return
	}
	response.Success(c, gin.H{"berakhir": true})
}
