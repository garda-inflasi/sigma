package http

import (
	"errors"
	"net/http"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/qr"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// POST /v1/auth/login
// Returns user profile and tokens for the BFF to set as HttpOnly cookies. Akun dengan MFA aktif
// menerima mfa_required dan mfa_token tanpa sesi.
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	res, err := h.uc.Login(audit.RequestContext(c), req.Email, req.Password, c.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidCredentials):
			response.Error(c, http.StatusUnauthorized, dto.KodeKredensialSalah, dto.PesanKredensialSalah)
		case errors.Is(err, usecase.ErrAccountLocked):
			jawabAkunTerkunci(c, err)
		case errors.Is(err, usecase.ErrAccountInactive):
			response.Error(c, http.StatusForbidden, dto.KodeAkunNonaktif, dto.PesanAkunNonaktif)
		default:
			response.InternalError(c, "Kesalahan server")
		}
		return
	}

	response.Success(c, loginResponse(res))
}

// loginResponse memetakan hasil login; profil dan token sesi hanya ada setelah semua faktor terbukti.
func loginResponse(res *usecase.LoginResult) dto.LoginResponse {
	if res.MFARequired {
		return dto.LoginResponse{MFARequired: true, MFAToken: res.MFAToken}
	}
	user := dto.ToUserResponse(res.User)
	return dto.LoginResponse{
		AccessToken: res.AccessToken, RefreshToken: res.RefreshToken, User: &user,
		MFASetupDiwajibkan: res.MFASetupRequired,
	}
}

// POST /v1/auth/refresh
// Returns new tokens in response body for the BFF to set as HttpOnly cookies.
func (h *Handler) Refresh(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	ip := c.ClientIP()
	result, err := h.uc.RefreshSession(audit.RequestContext(c), req.RefreshToken, ip)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidToken):
			response.Unauthorized(c, "Token tidak valid")
		case errors.Is(err, usecase.ErrTokenExpired):
			response.Unauthorized(c, "Token kedaluwarsa")
		default:
			response.InternalError(c, "Kesalahan server")
		}
		return
	}

	// RefreshToken kosong pada pemutaran ulang dalam jendela toleransi: klien mempertahankan token dari
	// permintaan pemenang.
	resp := dto.RefreshTokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}
	response.Success(c, resp)
}

// POST /v1/auth/logout
// Mengakhiri hanya sesi yang sedang dipakai (sid access token). Isi opsional {"refresh_token"} dipakai
// sebagai cadangan untuk token tanpa sid. Keluar dari semua perangkat: POST /v1/auth/logout-all.
func (h *Handler) Logout(c *gin.Context) {
	userID := c.GetString("user_id")
	jti := c.GetString("jti")
	var req dto.LogoutRequest
	// Isi opsional: badan kosong atau bukan JSON tidak menggagalkan logout.
	_ = c.ShouldBindJSON(&req)
	if err := h.uc.LogoutSession(requestContext(c), userID, jti, req.RefreshToken); err != nil {
		response.InternalError(c, "Gagal logout")
		return
	}
	response.Success(c, nil)
}

// GET /v1/auth/me
func (h *Handler) Me(c *gin.Context) {
	userID := c.GetString("user_id")
	user, err := h.uc.GetMe(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "Pengguna tidak ditemukan")
		return
	}
	status, err := h.uc.MFAStatus(c.Request.Context(), user)
	if err != nil {
		response.InternalError(c, "Gagal mengambil status MFA")
		return
	}

	response.Success(c, dto.MeUserResponse{
		UserResponse:        dto.ToUserResponse(user),
		MfaSetupDiwajibkan:  status.SetupDiwajibkan,
		MfaWajib:            status.Wajib,
		KodeCadanganTersisa: status.KodeCadanganTersisa,
	})
}

