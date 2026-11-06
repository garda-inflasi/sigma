package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Batas laju auth per IP di API Go. Caddy membuka /api/v1/* langsung ke Go sehingga login lewat jalur itu tidak
// melewati batas laju BFF (5/menit/IP); tanpa pembatas di sini, penebakan kata sandi lintas email hanya dibatasi
// batas global API (600/menit). Pembatas dipasang di rute publik auth (routes.Register) dan dikonfigurasi lewat
// AUTH_RATE_LIMIT_PER_MENIT; tes memakai Dependencies.AuthRateLimitPerMenit.

// kirimDariIP mengirim permintaan seolah datang dari klien dengan IP tertentu lewat proxy tepercaya (httptest
// memakai 192.0.2.1 sebagai alamat koneksi).
func (s *Suite) kirimDariIP(t *testing.T, ip, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return s.serveRequest(t, method, path, body, map[string]string{"X-Forwarded-For": ip})
}

type jawaban429 struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    struct {
		SisaDetik int `json:"sisa_detik"`
	} `json:"data"`
}

// tegas429 menegaskan kontrak 429 yang sama dengan BFF (kode RATE_LIMITED, Retry-After, data.sisa_detik).
func tegas429(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, w.Code, w.Body.String())
	var j jawaban429
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &j), w.Body.String())
	assert.Equal(t, "RATE_LIMITED", j.Code)
	assert.NotEmpty(t, j.Message)
	assert.Greater(t, j.Data.SisaDetik, 0)
	assert.LessOrEqual(t, j.Data.SisaDetik, 60)
	assert.Equal(t, strconv.Itoa(j.Data.SisaDetik), w.Header().Get("Retry-After"))
}

func TestBatasLajuAuth_LoginLangsungKeGoDibatasPerIP(t *testing.T) {
	s := newSuiteBatas(t, seedData, 3, []string{"192.0.2.0/24"})
	const ipA, ipB = "198.51.100.10", "198.51.100.11"

	// Tiga percobaan lolos ke handler (kata sandi salah → 401), yang keempat ditolak sebelum menyentuh akun.
	for i := 1; i <= 3; i++ {
		w := s.kirimDariIP(t, ipA, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailBiasa, "password": sandiSalahUji})
		require.Equal(t, http.StatusUnauthorized, w.Code, "percobaan %d: %s", i, w.Body.String())
	}
	tegas429(t, s.kirimDariIP(t, ipA, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailBiasa, "password": sandiSalahUji}))

	t.Run("kata sandi benar pun ditolak selama dibatasi, dan penolakan tidak dihitung sebagai tebakan", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			tegas429(t, s.kirimDariIP(t, ipA, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailBiasa, "password": sandiBenarUji}))
		}
		assert.Equal(t, 3, s.kunciDi(t, emailBiasa).Gagal, "yang ditolak batas laju tidak sampai ke lockout")
		assert.EqualValues(t, 3, s.hitung(t, "SELECT count(*) FROM login_attempts WHERE user_id = ?", idBiasa))
	})

	t.Run("IP lain tidak terpengaruh", func(t *testing.T) {
		w := s.kirimDariIP(t, ipB, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailAdmin, "password": sandiBenarUji})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})
}

func TestBatasLajuAuth_SatuEmberUntukLoginOTPLupaDanAturUlang(t *testing.T) {
	s := newSuiteBatas(t, seedData, 4, []string{"192.0.2.0/24"})
	const ip = "198.51.100.20"

	// Empat permintaan ke empat endpoint berbeda menghabiskan satu ember yang sama.
	assert.Equal(t, http.StatusUnauthorized, s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailBiasa, "password": sandiSalahUji}).Code)
	assert.Equal(t, http.StatusUnauthorized, s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": "palsu", "code": "123456"}).Code)
	assert.Equal(t, http.StatusOK, s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/forgot-password", map[string]any{"email": "tak.ada@gardainflasi.id"}).Code)
	assert.Equal(t, http.StatusUnauthorized, s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/reset-password", map[string]any{"token": "palsu", "password": "BaruSekali#77"}).Code)

	for _, ep := range []struct {
		path string
		body map[string]any
	}{
		{"/v1/auth/login", map[string]any{"email": emailBiasa, "password": sandiBenarUji}},
		{"/v1/auth/mfa/validate", map[string]any{"mfa_token": "palsu", "code": "123456"}},
		{"/v1/auth/forgot-password", map[string]any{"email": "tak.ada@gardainflasi.id"}},
		{"/v1/auth/reset-password", map[string]any{"token": "palsu", "password": "BaruSekali#77"}},
	} {
		tegas429(t, s.kirimDariIP(t, ip, http.MethodPost, ep.path, ep.body))
	}
}

func TestBatasLajuAuth_SSOPunyaEmberSendiriDanStatusTidakDibatasi(t *testing.T) {
	s := newSuiteBatas(t, seedData, 2, []string{"192.0.2.0/24"})
	const ip = "198.51.100.30"
	login := func() *httptest.ResponseRecorder {
		return s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailBiasa, "password": sandiSalahUji})
	}
	authorize := func() *httptest.ResponseRecorder {
		return s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/sso/google/authorize", nil)
	}

	login()
	login()
	tegas429(t, login())

	// Jatah login habis, tetapi SSO (belum dikonfigurasi: 503) masih dilayani handler-nya.
	assert.Equal(t, http.StatusServiceUnavailable, authorize().Code)
	assert.Equal(t, http.StatusServiceUnavailable, s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/sso/google/callback", map[string]any{"code": "k", "state": "s", "binding": "b"}).Code)
	tegas429(t, authorize())
	tegas429(t, s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/sso/google/callback", map[string]any{"code": "k", "state": "s", "binding": "b"}))

	// Status SSO dibaca halaman masuk di setiap render: tidak dibatasi.
	for i := 0; i < 10; i++ {
		require.Equal(t, http.StatusOK, s.kirimDariIP(t, ip, http.MethodGet, "/v1/auth/sso/status", nil).Code)
	}
}

func TestBatasLajuAuth_XForwardedForPalsuDariKlienLangsungTidakMembukaEmberBaru(t *testing.T) {
	// Tanpa proxy tepercaya (TRUSTED_PROXIES=none): X-Forwarded-For dari siapa pun diabaikan, kuncinya alamat koneksi.
	s := newSuiteBatas(t, seedData, 2, []string{})
	login := func(xff string) *httptest.ResponseRecorder {
		return s.kirimDariIP(t, xff, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailBiasa, "password": sandiSalahUji})
	}

	assert.Equal(t, http.StatusUnauthorized, login("1.1.1.1").Code)
	assert.Equal(t, http.StatusUnauthorized, login("2.2.2.2").Code)
	for _, palsu := range []string{"3.3.3.3", "4.4.4.4", "5.5.5.5, 6.6.6.6"} {
		tegas429(t, login(palsu))
	}
}

func TestBatasLajuAuth_RefreshDanEndpointBersesiTidakDibatasi(t *testing.T) {
	s := newSuiteBatas(t, seedData, 1, []string{"192.0.2.0/24"})
	const ip = "198.51.100.40"
	w := s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailAdmin, "password": sandiBenarUji})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var data struct {
		AccessToken string `json:"access_token"`
	}
	s.decodeData(t, w.Body.Bytes(), &data)
	tegas429(t, s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/login", map[string]any{"email": emailAdmin, "password": sandiBenarUji}))

	for i := 0; i < 5; i++ {
		// Refresh dipanggil otomatis oleh klien: jawaban 401 karena tokennya palsu, bukan 429.
		r := s.kirimDariIP(t, ip, http.MethodPost, "/v1/auth/refresh", map[string]any{"refresh_token": "palsu"})
		require.Equal(t, http.StatusUnauthorized, r.Code)
		me := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, map[string]string{
			"X-Forwarded-For": ip, "Authorization": "Bearer " + data.AccessToken,
		})
		require.Equal(t, http.StatusOK, me.Code, me.Body.String())
	}
}

func TestBatasLajuAuth_NolMematikanDanRetryAfterMendekatiSatuMenit(t *testing.T) {
	t.Run("nol berarti tanpa pembatas (nilai tes)", func(t *testing.T) {
		s := newSuiteBatas(t, seedData, 0, nil)
		for i := 0; i < 15; i++ {
			w := s.coba(t, emailAdmin, sandiBenarUji)
			require.Equal(t, http.StatusOK, w.Code, "login ke-%d: %s", i+1, w.Body.String())
		}
	})
	t.Run("kontrak Retry-After mendekati satu menit sesudah batas tercapai", func(t *testing.T) {
		s := newSuiteBatas(t, seedData, 1, nil)
		require.Equal(t, http.StatusOK, s.coba(t, emailAdmin, sandiBenarUji).Code)
		w := s.coba(t, emailAdmin, sandiBenarUji)
		tegas429(t, w)
		detik, err := strconv.Atoi(w.Header().Get("Retry-After"))
		require.NoError(t, err)
		assert.InDelta(t, 60, detik, 3)
	})
}
