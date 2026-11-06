package http

import (
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// jawabGalatFilterPengguna menjawab 400 bila err adalah masukan filter/urutan daftar pengguna yang salah
// (*domain.FilterPenggunaError) dan melaporkan apakah sudah menjawab. Dipakai bersama daftar dan ekspor.
func jawabGalatFilterPengguna(c *gin.Context, err error) bool {
	var salah *domain.FilterPenggunaError
	if !errors.As(err, &salah) {
		return false
	}
	response.BadRequest(c, salah.Pesan)
	return true
}

// POST /v1/users/:id/reset-mfa
// Admin (pengguna.kelola) mereset MFA seorang pengguna yang kehilangan autentikatornya: MFA mati, rahasia dan
// kode cadangan dihapus, semua sesinya berakhir. Tercatat di audit (RESET_MFA). Idempoten: pengguna tanpa MFA
// dijawab 200 dengan mfa_aktif_sebelumnya false. Akun sendiri ditolak 400 (gunakan /v1/auth/mfa/disable).
func (h *Handler) ResetMFAPengguna(c *gin.Context) {
	id := c.Param("id")

	res, err := h.uc.ResetMFAPengguna(audit.RequestContext(c), id, c.GetString("user_id"))
	switch {
	case err == nil:
		response.Success(c, dto.ResetMFAResponse{ID: id, MFAAktifSebelumnya: res.MFAAktifSebelumnya, SesiDicabut: res.SesiDicabut})
	case errors.Is(err, usecase.ErrUserNotFound):
		response.NotFound(c, "Pengguna tidak ditemukan")
	case errors.Is(err, usecase.ErrCannotResetOwnMFA):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "Gagal mereset MFA")
	}
}

// GET /v1/users/:id/riwayat-peran
// Riwayat perubahan peran satu pengguna dari jejak audit, terbaru lebih dulu (pengguna.kelola).
func (h *Handler) RiwayatPeranPengguna(c *gin.Context) {
	var req dto.RiwayatPeranRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	res, err := h.uc.RiwayatPeran(c.Request.Context(), c.Param("id"), req.Limit)
	switch {
	case err == nil:
		response.Success(c, dto.ToRiwayatPeranResponse(res.Items, res.Terpotong))
	case errors.Is(err, usecase.ErrUserNotFound):
		response.NotFound(c, "Pengguna tidak ditemukan")
	default:
		response.InternalError(c, "Gagal mengambil riwayat peran")
	}
}
