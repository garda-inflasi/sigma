package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/auth/dto"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/response"
	"github.com/gin-gonic/gin"
)

// Kode galat SSO pada amplop respons. BFF memetakannya ke /masuk?error=<kode> (lib/auth/sso.ts).
const (
	kodeSSOTidakAktif        = "SSO_NOT_CONFIGURED"
	kodeSSOTidakTersedia     = "SSO_UNAVAILABLE"
	kodeSSOStateTidakValid   = "SSO_STATE_INVALID"
	kodeSSOVerifikasiGagal   = "SSO_EXCHANGE_FAILED"
	kodeSSOEmailBelumValid   = "SSO_EMAIL_UNVERIFIED"
	kodeSSODomainDitolak     = "SSO_DOMAIN_NOT_ALLOWED"
	kodeSSOAkunTidakAda      = "SSO_ACCOUNT_NOT_FOUND"
	kodeSSOAkunBentrok       = "SSO_ACCOUNT_CONFLICT"
	kodeSSOAkunTidakAktif    = "SSO_ACCOUNT_INACTIVE"
	kodeSSOAkunTerkunci      = "SSO_ACCOUNT_LOCKED"
	kodeSSOKesalahanInternal = "500"
)

// SSOHandler melayani rute SSO publik. Seluruhnya dipanggil BFF dari server ke server.
type SSOHandler struct {
	uc *usecase.SSOUseCase
}

func NewSSO(uc *usecase.SSOUseCase) *SSOHandler {
	return &SSOHandler{uc: uc}
}

// GET /v1/auth/sso/status
// Publik: penyedia SSO yang aktif, supaya halaman masuk hanya menampilkan tombol yang berfungsi.
func (h *SSOHandler) Status(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	response.Success(c, dto.SSOStatusResponse{Google: h.uc.Enabled()})
}

// POST /v1/auth/sso/google/authorize
// Memulai alur dan mengembalikan URL otorisasi Google beserta binding untuk cookie browser.
func (h *SSOHandler) Authorize(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	res, err := h.uc.Authorize(audit.RequestContext(c))
	if err != nil {
		ssoError(c, err)
		return
	}
	response.Success(c, dto.SSOAuthorizeResponse{AuthorizeURL: res.URL, Binding: res.Binding})
}

// POST /v1/auth/sso/google/callback
// Menukar kode otorisasi dan memetakan identitas ke akun yang sudah ada. Hasilnya sama dengan login:
// sesi penuh, atau mfa_required dan mfa_token untuk akun ber-MFA.
func (h *SSOHandler) Callback(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req dto.SSOCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Validasi gagal: "+err.Error())
		return
	}

	res, err := h.uc.Callback(audit.RequestContext(c), usecase.SSOCallbackInput{
		Code: req.Code, State: req.State, Binding: req.Binding, IP: c.ClientIP(),
	})
	if err != nil {
		ssoError(c, err)
		return
	}
	response.Success(c, loginResponse(res))
}

// ssoError menjawab galat usecase SSO dengan status dan kode yang dipetakan BFF. Pesan tidak memuat
// detail internal penyedia.
func ssoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, usecase.ErrSSODisabled):
		response.Error(c, http.StatusServiceUnavailable, kodeSSOTidakAktif, "Masuk dengan Google belum dikonfigurasi")
	case errors.Is(err, usecase.ErrSSOUnavailable):
		response.Error(c, http.StatusServiceUnavailable, kodeSSOTidakTersedia, "Penyedia SSO tidak dapat dihubungi, coba lagi nanti")
	case errors.Is(err, usecase.ErrSSOStateInvalid):
		response.Error(c, http.StatusBadRequest, kodeSSOStateTidakValid, "Sesi masuk SSO tidak valid atau sudah kedaluwarsa, mulai lagi")
	case errors.Is(err, usecase.ErrSSOExchangeFailed):
		response.Error(c, http.StatusUnauthorized, kodeSSOVerifikasiGagal, "Verifikasi identitas gagal")
	case errors.Is(err, usecase.ErrSSOEmailUnverified):
		response.Error(c, http.StatusForbidden, kodeSSOEmailBelumValid, "Email akun belum terverifikasi di penyedia SSO")
	case errors.Is(err, usecase.ErrSSODomainNotAllowed):
		response.Error(c, http.StatusForbidden, kodeSSODomainDitolak, "Domain akun tidak diizinkan")
	case errors.Is(err, usecase.ErrSSOAccountNotFound):
		response.Error(c, http.StatusForbidden, kodeSSOAkunTidakAda, "Tidak ada akun untuk identitas ini, hubungi administrator")
	case errors.Is(err, usecase.ErrSSOAccountConflict):
		response.Error(c, http.StatusConflict, kodeSSOAkunBentrok, "Akun sudah tertaut ke identitas SSO lain, hubungi administrator")
	case errors.Is(err, usecase.ErrAccountInactive):
		response.Error(c, http.StatusForbidden, kodeSSOAkunTidakAktif, "Akun tidak aktif, hubungi administrator")
	case errors.Is(err, usecase.ErrAccountLocked):
		var terkunci *usecase.AkunTerkunciError
		if errors.As(err, &terkunci) {
			c.Header("Retry-After", strconv.Itoa(terkunci.SisaDetik))
		}
		response.Error(c, http.StatusLocked, kodeSSOAkunTerkunci, "Akun terkunci karena terlalu banyak percobaan gagal, coba lagi nanti")
	default:
		_ = c.Error(err)
		response.Error(c, http.StatusInternalServerError, kodeSSOKesalahanInternal, "Kesalahan server")
	}
}
