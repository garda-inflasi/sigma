package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/response"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

// POST /v1/auth/login
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
	if err := h.uc.Logout(c.Request.Context(), userID); err != nil {
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