// POST /v1/auth/change-password
func (h *Handler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	userID := c.GetString("user_id")
	ip := c.ClientIP()

	accessToken, refreshToken, err := h.uc.ChangePassword(requestContext(c), userID, req.OldPassword, req.NewPassword, ip)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrAccountLocked):
			jawabAkunTerkunci(c, err)
		case errors.Is(err, usecase.ErrWrongPassword):
			response.Unauthorized(c, "Password lama salah")
		case errors.Is(err, usecase.ErrWeakPassword):
			response.BadRequest(c, err.Error())
		case errors.Is(err, usecase.ErrPasswordReuse):
			response.BadRequest(c, "Password baru tidak boleh sama dengan password lama")
		default:
			response.InternalError(c, "Gagal mengubah password")
		}
		return
	}

	response.Success(c, dto.ChangePasswordResponse{AccessToken: accessToken, RefreshToken: refreshToken})
}

// POST /v1/auth/forgot-password
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if err := h.uc.ForgotPassword(audit.RequestContext(c), req.Email); err != nil {
		// Always return success to prevent email enumeration
		response.Success(c, nil)
		return
	}

	response.Success(c, nil)
}

// POST /v1/auth/reset-password
func (h *Handler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if err := h.uc.ResetPassword(audit.RequestContext(c), req.Token, req.Password); err != nil {
		switch {
		case errors.Is(err, usecase.ErrResetTokenExpired):
			response.Unauthorized(c, "Tautan atur ulang sudah kedaluwarsa. Minta tautan baru.")
		case errors.Is(err, usecase.ErrResetTokenInvalid), errors.Is(err, usecase.ErrUserNotFound):
			response.Unauthorized(c, "Tautan atur ulang tidak valid atau sudah dipakai. Minta tautan baru.")
		case errors.Is(err, usecase.ErrWeakPassword):
			response.BadRequest(c, err.Error())
		case errors.Is(err, usecase.ErrPasswordReuse):
			response.BadRequest(c, "Password baru tidak boleh sama dengan password lama")
		default:
			response.InternalError(c, "Gagal mereset password")
		}
		return
	}

	response.Success(c, nil)
}

// POST /v1/users
func (h *Handler) CreateUser(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	user := &domain.Pengguna{
		ID:      req.ID,
		Nama:    req.Nama,
		Email:   req.Email,
		Peran:   req.Peran,
		Lapis:   req.Lapis,
		Lembaga: req.Lembaga,
	}

	if err := h.uc.CreateUser(audit.RequestContext(c), user, req.Password, c.GetString("user_id")); err != nil {
		switch {
		case errors.Is(err, usecase.ErrEmailExists):
			response.Conflict(c, "Email sudah terdaftar")
		case errors.Is(err, usecase.ErrWeakPassword):
			// Masukan pengguna yang salah, bukan galat server (usecase membungkusnya dengan alasan).
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "Gagal membuat pengguna")
		}
		return
	}

	response.Created(c, nil)
}

// GET /v1/users
func (h *Handler) ListUsers(c *gin.Context) {
	var req dto.ListUserRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 100 {
		req.Limit = 20
	}

	users, total, err := h.uc.ListUsers(c.Request.Context(), req.Page, req.Limit, req.Search)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data")
		return
	}

	items, err := h.penggunaDenganKunci(c, users)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data")
		return
	}

	response.Success(c, dto.ListResponse{
		Items: items,
		Total: total,
		Page:  req.Page,
		Limit: req.Limit,
	})
}

// GET /v1/users/:id
func (h *Handler) GetUser(c *gin.Context) {
	id := c.Param("id")
	user, err := h.uc.GetUser(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrUserNotFound) {
			response.NotFound(c, "Pengguna tidak ditemukan")
			return
		}
		response.InternalError(c, "Gagal mengambil pengguna")
		return
	}

	item, err := h.penggunaDenganKunci(c, []domain.Pengguna{*user})
	if err != nil {
		response.InternalError(c, "Gagal mengambil pengguna")
		return
	}

	response.Success(c, item[0])
}

