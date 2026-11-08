package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/pkg/token"
)

func passwordChangeRouter(tokenMgr *token.Manager) *gin.Engine {
	router := gin.New()
	v1 := router.Group("/v1")
	v1.Use(JWTAuth(tokenMgr))
	for _, path := range []string{"/auth/me", "/auth/logout", "/auth/change-password", "/harga", "/users"} {
		v1.Any(path, func(c *gin.Context) { c.Status(http.StatusOK) })
	}
	return router
}

func TestJWTAuth_PasswordChangeRequired_OnlyExemptRoutesOpen(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	router := passwordChangeRouter(tokenMgr)

	cases := []struct {
		path string
		want int
	}{
		{"/v1/auth/me", http.StatusOK},
		{"/v1/auth/logout", http.StatusOK},
		{"/v1/auth/change-password", http.StatusOK},
		{"/v1/harga", http.StatusForbidden},
		{"/v1/users", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			tokenStr, err := tokenMgr.CreateAccessToken(uid("user-1"), "ADMIN_SISTEM", []string{"pengguna.kelola"}, token.WithPasswordChangeRequired(true))
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.want, w.Code)
			if tc.want == http.StatusForbidden {
				var body struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, PasswordChangeRequiredCode, body.Code)
			}
		})
	}
}

func TestJWTAuth_NoPasswordChangeRequired_AllRoutesOpen(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	router := passwordChangeRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("user-1"), "ADMIN_SISTEM", nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/v1/harga", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJWTAuth_MFASetupRequired_OnlySetupRoutesOpen(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	router := gin.New()
	v1 := router.Group("/v1")
	v1.Use(JWTAuth(tokenMgr))
	for _, path := range []string{"/auth/me", "/auth/logout", "/auth/change-password", "/auth/mfa/setup", "/auth/mfa/verify", "/auth/mfa/disable", "/harga"} {
		v1.Any(path, func(c *gin.Context) { c.Status(http.StatusOK) })
	}

	cases := map[string]int{
		"/v1/auth/mfa/setup":       http.StatusOK,
		"/v1/auth/mfa/verify":      http.StatusOK,
		"/v1/auth/me":              http.StatusOK,
		"/v1/auth/logout":          http.StatusOK,
		"/v1/auth/change-password": http.StatusOK,
		"/v1/auth/mfa/disable":     http.StatusForbidden,
		"/v1/harga":                http.StatusForbidden,
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			tokenStr, err := tokenMgr.CreateAccessToken(uid("user-1"), "ADMIN_SISTEM", []string{"harga.lihat"}, token.WithMFASetupRequired(true))
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, want, w.Code)
			if want == http.StatusForbidden {
				var body struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, MFASetupRequiredCode, body.Code)
			}
		})
	}
}

func TestJWTAuth_RejectsMFAChallengeToken(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	router := passwordChangeRouter(tokenMgr)
	challenge, err := tokenMgr.CreateMFAChallengeToken(uid("user-1"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+challenge)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "token tantangan MFA bukan kredensial sesi")
}

// Profil sendiri terbuka bagi sesi terbatas hanya untuk dibaca: PATCH /v1/auth/me (20-A) menunggu kewajiban selesai.
func TestJWTAuth_SesiTerbatas_ProfilSendiriHanyaDapatDibaca(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	router := passwordChangeRouter(tokenMgr)

	cases := []struct {
		name     string
		opsi     []token.ClaimOption
		method   string
		want     int
		wantCode string
	}{
		{"wajib ganti sandi: GET dibaca", []token.ClaimOption{token.WithPasswordChangeRequired(true)}, http.MethodGet, http.StatusOK, ""},
		{"wajib ganti sandi: PATCH ditolak", []token.ClaimOption{token.WithPasswordChangeRequired(true)}, http.MethodPatch, http.StatusForbidden, PasswordChangeRequiredCode},
		{"wajib setup MFA: GET dibaca", []token.ClaimOption{token.WithMFASetupRequired(true)}, http.MethodGet, http.StatusOK, ""},
		{"wajib setup MFA: PATCH ditolak", []token.ClaimOption{token.WithMFASetupRequired(true)}, http.MethodPatch, http.StatusForbidden, MFASetupRequiredCode},
		{"sesi biasa: PATCH boleh", nil, http.MethodPatch, http.StatusOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokenStr, err := tokenMgr.CreateAccessToken(uid("user-1"), "OPS_MAHASISWA", []string{"harga.lihat"}, tc.opsi...)
			require.NoError(t, err)

			req := httptest.NewRequest(tc.method, "/v1/auth/me", nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.want, w.Code)
			if tc.wantCode != "" {
				var body struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tc.wantCode, body.Code)
			}
		})
	}
}
