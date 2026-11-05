package integration

import (
	"net/http"
	"testing"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	adminEmail       = "admin.awal@gardainflasi.id"
	adminTempSandi   = "Sementara1!"
	adminNewPassword = "SandiBaru#2026"
)

func seedAdminAwal(t *testing.T, db *gorm.DB) {
	t.Helper()
	sum, err := authseeder.SeedAdmin(t.Context(), db, authseeder.AdminConfig{Email: adminEmail, Password: adminTempSandi})
	require.NoError(t, err)
	require.Equal(t, 1, sum.Dibuat)
}

// TestIntegration_AdminSeed_MandatoryPasswordChange mengikuti alur 01-A:
// admin seed -> login -> 403 PASSWORD_CHANGE_REQUIRED -> ganti password -> normal.
func TestIntegration_AdminSeed_MandatoryPasswordChange(t *testing.T) {
	s := setupSuiteWith(t, seedAdminAwal)

	accessToken, _ := s.loginAs(t, adminEmail, adminTempSandi)

	t.Run("login menandai wajib ganti password", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(accessToken))
		require.Equal(t, http.StatusOK, w.Code)
		data := s.parseResponse(t, w)["data"].(map[string]interface{})
		assert.Equal(t, true, data["wajib_ganti_password"])
	})

	t.Run("endpoint lain dijawab 403 PASSWORD_CHANGE_REQUIRED", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(accessToken))
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, middleware.PasswordChangeRequiredCode, s.parseResponse(t, w)["code"])
	})

	var newAccess string
	t.Run("ganti password mematikan penanda dan menerbitkan sesi baru", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/change-password", map[string]interface{}{
			"old_password": adminTempSandi,
			"new_password": adminNewPassword,
		}, s.authHeader(accessToken))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		data := s.parseResponse(t, w)["data"].(map[string]interface{})
		newAccess, _ = data["access_token"].(string)
		require.NotEmpty(t, newAccess)
		assert.NotEmpty(t, data["refresh_token"])
	})

	t.Run("akses normal dengan token baru", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(newAccess))
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

		me := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(newAccess))
		data := s.parseResponse(t, me)["data"].(map[string]interface{})
		assert.Equal(t, false, data["wajib_ganti_password"])
	})

	t.Run("login ulang dengan password baru tidak lagi wajib ganti", func(t *testing.T) {
		token, _ := s.loginAs(t, adminEmail, adminNewPassword)
		w := s.serveRequest(t, http.MethodGet, "/v1/users", nil, s.authHeader(token))
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestIntegration_EmailIsCaseInsensitive(t *testing.T) {
	s := setupSuite(t)

	accessToken, _ := s.loginAs(t, "TEST@GardaInflasi.ID", "Test1234!")
	assert.NotEmpty(t, accessToken)

	err := s.db.Exec(`
		INSERT INTO pengguna (id, nama, email, password_hash, peran, lapis, lembaga)
		VALUES ('dup', 'Dup', 'Test@gardainflasi.id', 'x', 'OPS_MAHASISWA', 'OPERASIONAL', 'X')`).Error
	require.Error(t, err, "email ganda berbeda huruf harus ditolak indeks unik")
}
