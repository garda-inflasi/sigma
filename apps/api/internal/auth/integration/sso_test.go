package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"github.com/garda-inflasi/api/pkg/sso"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const ssoRedirectURL = "http://localhost:8090/api/auth/sso/google/callback"

func ssoConfig(idp *mockIdP) *sso.Config {
	return &sso.Config{
		Issuer: idp.Issuer(), ClientID: "garda-inflasi", ClientSecret: "rahasia-uji", RedirectURL: ssoRedirectURL,
		RetryAfter: 100 * time.Millisecond,
	}
}

// seedSSO menambah akun uji SSO di atas seed auth bawaan. Akun SSO-saja tidak punya kata sandi.
func seedSSO(t *testing.T, db *gorm.DB) {
	t.Helper()
	seedData(t, db)
	insert := func(id, mail, status string, extra string, args ...any) {
		require.NoError(t, db.Exec(`INSERT INTO pengguna (id, nama, email, peran, lapis, lembaga, status`+extra+`)
			VALUES (?, ?, ?, 'OPS_MAHASISWA', 'OPERASIONAL', 'BPPSDMP', ?`+placeholdersFor(args)+`)`,
			append([]any{id, "Pengguna " + id, mail, status}, args...)...).Error)
	}
	insert("sso-ani", "ani@kemenkeu.go.id", "AKTIF", "")
	insert("sso-ulang", "ulang@kemenkeu.go.id", "AKTIF", "")
	insert("sso-replay", "replay@kemenkeu.go.id", "AKTIF", "")
	insert("sso-pkce", "pkce@kemenkeu.go.id", "AKTIF", "")
	insert("sso-nonce", "nonce@kemenkeu.go.id", "AKTIF", "")
	insert("sso-kedaluwarsa", "kedaluwarsa@kemenkeu.go.id", "AKTIF", "")
	insert("sso-belum", "belum@kemenkeu.go.id", "AKTIF", "")
	insert("sso-nonaktif", "nonaktif@kemenkeu.go.id", "NONAKTIF", "")
	insert("sso-terkunci", "terkunci@kemenkeu.go.id", "AKTIF", "")
	insert("sso-hd", "hd@kemenkeu.go.id", "AKTIF", "")
	insert("sso-bertaut", "bertaut@kemenkeu.go.id", "AKTIF", ", sso_provider, sso_subject", "google", "sub-pemilik-asli")
	// Kunci login dicatat per email (kunci_login, lihat 01-F), bukan dari login_attempts.
	require.NoError(t, db.Exec(`INSERT INTO kunci_login (email_hash, gagal, jendela_mulai, terkunci_sampai, diperbarui_pada)
		VALUES (?, 5, now(), now() + interval '15 minutes', now())`, usecase.HashEmailKunci("terkunci@kemenkeu.go.id")).Error)
}

func placeholdersFor(args []any) string {
	out := ""
	for range args {
		out += ", ?"
	}
	return out
}

type ssoResult struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Suite) ssoAuthorize(t *testing.T) (authorizeURL, binding string) {
	t.Helper()
	w := s.serveRequest(t, http.MethodPost, "/v1/auth/sso/google/authorize", nil, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var data struct {
		AuthorizeURL string `json:"authorize_url"`
		Binding      string `json:"binding"`
	}
	s.decodeData(t, w.Body.Bytes(), &data)
	require.NotEmpty(t, data.AuthorizeURL)
	require.NotEmpty(t, data.Binding)
	return data.AuthorizeURL, data.Binding
}

func (s *Suite) ssoCallback(t *testing.T, code, state, binding string) *httptest.ResponseRecorder {
	t.Helper()
	return s.serveRequest(t, http.MethodPost, "/v1/auth/sso/google/callback",
		map[string]any{"code": code, "state": state, "binding": binding}, nil)
}

// ssoLogin menjalankan alur penuh dengan identitas tertentu: authorize, login di mock IdP, callback.
func (s *Suite) ssoLogin(t *testing.T, idp *mockIdP, sub string, claims map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	authorizeURL, binding := s.ssoAuthorize(t)
	raw, err := json.Marshal(claims)
	require.NoError(t, err)
	code, state := idp.signIn(t, authorizeURL, sub, string(raw))
	return s.ssoCallback(t, code, state, binding)
}

func identitas(mail string) map[string]any {
	return map[string]any{"email": mail, "email_verified": true, "name": "Pengguna Uji"}
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var res ssoResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res), w.Body.String())
	return res.Code
}

type ssoSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	MFARequired  bool   `json:"mfa_required"`
	MFAToken     string `json:"mfa_token"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Peran string `json:"peran"`
	} `json:"user"`
}

func (s *Suite) ssoSession(t *testing.T, w *httptest.ResponseRecorder) ssoSession {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var data ssoSession
	s.decodeData(t, w.Body.Bytes(), &data)
	return data
}

func (s *Suite) ssoLink(t *testing.T, userID string) (provider, subject *string) {
	t.Helper()
	var row struct {
		SSOProvider *string
		SSOSubject  *string
	}
	require.NoError(t, s.db.Raw("SELECT sso_provider, sso_subject FROM pengguna WHERE id = ?", userID).Scan(&row).Error)
	return row.SSOProvider, row.SSOSubject
}

func (s *Suite) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Raw(query, args...).Scan(&n).Error)
	return n
}

func TestIntegration_SSO(t *testing.T) {
	idp := startMockIdP(t)
	cfg := routes.ConfigFromEnv(os.Getenv)
	s := newSuite(t, seedSSO, routes.Options{UseCaseConfig: cfg, SSO: ssoConfig(idp)})

	t.Run("status melaporkan Google aktif", func(t *testing.T) {
		w := s.serveRequest(t, http.MethodGet, "/v1/auth/sso/status", nil, nil)
		require.Equal(t, http.StatusOK, w.Code)
		var data struct {
			Google bool `json:"google"`
		}
		s.decodeData(t, w.Body.Bytes(), &data)
		assert.True(t, data.Google)
	})

	t.Run("authorize memuat PKCE S256, state, nonce dan menyimpan state sebagai hash", func(t *testing.T) {
		before := s.count(t, "SELECT count(*) FROM sso_login_state")
		authorizeURL, binding := s.ssoAuthorize(t)

		u, err := url.Parse(authorizeURL)
		require.NoError(t, err)
		q := u.Query()
		assert.Equal(t, idp.Issuer()+"/authorize", u.Scheme+"://"+u.Host+u.Path)
		assert.Equal(t, "code", q.Get("response_type"))
		assert.Equal(t, "garda-inflasi", q.Get("client_id"))
		assert.Equal(t, ssoRedirectURL, q.Get("redirect_uri"))
		assert.Equal(t, "S256", q.Get("code_challenge_method"))
		assert.Len(t, q.Get("code_challenge"), 43)
		assert.GreaterOrEqual(t, len(q.Get("state")), 43)
		assert.GreaterOrEqual(t, len(q.Get("nonce")), 43)
		assert.Contains(t, q.Get("scope"), "openid")
		assert.Contains(t, q.Get("scope"), "email")

		assert.EqualValues(t, before+1, s.count(t, "SELECT count(*) FROM sso_login_state"))
		assert.Zero(t, s.count(t, "SELECT count(*) FROM sso_login_state WHERE state_hash = ? OR binding_hash = ? OR state_hash = ? OR binding_hash = ?",
			q.Get("state"), q.Get("state"), binding, binding), "state dan binding tidak disimpan polos")
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM sso_login_state WHERE nonce = ? AND provider = 'google'", q.Get("nonce")),
			"nonce di URL sama dengan yang tersimpan di server")
	})

	t.Run("authorize membersihkan baris kedaluwarsa", func(t *testing.T) {
		require.NoError(t, s.db.Exec(`INSERT INTO sso_login_state (state_hash, binding_hash, provider, nonce, code_verifier, expires_at)
			VALUES ('kedaluwarsa-1', 'b', 'google', 'n', 'v', now() - interval '1 minute')`).Error)
		s.ssoAuthorize(t)
		assert.Zero(t, s.count(t, "SELECT count(*) FROM sso_login_state WHERE state_hash = 'kedaluwarsa-1'"))
	})

	t.Run("masuk pertama menautkan akun lalu menerbitkan sesi yang bekerja", func(t *testing.T) {
		assert.Nil(t, firstOf(s.ssoLink(t, "sso-ani")))

		w := s.ssoLogin(t, idp, "sub-ani", identitas("ani@kemenkeu.go.id"))
		data := s.ssoSession(t, w)
		assert.Equal(t, "sso-ani", data.User.ID)
		assert.Equal(t, "OPS_MAHASISWA", data.User.Peran)
		require.NotEmpty(t, data.AccessToken)
		assert.NotEmpty(t, data.RefreshToken)

		provider, subject := s.ssoLink(t, "sso-ani")
		require.NotNil(t, provider)
		require.NotNil(t, subject)
		assert.Equal(t, "google", *provider)
		assert.Equal(t, "sub-ani", *subject)
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM pengguna WHERE id = 'sso-ani' AND sso_terhubung_pada IS NOT NULL AND terakhir_masuk IS NOT NULL"))

		me := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(data.AccessToken))
		assert.Equal(t, http.StatusOK, me.Code, "token SSO diterima middleware JWT yang sama")

		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND aktor_id = 'sso-ani' AND hasil = 'BERHASIL' AND metadata->>'metode' = 'sso_google'"))
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'SSO_LINK' AND aktor_id = 'sso-ani' AND metadata->>'sso_subject' = 'sub-ani'"))
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM login_attempts WHERE user_id = 'sso-ani' AND success"))
	})

	t.Run("masuk berikutnya mencocokkan subjek walau email di penyedia berubah", func(t *testing.T) {
		w := s.ssoLogin(t, idp, "sub-ani", identitas("ani.ganti-nama@kemenkeu.go.id"))
		data := s.ssoSession(t, w)
		assert.Equal(t, "sso-ani", data.User.ID)
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'SSO_LINK' AND aktor_id = 'sso-ani'"), "tidak ditautkan ulang")
		assert.EqualValues(t, 2, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND aktor_id = 'sso-ani' AND metadata->>'metode' = 'sso_google'"))
	})

	t.Run("email tak dikenal ditolak tanpa membuat akun", func(t *testing.T) {
		before := s.count(t, "SELECT count(*) FROM pengguna")
		w := s.ssoLogin(t, idp, "sub-orang-asing", identitas("orang.asing@example.com"))
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "SSO_ACCOUNT_NOT_FOUND", errorCode(t, w))
		assert.Equal(t, before, s.count(t, "SELECT count(*) FROM pengguna"), "tidak ada pendaftaran mandiri")
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND hasil = 'GAGAL' AND metadata->>'alasan' = 'sso_akun_tidak_ditemukan' AND metadata->>'email' = 'orang.asing@example.com'"))
	})

	t.Run("email milik akun yang bertaut ke subjek lain ditolak dan diaudit", func(t *testing.T) {
		w := s.ssoLogin(t, idp, "sub-penyerang", identitas("bertaut@kemenkeu.go.id"))
		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Equal(t, "SSO_ACCOUNT_CONFLICT", errorCode(t, w))
		_, subject := s.ssoLink(t, "sso-bertaut")
		require.NotNil(t, subject)
		assert.Equal(t, "sub-pemilik-asli", *subject, "tautan lama tidak tergeser")
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND aktor_id = 'sso-bertaut' AND hasil = 'DITOLAK' AND metadata->>'alasan' = 'sso_subjek_berbeda' AND metadata->>'sso_subject' = 'sub-penyerang'"))
		assert.Zero(t, s.count(t, "SELECT count(*) FROM refresh_tokens WHERE user_id = 'sso-bertaut'"))

		// Pemilik asli tetap bisa masuk.
		ok := s.ssoLogin(t, idp, "sub-pemilik-asli", identitas("bertaut@kemenkeu.go.id"))
		assert.Equal(t, "sso-bertaut", s.ssoSession(t, ok).User.ID)
	})

	t.Run("email belum terverifikasi ditolak dan tidak menautkan", func(t *testing.T) {
		w := s.ssoLogin(t, idp, "sub-belum", map[string]any{"email": "belum@kemenkeu.go.id", "email_verified": false})
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "SSO_EMAIL_UNVERIFIED", errorCode(t, w))
		assert.Nil(t, firstOf(s.ssoLink(t, "sso-belum")))

		tanpaKlaim := s.ssoLogin(t, idp, "sub-belum", map[string]any{"email": "belum@kemenkeu.go.id"})
		assert.Equal(t, "SSO_EMAIL_UNVERIFIED", errorCode(t, tanpaKlaim), "tanpa klaim email_verified = tidak terverifikasi")
	})

	t.Run("akun nonaktif ditolak seperti login kata sandi", func(t *testing.T) {
		w := s.ssoLogin(t, idp, "sub-nonaktif", identitas("nonaktif@kemenkeu.go.id"))
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "SSO_ACCOUNT_INACTIVE", errorCode(t, w))
		assert.Nil(t, firstOf(s.ssoLink(t, "sso-nonaktif")))
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM login_attempts WHERE user_id = 'sso-nonaktif' AND NOT success AND failure_reason = 'account_inactive'"))
	})

	t.Run("akun terkunci ditolak seperti login kata sandi", func(t *testing.T) {
		w := s.ssoLogin(t, idp, "sub-terkunci", identitas("terkunci@kemenkeu.go.id"))
		assert.Equal(t, http.StatusLocked, w.Code)
		assert.NotEmpty(t, w.Header().Get("Retry-After"))
		assert.Equal(t, "SSO_ACCOUNT_LOCKED", errorCode(t, w))
		assert.Nil(t, firstOf(s.ssoLink(t, "sso-terkunci")))
	})

	t.Run("akun ber-MFA melanjutkan ke langkah OTP yang sama dengan login kata sandi", func(t *testing.T) {
		w := s.ssoLogin(t, idp, "sub-mfa", identitas(emailMFA))
		data := s.ssoSession(t, w)
		require.True(t, data.MFARequired, w.Body.String())
		require.NotEmpty(t, data.MFAToken)
		assert.Empty(t, data.AccessToken, "belum ada sesi sebelum OTP")
		assert.Empty(t, data.RefreshToken)
		assert.Zero(t, s.count(t, "SELECT count(*) FROM refresh_tokens WHERE user_id = 'test-user-mfa'"))

		tanpaSesi := s.serveRequest(t, http.MethodGet, "/v1/auth/me", nil, s.authHeader(data.MFAToken))
		assert.Equal(t, http.StatusUnauthorized, tanpaSesi.Code, "token tantangan bukan kredensial sesi")

		salah := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate", map[string]any{"mfa_token": data.MFAToken, "code": "000000"}, nil)
		assert.Equal(t, http.StatusUnauthorized, salah.Code)

		valid := s.serveRequest(t, http.MethodPost, "/v1/auth/mfa/validate",
			map[string]any{"mfa_token": data.MFAToken, "code": kodeOTP(t, mfaSecretUji, time.Now())}, nil)
		sesi := s.ssoSession(t, valid)
		assert.Equal(t, "test-user-mfa", sesi.User.ID)
		assert.NotEmpty(t, sesi.AccessToken)

		_, subject := s.ssoLink(t, "test-user-mfa")
		require.NotNil(t, subject)
		assert.Equal(t, "sub-mfa", *subject)
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'SSO_LOGIN' AND aktor_id = 'test-user-mfa' AND metadata->>'metode' = 'sso_google' AND metadata->>'tahap' = 'menunggu_mfa'"))
		// LOGIN akhir (sesi terbit setelah OTP) memuat metode masuk SSO bersama faktor keduanya, bukan hanya mfa=totp.
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND hasil = 'BERHASIL' AND aktor_id = 'test-user-mfa' AND metadata->>'metode' = 'sso_google' AND metadata->>'mfa' = 'totp'"))
	})

	t.Run("state hanya sekali pakai: callback yang diulang ditolak", func(t *testing.T) {
		authorizeURL, binding := s.ssoAuthorize(t)
		code, state := idp.signIn(t, authorizeURL, "sub-replay", mustJSON(t, identitas("replay@kemenkeu.go.id")))

		first := s.ssoCallback(t, code, state, binding)
		require.Equal(t, http.StatusOK, first.Code, first.Body.String())

		again := s.ssoCallback(t, code, state, binding)
		assert.Equal(t, http.StatusBadRequest, again.Code)
		assert.Equal(t, "SSO_STATE_INVALID", errorCode(t, again))
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM refresh_tokens WHERE user_id = 'sso-replay'"), "tidak ada sesi kedua")
	})

	t.Run("binding salah (login CSRF) ditolak dan state hangus", func(t *testing.T) {
		authorizeURL, binding := s.ssoAuthorize(t)
		code, state := idp.signIn(t, authorizeURL, "sub-ulang", mustJSON(t, identitas("ulang@kemenkeu.go.id")))

		// Korban membawa state penyerang tetapi cookie binding miliknya sendiri.
		serangan := s.ssoCallback(t, code, state, "binding-browser-korban")
		assert.Equal(t, http.StatusBadRequest, serangan.Code)
		assert.Equal(t, "SSO_STATE_INVALID", errorCode(t, serangan))
		assert.Nil(t, firstOf(s.ssoLink(t, "sso-ulang")))

		sah := s.ssoCallback(t, code, state, binding)
		assert.Equal(t, http.StatusBadRequest, sah.Code, "state sudah hangus pada percobaan pertama")
		assert.Zero(t, s.count(t, "SELECT count(*) FROM refresh_tokens WHERE user_id = 'sso-ulang'"))
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND hasil = 'DITOLAK' AND metadata->>'alasan' = 'binding_tidak_cocok'"))
	})

	t.Run("state kedaluwarsa ditolak", func(t *testing.T) {
		authorizeURL, binding := s.ssoAuthorize(t)
		code, state := idp.signIn(t, authorizeURL, "sub-kedaluwarsa", mustJSON(t, identitas("kedaluwarsa@kemenkeu.go.id")))
		require.NoError(t, s.db.Exec("UPDATE sso_login_state SET expires_at = now() - interval '1 second'").Error)

		w := s.ssoCallback(t, code, state, binding)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "SSO_STATE_INVALID", errorCode(t, w))
		assert.Nil(t, firstOf(s.ssoLink(t, "sso-kedaluwarsa")))
	})

	t.Run("state tak dikenal dan parameter tidak lengkap ditolak", func(t *testing.T) {
		w := s.ssoCallback(t, "kode", "state-karangan", "binding")
		assert.Equal(t, "SSO_STATE_INVALID", errorCode(t, w))
		assert.Equal(t, http.StatusBadRequest, s.serveRequest(t, http.MethodPost, "/v1/auth/sso/google/callback", map[string]any{"code": "x"}, nil).Code)
	})

	t.Run("PKCE ditegakkan: kode dari alur lain tidak bisa ditukar dengan verifier alur ini", func(t *testing.T) {
		urlA, bindingA := s.ssoAuthorize(t)
		stateA := mustQuery(t, urlA, "state")
		urlB, _ := s.ssoAuthorize(t)
		codeB, _ := idp.signIn(t, urlB, "sub-pkce", mustJSON(t, identitas("pkce@kemenkeu.go.id")))

		w := s.ssoCallback(t, codeB, stateA, bindingA)
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		assert.Equal(t, "SSO_EXCHANGE_FAILED", errorCode(t, w))
		assert.Nil(t, firstOf(s.ssoLink(t, "sso-pkce")))
	})

	t.Run("nonce diverifikasi: ID token dengan nonce alur lain ditolak", func(t *testing.T) {
		authorizeURL, binding := s.ssoAuthorize(t)
		u, err := url.Parse(authorizeURL)
		require.NoError(t, err)
		q := u.Query()
		q.Set("nonce", "nonce-penyerang") // IdP menerbitkan token dengan nonce yang bukan milik alur ini
		u.RawQuery = q.Encode()
		code, state := idp.signIn(t, u.String(), "sub-nonce", mustJSON(t, identitas("nonce@kemenkeu.go.id")))

		w := s.ssoCallback(t, code, state, binding)
		assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		assert.Equal(t, "SSO_EXCHANGE_FAILED", errorCode(t, w))
		assert.Nil(t, firstOf(s.ssoLink(t, "sso-nonce")))
		assert.EqualValues(t, 1, s.count(t, "SELECT count(*) FROM audit_log WHERE aksi = 'LOGIN' AND hasil = 'DITOLAK' AND metadata->>'alasan' = 'sso_nonce_tidak_cocok'"))
	})

	t.Run("identitas yang sama tidak bisa menaut dua akun (UNIQUE sso_provider+sso_subject)", func(t *testing.T) {
		err := s.db.Exec("UPDATE pengguna SET sso_provider = 'google', sso_subject = 'sub-ani' WHERE id = 'sso-hd'").Error
		require.Error(t, err)
		assert.ErrorIs(t, err, gorm.ErrDuplicatedKey)
		err = s.db.Exec("UPDATE pengguna SET sso_provider = 'google' WHERE id = 'sso-hd'").Error
		assert.Error(t, err, "provider tanpa subject melanggar CHECK pasangan")
	})

	t.Run("domain Workspace dibatasi lewat SSO_GOOGLE_HOSTED_DOMAIN", func(t *testing.T) {
		cfg := ssoConfig(idp)
		cfg.HostedDomain = "kemenkeu.go.id"
		s := newSuite(t, seedSSO, routes.Options{UseCaseConfig: routes.ConfigFromEnv(os.Getenv), SSO: cfg})

		authorizeURL, _ := s.ssoAuthorize(t)
		assert.Equal(t, "kemenkeu.go.id", mustQuery(t, authorizeURL, "hd"), "petunjuk domain diteruskan ke Google")

		t.Run("hd cocok diterima", func(t *testing.T) {
			claims := identitas("hd@kemenkeu.go.id")
			claims["hd"] = "kemenkeu.go.id"
			w := s.ssoLogin(t, idp, "sub-hd", claims)
			assert.Equal(t, "sso-hd", s.ssoSession(t, w).User.ID)
		})
		t.Run("hd lain ditolak", func(t *testing.T) {
			claims := identitas("ani@kemenkeu.go.id")
			claims["hd"] = "lain.go.id"
			w := s.ssoLogin(t, idp, "sub-hd-lain", claims)
			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Equal(t, "SSO_DOMAIN_NOT_ALLOWED", errorCode(t, w))
			assert.Nil(t, firstOf(s.ssoLink(t, "sso-ani")))
		})
		t.Run("akun pribadi tanpa hd ditolak", func(t *testing.T) {
			w := s.ssoLogin(t, idp, "sub-pribadi", identitas("ani@kemenkeu.go.id"))
			assert.Equal(t, "SSO_DOMAIN_NOT_ALLOWED", errorCode(t, w))
		})
	})
}

func TestIntegration_SSO_TanpaKonfigurasi(t *testing.T) {
	s := newSuite(t, seedSSO, routes.Options{UseCaseConfig: routes.ConfigFromEnv(os.Getenv)})

	status := s.serveRequest(t, http.MethodGet, "/v1/auth/sso/status", nil, nil)
	require.Equal(t, http.StatusOK, status.Code)
	var data struct {
		Google bool `json:"google"`
	}
	s.decodeData(t, status.Body.Bytes(), &data)
	assert.False(t, data.Google)

	authorize := s.serveRequest(t, http.MethodPost, "/v1/auth/sso/google/authorize", nil, nil)
	assert.Equal(t, http.StatusServiceUnavailable, authorize.Code)
	assert.Equal(t, "SSO_NOT_CONFIGURED", errorCode(t, authorize))

	callback := s.ssoCallback(t, "kode", "state", "binding")
	assert.Equal(t, http.StatusServiceUnavailable, callback.Code)
	assert.Equal(t, "SSO_NOT_CONFIGURED", errorCode(t, callback))
	assert.Zero(t, s.count(t, "SELECT count(*) FROM sso_login_state"))

	// Login kata sandi tidak terpengaruh.
	s.loginAs(t, "test@gardainflasi.id", "Test1234!")
}

func TestIntegration_SSO_KonfigurasiTidakValidMenonaktifkanSSOTanpaMenghentikanAPI(t *testing.T) {
	s := newSuite(t, seedSSO, routes.Options{
		UseCaseConfig: routes.ConfigFromEnv(os.Getenv),
		SSO:           &sso.Config{ClientID: "ada", Issuer: "https://accounts.google.com"}, // tanpa rahasia dan redirect URL
	})
	authorize := s.serveRequest(t, http.MethodPost, "/v1/auth/sso/google/authorize", nil, nil)
	assert.Equal(t, http.StatusServiceUnavailable, authorize.Code)
	assert.Equal(t, "SSO_NOT_CONFIGURED", errorCode(t, authorize))
	s.loginAs(t, "test@gardainflasi.id", "Test1234!")
}

func TestIntegration_SSO_PenerbitTidakTerjangkauAPITetapStart(t *testing.T) {
	// Port 1 tidak mendengarkan: penemuan gagal, tetapi API sudah berdiri dan login kata sandi jalan.
	s := newSuite(t, seedSSO, routes.Options{
		UseCaseConfig: routes.ConfigFromEnv(os.Getenv),
		SSO: &sso.Config{
			Issuer: "http://127.0.0.1:1/default", ClientID: "garda-inflasi", ClientSecret: "rahasia-uji", RedirectURL: ssoRedirectURL,
			RetryAfter: 50 * time.Millisecond,
		},
	})
	s.loginAs(t, "test@gardainflasi.id", "Test1234!")

	status := s.serveRequest(t, http.MethodGet, "/v1/auth/sso/status", nil, nil)
	var data struct {
		Google bool `json:"google"`
	}
	s.decodeData(t, status.Body.Bytes(), &data)
	assert.True(t, data.Google, "SSO terkonfigurasi walau penerbit sedang mati")

	authorize := s.serveRequest(t, http.MethodPost, "/v1/auth/sso/google/authorize", nil, nil)
	assert.Equal(t, http.StatusServiceUnavailable, authorize.Code)
	assert.Equal(t, "SSO_UNAVAILABLE", errorCode(t, authorize))
	assert.Zero(t, s.count(t, "SELECT count(*) FROM sso_login_state"), "tidak ada state tersimpan saat penyedia mati")
}

func firstOf(a, _ *string) *string { return a }

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

func mustQuery(t *testing.T, rawURL, key string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u.Query().Get(key)
}
