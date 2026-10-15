package http

import (
	"net/http"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
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
// Returns user profile only. Tokens are returned in response body for the BFF to set as HttpOnly cookies.
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	ip := c.ClientIP()
	user, accessToken, refreshToken, err := h.uc.Login(c.Request.Context(), req.Email, req.Password, ip)
	if err != nil {
		switch err {
		case usecase.ErrInvalidCredentials:
			response.Unauthorized(c, "Email atau password salah")
		case usecase.ErrAccountLocked:
			response.Error(c, http.StatusTooManyRequests, "429", "Akun terkunci, coba lagi nanti")
		case usecase.ErrAccountInactive:
			response.Forbidden(c, "Akun tidak aktif, hubungi administrator")
		default:
			response.InternalError(c, "Kesalahan server")
		}
		return
	}

	resp := dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: dto.UserResponse{
			ID:            user.ID,
			Nama:          user.Nama,
			Email:         user.Email,
			Peran:         user.Peran,
			Lapis:         user.Lapis,
			Lembaga:       user.Lembaga,
			KabupatenKode: user.KabupatenKode,
			MfaAktif:      user.MfaAktif,
			Status:        user.Status,
			TerakhirMasuk: user.TerakhirMasuk,
			DibuatPada:    user.DibuatPada,
		},
	}

	response.Success(c, resp)
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
	accessToken, refreshToken, err := h.uc.Refresh(c.Request.Context(), req.RefreshToken, ip)
	if err != nil {
		switch err {
		case usecase.ErrInvalidToken:
			response.Unauthorized(c, "Token tidak valid")
		case usecase.ErrTokenExpired:
			response.Unauthorized(c, "Token kedaluwarsa")
		default:
			response.InternalError(c, "Kesalahan server")
		}
		return
	}

	resp := dto.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
	response.Success(c, resp)
}

// POST /v1/auth/logout
func (h *Handler) Logout(c *gin.Context) {
	userID := c.GetString("user_id")
	jti := c.GetString("jti")
	ip := c.ClientIP()
	if err := h.uc.Logout(c.Request.Context(), userID, jti, ip); err != nil {
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

	resp := dto.UserResponse{
		ID:            user.ID,
		Nama:          user.Nama,
		Email:         user.Email,
		Peran:         user.Peran,
		Lapis:         user.Lapis,
		Lembaga:       user.Lembaga,
		KabupatenKode: user.KabupatenKode,
		MfaAktif:      user.MfaAktif,
		Status:        user.Status,
		TerakhirMasuk: user.TerakhirMasuk,
		DibuatPada:    user.DibuatPada,
	}
	response.Success(c, resp)
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

	if err := h.uc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword, ip); err != nil {
		switch err {
		case usecase.ErrWrongPassword:
			response.Unauthorized(c, "Password lama salah")
		case usecase.ErrWeakPassword:
			response.BadRequest(c, "Password baru terlalu lemah")
		case usecase.ErrPasswordReuse:
			response.BadRequest(c, "Password baru tidak boleh sama dengan password lama")
		default:
			response.InternalError(c, "Gagal mengubah password")
		}
		return
	}

	response.Success(c, nil)
}

// POST /v1/auth/forgot-password
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	ip := c.ClientIP()
	if err := h.uc.ForgotPassword(c.Request.Context(), req.Email, ip); err != nil {
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

	ip := c.ClientIP()
	if err := h.uc.ResetPassword(c.Request.Context(), req.Token, req.Password, ip); err != nil {
		switch err {
		case usecase.ErrResetTokenInvalid, usecase.ErrResetTokenExpired:
			response.Unauthorized(c, "Token reset tidak valid atau sudah kedaluwarsa")
		case usecase.ErrWeakPassword:
			response.BadRequest(c, "Password terlalu lemah")
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

	if err := h.uc.CreateUser(c.Request.Context(), user, req.Password); err != nil {
		switch err {
		case usecase.ErrEmailExists:
			response.Conflict(c, "Email sudah terdaftar")
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

	users, total, err := h.uc.ListUsers(c.Request.Context(), req.Page, req.Limit)
	if err != nil {
		response.InternalError(c, "Gagal mengambil data")
		return
	}

	items := make([]dto.UserResponse, len(users))
	for i, u := range users {
		items[i] = dto.UserResponse{
			ID:            u.ID,
			Nama:          u.Nama,
			Email:         u.Email,
			Peran:         u.Peran,
			Lapis:         u.Lapis,
			Lembaga:       u.Lembaga,
			KabupatenKode: u.KabupatenKode,
			MfaAktif:      u.MfaAktif,
			Status:        u.Status,
			TerakhirMasuk: u.TerakhirMasuk,
			DibuatPada:    u.DibuatPada,
		}
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
		response.NotFound(c, "Pengguna tidak ditemukan")
		return
	}

	resp := dto.UserResponse{
		ID:            user.ID,
		Nama:          user.Nama,
		Email:         user.Email,
		Peran:         user.Peran,
		Lapis:         user.Lapis,
		Lembaga:       user.Lembaga,
		KabupatenKode: user.KabupatenKode,
		MfaAktif:      user.MfaAktif,
		Status:        user.Status,
		TerakhirMasuk: user.TerakhirMasuk,
		DibuatPada:    user.DibuatPada,
	}
	response.Success(c, resp)
}

// POST /v1/users/bulk
func (h *Handler) BulkUser(c *gin.Context) {
	var req dto.BulkUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	ip := c.ClientIP()
	count, err := h.uc.BulkUserAction(c.Request.Context(), req.IDs, req.Action, ip)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, dto.BulkUserResponse{
		Berhasil: int(count),
	})
}

// ── MFA Handlers ──────────────────────────────────────────────────────

// POST /v1/auth/mfa/setup
func (h *Handler) SetupMFA(c *gin.Context) {
	userID := c.GetString("user_id")

	secret, uri, backupCodes, err := h.uc.SetupMFA(c.Request.Context(), userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, dto.MFASetupResponse{
		Secret:          secret,
		ProvisioningURI: uri,
		BackupCodes:     backupCodes,
	})
}

// POST /v1/auth/mfa/verify
func (h *Handler) VerifyMFA(c *gin.Context) {
	var req dto.MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	userID := c.GetString("user_id")
	if err := h.uc.VerifyMFA(c.Request.Context(), userID, req.Code); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// POST /v1/auth/mfa/validate
func (h *Handler) ValidateMFA(c *gin.Context) {
	var req dto.MFAValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	userID := c.GetString("user_id")
	valid, err := h.uc.ValidateMFACode(c.Request.Context(), userID, req.Code)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if !valid {
		response.Unauthorized(c, "Kode MFA tidak valid")
		return
	}

	response.Success(c, nil)
}

// POST /v1/auth/mfa/disable
func (h *Handler) DisableMFA(c *gin.Context) {
	var req dto.MFADisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	userID := c.GetString("user_id")
	if err := h.uc.DisableMFA(c.Request.Context(), userID, req.Password); err != nil {
		switch err {
		case usecase.ErrWrongPassword:
			response.Unauthorized(c, "Password salah")
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}

	response.Success(c, nil)
}