// PATCH /v1/users/:id
func (h *Handler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	actorID := c.GetString("user_id")

	user, err := h.uc.UpdateUser(audit.RequestContext(c), id, usecase.UserPatch{
		Nama:          req.Nama,
		Email:         req.Email,
		Peran:         req.Peran,
		Lapis:         req.Lapis,
		Lembaga:       req.Lembaga,
		KabupatenKode: req.KabupatenKode,
		Status:        req.Status,
	}, actorID)
	if err != nil {
		switch err {
		case usecase.ErrUserNotFound:
			response.NotFound(c, "Pengguna tidak ditemukan")
		case usecase.ErrEmailExists:
			response.Conflict(c, "Email sudah terdaftar")
		case usecase.ErrRoleNotFound:
			response.BadRequest(c, "Peran tidak dikenal")
		default:
			response.InternalError(c, "Gagal memperbarui pengguna")
		}
		return
	}

	response.Success(c, dto.ToUserResponse(user))
}

// DELETE /v1/users/:id
func (h *Handler) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	actorID := c.GetString("user_id")

	if err := h.uc.DeleteUser(audit.RequestContext(c), id, actorID); err != nil {
		switch err {
		case usecase.ErrUserNotFound:
			response.NotFound(c, "Pengguna tidak ditemukan")
		case usecase.ErrCannotDeleteSelf:
			response.BadRequest(c, "Tidak dapat menghapus akun sendiri")
		case usecase.ErrUserHasReferences:
			response.Conflict(c, "Pengguna masih memiliki data terkait (tugas, entri harga, atau audit). Nonaktifkan akun sebagai gantinya.")
		default:
			response.InternalError(c, "Gagal menghapus pengguna")
		}
		return
	}

	response.Success(c, gin.H{"id": id})
}

// POST /v1/users/:id/reset-password
func (h *Handler) ResetPasswordAdmin(c *gin.Context) {
	id := c.Param("id")

	var req dto.ResetPasswordAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	actorID := c.GetString("user_id")

	if err := h.uc.ResetPasswordAdmin(audit.RequestContext(c), id, req.NewPassword, actorID); err != nil {
		switch err {
		case usecase.ErrUserNotFound:
			response.NotFound(c, "Pengguna tidak ditemukan")
		case usecase.ErrWeakPassword:
			response.BadRequest(c, err.Error())
		default:
			if errors.Is(err, usecase.ErrWeakPassword) {
				response.BadRequest(c, err.Error())
				return
			}
			response.InternalError(c, "Gagal mengatur ulang kata sandi")
		}
		return
	}

	response.Success(c, gin.H{"id": id})
}

// PUT /v1/users/:id/roles
func (h *Handler) AssignRole(c *gin.Context) {
	id := c.Param("id")

	var req dto.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	actorID := c.GetString("user_id")

	user, err := h.uc.AssignRole(audit.RequestContext(c), id, req.Peran, actorID)
	if err != nil {
		switch err {
		case usecase.ErrUserNotFound:
			response.NotFound(c, "Pengguna tidak ditemukan")
		case usecase.ErrRoleNotFound:
			response.BadRequest(c, "Peran tidak dikenal")
		default:
			response.InternalError(c, "Gagal mengubah peran")
		}
		return
	}

	response.Success(c, dto.ToUserResponse(user))
}

// GET /v1/roles
func (h *Handler) ListRoles(c *gin.Context) {
	roles := h.uc.ListRoles()
	items := make([]dto.RoleResponse, len(roles))
	for i, r := range roles {
		items[i] = dto.RoleResponse{ID: r.ID, Permissions: r.Permissions}
	}
	response.Success(c, items)
}

// GET /v1/permissions
func (h *Handler) ListPermissions(c *gin.Context) {
	perms := h.uc.ListPermissions()
	items := make([]dto.PermissionResponse, len(perms))
	for i, p := range perms {
		items[i] = dto.PermissionResponse{ID: p}
	}
	response.Success(c, items)
}

// POST /v1/users/bulk
func (h *Handler) BulkUser(c *gin.Context) {
	var req dto.BulkUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	count, err := h.uc.BulkUserAction(audit.RequestContext(c), req.IDs, req.Action, c.GetString("user_id"))
	if err != nil {
		// Hanya aksi tak dikenal yang salah masukan; galat basis data tidak boleh bocor ke klien sebagai 400.
		if errors.Is(err, usecase.ErrInvalidAction) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "Gagal memproses aksi massal")
		return
	}

	response.Success(c, dto.BulkUserResponse{
		Berhasil: int(count),
	})
}

// ── MFA Handlers ──────────────────────────────────────────────────────

