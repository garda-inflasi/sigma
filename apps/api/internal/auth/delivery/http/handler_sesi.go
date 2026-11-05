package http

import (
	"context"
	"errors"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// requestContext adalah audit.RequestContext ditambah id sesi (klaim sid) access token yang dipakai, supaya
// usecase tahu sesi mana yang "sedang dipakai" tanpa mengubah tanda tangannya.
func requestContext(c *gin.Context) context.Context {
	return usecase.WithSession(audit.RequestContext(c), middleware.SessionID(c))
}

// POST /v1/auth/logout-all
// Mengakhiri semua sesi pengguna ("keluar dari semua perangkat"), termasuk yang sedang dipakai.
func (h *Handler) LogoutAll(c *gin.Context) {
	n, err := h.uc.LogoutAll(requestContext(c), c.GetString("user_id"), c.GetString("jti"))
	if err != nil {
		response.InternalError(c, "Gagal keluar dari semua perangkat")
		return
	}
	response.Success(c, dto.SessionsRevokedResponse{Dicabut: n})
}

// GET /v1/auth/sessions
// Sesi aktif milik pengguna yang sedang masuk; saat_ini menandai sesi pemanggil.
func (h *Handler) ListSessions(c *gin.Context) {
	items, err := h.uc.ListSessions(requestContext(c), c.GetString("user_id"))
	if err != nil {
		response.InternalError(c, "Gagal mengambil daftar sesi")
		return
	}
	out := make([]dto.SessionResponse, len(items))
	for i, s := range items {
		out[i] = dto.SessionResponse{
			ID:                  s.ID,
			Perangkat:           dto.RingkasPerangkat(s.UserAgent),
			UserAgent:           s.UserAgent,
			IP:                  s.IP,
			DibuatPada:          s.StartedAt,
			TerakhirDipakaiPada: s.LastUsedAt,
			KedaluwarsaPada:     s.ExpiresAt,
			SaatIni:             s.Current,
		}
	}
	response.Success(c, out)
}

// DELETE /v1/auth/sessions/:id
// Mencabut satu sesi milik pengguna sendiri selain yang sedang dipakai.
func (h *Handler) RevokeSession(c *gin.Context) {
	id := c.Param("id")
	err := h.uc.RevokeSession(requestContext(c), c.GetString("user_id"), id)
	switch {
	case err == nil:
		response.Success(c, gin.H{"id": id})
	case errors.Is(err, usecase.ErrSessionNotFound):
		response.NotFound(c, "Sesi tidak ditemukan")
	case errors.Is(err, usecase.ErrCannotRevokeCurrent):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "Gagal mencabut sesi")
	}
}

// POST /v1/auth/sessions/revoke-others
// Mencabut semua sesi pengguna selain yang sedang dipakai.
func (h *Handler) RevokeOtherSessions(c *gin.Context) {
	n, err := h.uc.RevokeOtherSessions(requestContext(c), c.GetString("user_id"))
	switch {
	case err == nil:
		response.Success(c, dto.SessionsRevokedResponse{Dicabut: n})
	case errors.Is(err, usecase.ErrCurrentSessionUnknown):
		response.Conflict(c, err.Error())
	default:
		response.InternalError(c, "Gagal mencabut sesi lain")
	}
}

// POST /v1/users/:id/revoke-sessions
// Admin (pengguna.kelola) memaksa semua sesi seorang pengguna berakhir.
func (h *Handler) ForceLogoutUser(c *gin.Context) {
	id := c.Param("id")
	n, err := h.uc.ForceLogoutUser(requestContext(c), id, c.GetString("user_id"))
	switch {
	case err == nil:
		response.Success(c, dto.ForceLogoutResponse{ID: id, Dicabut: n})
	case errors.Is(err, usecase.ErrUserNotFound):
		response.NotFound(c, "Pengguna tidak ditemukan")
	default:
		response.InternalError(c, "Gagal memaksa keluar pengguna")
	}
}
