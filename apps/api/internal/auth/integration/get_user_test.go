package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_GetUser(t *testing.T) {
	s := setupSuite(t)
	admin, _ := s.loginAs(t, emailUji, passwordLama)
	biasa, _ := s.loginAs(t, "uji.biasa@gardainflasi.id", "Test1234!")

	t.Run("admin membaca pengguna tanpa rahasia", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/users/test-user-001", nil, s.authHeader(admin))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		data, ok := s.parseResponse(t, w)["data"].(map[string]any)
		require.True(t, ok, w.Body.String())
		assert.Equal(t, "test-user-001", data["id"])
		assert.Equal(t, "Test User", data["nama"])
		assert.Equal(t, emailUji, data["email"])
		assert.Equal(t, "ADMIN_SISTEM", data["peran"])
		assert.Equal(t, "1171", data["kabupaten_kode"])
		assert.NotContains(t, w.Body.String(), "password_hash")
		assert.NotContains(t, w.Body.String(), "mfa_secret")
	})

	t.Run("id tidak ada dijawab 404", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/users/tidak-ada", nil, s.authHeader(admin))
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("tanpa pengguna.kelola dijawab 403", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/users/test-user-001", nil, s.authHeader(biasa))
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("tanpa token dijawab 401", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/users/test-user-001", nil, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