// mfaError memetakan galat MFA ke respons; true bila galat sudah dijawab.
func mfaError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, usecase.ErrMFAChallengeInvalid):
		response.Error(c, http.StatusUnauthorized, dto.KodeMFASesiBerakhir, err.Error())
	case errors.Is(err, usecase.ErrAccountLocked):
		jawabAkunTerkunci(c, err)
	case errors.Is(err, usecase.ErrMFACodeInvalid):
		response.Error(c, http.StatusBadRequest, dto.KodeMFAKodeSalah, dto.PesanMFAKodeSalah)
	case errors.Is(err, usecase.ErrMFANotSetup):
		response.BadRequest(c, err.Error())
	case errors.Is(err, usecase.ErrMFAAlreadyActive):
		response.Conflict(c, err.Error())
	case errors.Is(err, usecase.ErrMFAMandatory):
		response.Forbidden(c, err.Error())
	case errors.Is(err, usecase.ErrWrongPassword):
		response.Unauthorized(c, "Password salah")
	case errors.Is(err, usecase.ErrMFAUnavailable):
		response.Error(c, http.StatusServiceUnavailable, "503", err.Error())
	default:
		response.InternalError(c, "Kesalahan server")
	}
	return true
}

// POST /v1/auth/mfa/setup
// Menyiapkan rahasia TOTP (belum aktif sampai /mfa/verify). Terbuka juga bagi akun yang wajib setup MFA.
func (h *Handler) SetupMFA(c *gin.Context) {
	setup, err := h.uc.SetupMFA(audit.RequestContext(c), c.GetString("user_id"))
	if mfaError(c, err) {
		return
	}
	svg, err := qr.SVG(setup.ProvisioningURI)
	if err != nil {
		response.InternalError(c, "Gagal membuat kode QR")
		return
	}

	response.Success(c, dto.MFASetupResponse{Secret: setup.Secret, ProvisioningURI: setup.ProvisioningURI, QRSVG: svg})
}

// POST /v1/auth/mfa/verify
// Memverifikasi kode pertama, mengaktifkan MFA, dan menerbitkan sesi baru beserta kode cadangan (sekali tampil).
func (h *Handler) VerifyMFA(c *gin.Context) {
	var req dto.MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	res, err := h.uc.VerifyMFA(requestContext(c), c.GetString("user_id"), req.Code, c.ClientIP())
	if mfaError(c, err) {
		return
	}

	response.Success(c, dto.MFAVerifyResponse{AccessToken: res.AccessToken, RefreshToken: res.RefreshToken, KodeCadangan: res.KodeCadangan})
}

// POST /v1/auth/mfa/validate
// Publik: menukar mfa_token dari login dan kode OTP atau kode cadangan dengan sesi.
func (h *Handler) ValidateMFA(c *gin.Context) {
	var req dto.MFAValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	res, err := h.uc.ValidateMFALogin(audit.RequestContext(c), req.MFAToken, req.Code, c.ClientIP())
	if err != nil {
		if errors.Is(err, usecase.ErrMFACodeInvalid) {
			response.Error(c, http.StatusUnauthorized, dto.KodeMFAKodeSalah, dto.PesanMFAKodeSalah)
			return
		}
		mfaError(c, err)
		return
	}

	response.Success(c, loginResponse(res))
}

// POST /v1/auth/mfa/backup-codes
// Menerbitkan kode cadangan baru (yang lama hangus) setelah konfirmasi kata sandi; sekali tampil.
func (h *Handler) RegenerateBackupCodes(c *gin.Context) {
	var req dto.MFARegenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	kode, err := h.uc.RegenerateBackupCodes(audit.RequestContext(c), c.GetString("user_id"), req.Password)
	if mfaError(c, err) {
		return
	}

	response.Success(c, dto.MFABackupCodesResponse{KodeCadangan: kode})
}

// POST /v1/auth/mfa/disable
func (h *Handler) DisableMFA(c *gin.Context) {
	var req dto.MFADisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	if mfaError(c, h.uc.DisableMFA(audit.RequestContext(c), c.GetString("user_id"), req.Password)) {
		return
	}

	response.Success(c, nil)
}
