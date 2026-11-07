package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	emailUji     = "test@gardainflasi.id"
	passwordLama = "Test1234!"
	passwordBaru = "SandiBaru#2026"
)

// simpanTokenReset menyimpan token atur ulang untuk akun uji seperti ForgotPassword (hanya hash yang disimpan).
func (s *Suite) simpanTokenReset(t *testing.T, raw string, kedaluwarsa time.Time) {
	t.Helper()
	hash := sha256.Sum256([]byte(raw))
	require.NoError(t, s.db.Exec(
		"INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (garda_uuid_stabil('pengguna', 'test-user-001'), ?, ?)",
		hex.EncodeToString(hash[:]), kedaluwarsa).Error)
}

func (s *Suite) reset(t *testing.T, token, password string) *httptest.ResponseRecorder {
	t.Helper()
	return s.serveRequest(t, http.MethodPost, "/v1/auth/reset-password", map[string]any{"token": token, "password": password}, nil)
}

func TestIntegration_ResetPassword_FullFlow(t *testing.T) {
	s := setupSuite(t)
	_, refresh := s.loginAs(t, emailUji, passwordLama)
	s.simpanTokenReset(t, "token-sah", time.Now().Add(30*time.Minute))

	w := s.reset(t, "token-sah", passwordBaru)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	t.Run("login dengan password baru berhasil, yang lama ditolak", func(t *testing.T) {
		s.loginAs(t, emailUji, passwordBaru)
		lama := s.serveRequest(t, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailUji, "password": passwordLama}, nil)
		assert.Equal(t, http.StatusUnauthorized, lama.Code)
	})
	t.Run("sesi lama dicabut", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodPost, "/v1/auth/refresh", map[string]any{"refresh_token": refresh}, nil)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
	t.Run("token hanya berlaku satu kali", func(t *testing.T) {
		again := s.reset(t, "token-sah", "LainLagi#2026")
		assert.Equal(t, http.StatusUnauthorized, again.Code)
		assert.Contains(t, s.parseResponse(t, again)["message"], "sudah dipakai")
		s.loginAs(t, emailUji, passwordBaru)
	})
	t.Run("tercatat di audit trail", func(t *testing.T) {
		var n int64
		require.NoError(t, s.db.Raw("SELECT count(*) FROM audit_log WHERE aksi = 'RESET_PASSWORD' AND entitas_id = garda_uuid_stabil('pengguna', 'test-user-001')::text").Scan(&n).Error)
		assert.EqualValues(t, 1, n)
	})
}

func TestIntegration_ResetPassword_ExpiredToken(t *testing.T) {
	s := setupSuite(t)
	s.simpanTokenReset(t, "token-lewat", time.Now().Add(-time.Minute))

	w := s.reset(t, "token-lewat", passwordBaru)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, s.parseResponse(t, w)["message"], "kedaluwarsa")
	s.loginAs(t, emailUji, passwordLama)
}

func TestIntegration_ResetPassword_WeakPasswordKeepsTokenUsable(t *testing.T) {
	s := setupSuite(t)
	s.simpanTokenReset(t, "token-sah", time.Now().Add(30*time.Minute))

	lemah := s.reset(t, "token-sah", "semuahurufkecil")
	assert.Equal(t, http.StatusBadRequest, lemah.Code)
	assert.Contains(t, s.parseResponse(t, lemah)["message"], "huruf kapital", "alasan kebijakan sampai ke pengguna")

	sama := s.reset(t, "token-sah", passwordLama)
	assert.Equal(t, http.StatusBadRequest, sama.Code, "tidak boleh sama dengan password lama")

	assert.Equal(t, http.StatusOK, s.reset(t, "token-sah", passwordBaru).Code, "token belum terpakai oleh percobaan yang ditolak")
}

func TestIntegration_ResetPassword_ConcurrentRequestsConsumeTokenOnce(t *testing.T) {
	s := setupSuite(t)
	s.simpanTokenReset(t, "token-balapan", time.Now().Add(30*time.Minute))

	const jumlah = 6
	hasil := make([]*httptest.ResponseRecorder, jumlah)
	var wg sync.WaitGroup
	for i := range jumlah {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasil[i] = s.reset(t, "token-balapan", passwordBaru)
		}()
	}
	wg.Wait()

	var berhasil int
	for _, w := range hasil {
		if w.Code == http.StatusOK {
			berhasil++
		} else {
			assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		}
	}
	assert.Equal(t, 1, berhasil, "tepat satu permintaan berhasil")
}

func TestIntegration_ResetPassword_InvalidatesOtherOutstandingTokens(t *testing.T) {
	s := setupSuite(t)
	s.simpanTokenReset(t, "token-pertama", time.Now().Add(30*time.Minute))
	s.simpanTokenReset(t, "token-kedua", time.Now().Add(30*time.Minute))

	require.Equal(t, http.StatusOK, s.reset(t, "token-pertama", passwordBaru).Code)

	assert.Equal(t, http.StatusUnauthorized, s.reset(t, "token-kedua", "LainLagi#2026").Code, "tautan lain milik akun yang sama ikut hangus")
}

func TestIntegration_ResetPassword_ClearsMandatoryChangeFlag(t *testing.T) {
	s := setupSuite(t)
	require.NoError(t, s.db.Exec("UPDATE pengguna SET wajib_ganti_password = true WHERE id = garda_uuid_stabil('pengguna', 'test-user-001')").Error)
	s.simpanTokenReset(t, "token-sah", time.Now().Add(30*time.Minute))

	require.Equal(t, http.StatusOK, s.reset(t, "token-sah", passwordBaru).Code)

	access, _ := s.loginAs(t, emailUji, passwordBaru)
	me := s.parseResponse(t, s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(access)))["data"].(map[string]any)
	assert.Equal(t, false, me["wajib_ganti_password"])
}
