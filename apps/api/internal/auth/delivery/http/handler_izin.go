package http

import (
	"errors"
	"net/http"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/idx"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

func roleResponse(r usecase.RoleInfo) dto.RoleResponse {
	return dto.RoleResponse{
		ID: r.ID, Label: r.Label, Lapis: r.Lapis, Permissions: r.Permissions, Bawaan: r.Bawaan,
		Versi: r.Versi, DiperbaruiPada: r.DiperbaruiPada, DiperbaruiOleh: r.DiperbaruiOleh,
	}
}

func izinLangsungResponse(g usecase.IzinLangsungInfo) dto.IzinLangsungResponse {
	return dto.IzinLangsungResponse{
		Izin: g.Izin, Deskripsi: g.Deskripsi, Kelompok: g.Kelompok, BerlakuSampai: g.BerlakuSampai, Alasan: g.Alasan,
		DiberikanOleh: g.DiberikanOleh, DiberikanOlehNama: g.DiberikanOlehNama, DiberikanPada: g.DiberikanPada, Aktif: g.Aktif,
	}
}

// jawabPemegangTerakhir menjawab 409 LAST_PERMISSION_HOLDER: perubahan akan menghapus pemegang terakhir izin
// pengguna.kelola (matriks peran, ganti peran, penonaktifan, penghapusan).
func jawabPemegangTerakhir(c *gin.Context) {
	response.Error(c, http.StatusConflict, dto.KodePemegangTerakhir,
		"Perubahan ditolak: sistem harus tetap punya pengguna aktif yang memegang izin pengguna.kelola.")
}

// PUT /v1/roles/:id/permissions
// Mengganti seluruh izin satu peran (pengguna.kelola). Kunci optimistis `versi`: 409 ROLE_VERSION_CONFLICT bila
// admin lain sudah mengubahnya (data berisi peran saat ini). 409 LAST_PERMISSION_HOLDER bila perubahan membuat
// sistem kehilangan pemegang terakhir pengguna.kelola. Tercatat di audit (UPDATE_ROLE_PERMISSIONS).
func (h *Handler) UpdateRolePermissions(c *gin.Context) {
	var req dto.UpdateRolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	role, err := h.uc.UpdateRolePermissions(audit.RequestContext(c), c.Param("id"), req.Versi, req.Permissions, c.GetString("user_id"))
	if err != nil {
		var konflik *usecase.KonflikVersiPeran
		switch {
		case errors.As(err, &konflik):
			response.ErrorWithData(c, http.StatusConflict, dto.KodeVersiPeranBerubah,
				"Matriks peran sudah diubah admin lain. Muat ulang lalu ulangi perubahan Anda.", roleResponse(konflik.Saat))
		case errors.Is(err, usecase.ErrRoleNotFound):
			response.NotFound(c, "Peran tidak dikenal")
		case errors.Is(err, usecase.ErrIzinTidakDikenal):
			response.Error(c, http.StatusBadRequest, dto.KodeIzinTidakDikenal, err.Error())
		case errors.Is(err, usecase.ErrPemegangTerakhir):
			jawabPemegangTerakhir(c)
		default:
			response.InternalError(c, "Gagal menyimpan matriks peran")
		}
		return
	}

	response.Success(c, roleResponse(*role))
}

// GET /v1/users/:id/permissions
// Izin seorang pengguna: dari perannya, langsung (aktif dan kedaluwarsa), dan yang berlaku sekarang.
func (h *Handler) ListUserPermissions(c *gin.Context) {
	id, ok := idx.ParamUUID(c, "id", "Pengguna tidak ditemukan")
	if !ok {
		return
	}
	info, err := h.uc.ListUserPermissions(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrUserNotFound) {
			response.NotFound(c, "Pengguna tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil izin pengguna")
		return
	}

	out := dto.IzinPenggunaResponse{Peran: info.Peran, IzinPeran: info.IzinPeran, Efektif: info.Efektif, Langsung: make([]dto.IzinLangsungResponse, len(info.Langsung))}
	for i, g := range info.Langsung {
		out.Langsung[i] = izinLangsungResponse(g)
	}
	response.Success(c, out)
}

// POST /v1/users/:id/permissions
// Memberi izin langsung dengan alasan wajib dan masa berlaku opsional (pengguna.kelola). Izin yang sudah dimiliki
// lewat peran atau masih berlaku sebagai izin langsung dijawab 409. Tercatat di audit (GRANT_USER_PERMISSION).
func (h *Handler) GrantUserPermission(c *gin.Context) {
	id, ok := idx.ParamUUID(c, "id", "Pengguna tidak ditemukan")
	if !ok {
		return
	}

	var req dto.GrantUserPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	info, err := h.uc.GrantUserPermission(audit.RequestContext(c), usecase.GrantInput{
		UserID: id, Izin: req.Izin, BerlakuSampai: req.BerlakuSampai, Alasan: req.Alasan,
	}, c.GetString("user_id"))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrUserNotFound):
			response.NotFound(c, "Pengguna tidak ditemukan")
		case errors.Is(err, usecase.ErrIzinTidakDikenal):
			response.Error(c, http.StatusBadRequest, dto.KodeIzinTidakDikenal, err.Error())
		case errors.Is(err, usecase.ErrAlasanWajib), errors.Is(err, usecase.ErrAlasanTerlaluPanjang), errors.Is(err, usecase.ErrBerlakuSampaiLampau):
			response.BadRequest(c, err.Error())
		case errors.Is(err, usecase.ErrIzinDariPeran):
			response.Error(c, http.StatusConflict, dto.KodeIzinDariPeran, "Pengguna sudah memiliki izin ini lewat perannya.")
		case errors.Is(err, usecase.ErrIzinLangsungAda):
			response.Error(c, http.StatusConflict, dto.KodeIzinLangsungAda, "Izin langsung ini sudah diberikan dan masih berlaku. Cabut dulu bila ingin mengubahnya.")
		default:
			response.InternalError(c, "Gagal memberi izin langsung")
		}
		return
	}

	response.Created(c, izinLangsungResponse(*info))
}

// DELETE /v1/users/:id/permissions/:izin
// Mencabut izin langsung (aktif maupun kedaluwarsa). Tercatat di audit (REVOKE_USER_PERMISSION).
func (h *Handler) RevokeUserPermission(c *gin.Context) {
	id, ok := idx.ParamUUID(c, "id", "Pengguna tidak ditemukan")
	if !ok {
		return
	}
	izin := c.Param("izin")

	if err := h.uc.RevokeUserPermission(audit.RequestContext(c), id, izin, c.GetString("user_id")); err != nil {
		if errors.Is(err, usecase.ErrIzinLangsungTidakAda) {
			response.Error(c, http.StatusNotFound, dto.KodeIzinLangsungTidakAda, "Izin langsung tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mencabut izin langsung")
		return
	}

	response.Success(c, dto.CabutIzinResponse{ID: id, Izin: izin})
}
