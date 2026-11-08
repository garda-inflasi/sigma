package http

import (
	"context"
	"errors"
	"io"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// Pesan galat PATCH /v1/auth/me; teksnya bagian dari kontrak API (ditampilkan apa adanya oleh UI).
const (
	pesanNamaPendek      = "Nama minimal 3 karakter"
	pesanNamaPanjang     = "Nama maksimal 100 karakter"
	pesanTeleponTakValid = "Nomor telepon tidak valid"
	pesanTanpaPerubahan  = "Tidak ada perubahan"
)

// meResponse menyusun profil pemilik akun beserta keadaan MFA-nya (GET dan PATCH /v1/auth/me).
func (h *Handler) meResponse(ctx context.Context, user *domain.Pengguna) (dto.MeUserResponse, error) {
	status, err := h.uc.MFAStatus(ctx, user)
	if err != nil {
		return dto.MeUserResponse{}, err
	}
	return dto.MeUserResponse{
		UserResponse:        dto.ToUserResponse(user),
		MfaSetupDiwajibkan:  status.SetupDiwajibkan,
		MfaWajib:            status.Wajib,
		KodeCadanganTersisa: status.KodeCadanganTersisa,
	}, nil
}

// PATCH /v1/auth/me
// Pemilik akun mengubah nama dan/atau telepon sendiri; field lain di badan diabaikan. Menjawab profil terbaru dalam
// bentuk yang sama dengan GET /v1/auth/me.
func (h *Handler) UpdateMe(c *gin.Context) {
	var req dto.UpdateProfilRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if errors.Is(err, io.EOF) { // badan kosong: tidak ada yang diubah
			response.BadRequest(c, pesanTanpaPerubahan)
			return
		}
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	user, err := h.uc.UpdateProfil(audit.RequestContext(c), c.GetString("user_id"), usecase.ProfilPatch{
		Nama:    req.Nama,
		Telepon: req.Telepon,
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrNamaTerlaluPendek):
			response.BadRequest(c, pesanNamaPendek)
		case errors.Is(err, usecase.ErrNamaTerlaluPanjang):
			response.BadRequest(c, pesanNamaPanjang)
		case errors.Is(err, usecase.ErrTeleponTidakValid):
			response.BadRequest(c, pesanTeleponTakValid)
		case errors.Is(err, usecase.ErrTidakAdaPerubahan):
			response.BadRequest(c, pesanTanpaPerubahan)
		case errors.Is(err, usecase.ErrUserNotFound):
			response.NotFound(c, "Pengguna tidak ditemukan")
		default:
			response.InternalError(c, "Gagal memperbarui profil")
		}
		return
	}

	me, err := h.meResponse(c.Request.Context(), user)
	if err != nil {
		response.InternalError(c, "Gagal mengambil status MFA")
		return
	}
	response.Success(c, me)
}
