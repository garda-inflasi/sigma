package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testClientID     = "garda-client"
	testClientSecret = "garda-secret"
	testRedirectURL  = "https://garda.example.id/api/auth/sso/google/callback"
	testKeyID        = "kunci-uji"
)

// fakeIdP adalah penyedia OIDC dalam proses: penemuan dan JWKS dari oidctest, ditambah endpoint token
// yang mengembalikan ID token hasil rakitan tes. Menyimpan permintaan token terakhir untuk diperiksa.
type fakeIdP struct {
	t    *testing.T
	srv  *httptest.Server
	priv *rsa.PrivateKey
	oidc *oidctest.Server

	mu          sync.Mutex
	claims      func(issuer string) map[string]any // klaim ID token; nil = klaim valid bawaan
	signWith    *rsa.PrivateKey                    // kunci penanda tangan; nil = kunci yang diiklankan JWKS
	rawIDToken  func(issuer string) string         // menimpa seluruh id_token (mis. alg none)
	tokenStatus int                                // 0 = 200
	tokenBody   string                             // menimpa badan galat
	omitIDToken bool
	tokenForm   url.Values
	tokenUser   string
	tokenPass   string
	discoveries atomic.Int32
	down        atomic.Bool
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := &fakeIdP{t: t, priv: priv}
	f.oidc = &oidctest.Server{PublicKeys: []oidctest.PublicKey{{PublicKey: priv.Public(), KeyID: testKeyID, Algorithm: oidc.RS256}}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	f.oidc.SetIssuer(f.srv.URL)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIdP) serve(w http.ResponseWriter, r *http.Request) {
	if f.down.Load() {
		http.Error(w, "penyedia mati", http.StatusServiceUnavailable)
		return
	}
	switch r.URL.Path {
	case "/token":
		f.serveToken(w, r)
	case "/.well-known/openid-configuration":
		f.discoveries.Add(1)
		f.oidc.ServeHTTP(w, r)
	default:
		f.oidc.ServeHTTP(w, r)
	}
}

func (f *fakeIdP) serveToken(w http.ResponseWriter, r *http.Request) {
	require.NoError(f.t, r.ParseForm())
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenForm = r.PostForm
	f.tokenUser, f.tokenPass, _ = r.BasicAuth()
	w.Header().Set("Content-Type", "application/json")
	if f.tokenStatus != 0 {
		w.WriteHeader(f.tokenStatus)
		_, _ = w.Write([]byte(f.tokenBody))
		return
	}
	resp := map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600}
	if !f.omitIDToken {
		resp["id_token"] = f.signIDToken()
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (f *fakeIdP) signIDToken() string {
	if f.rawIDToken != nil {
		return f.rawIDToken(f.srv.URL)
	}
	claims := validClaims(f.srv.URL)
	if f.claims != nil {
		claims = f.claims(f.srv.URL)
	}
	raw, err := json.Marshal(claims)
	require.NoError(f.t, err)
	key := f.priv
	if f.signWith != nil {
		key = f.signWith
	}
	return oidctest.SignIDToken(key, testKeyID, oidc.RS256, string(raw))
}

func validClaims(issuer string) map[string]any {
	return map[string]any{
		"iss": issuer, "aud": testClientID, "sub": "google-sub-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"nonce": "nonce-1", "email": "ani@kemenkeu.go.id", "email_verified": true, "name": "Ani", "hd": "kemenkeu.go.id",
	}
}

func (f *fakeIdP) config() Config {
	return Config{
		Issuer: f.srv.URL, ClientID: testClientID, ClientSecret: testClientSecret, RedirectURL: testRedirectURL,
		HTTPClient: f.srv.Client(),
	}
}

func (f *fakeIdP) provider() *Provider {
	f.t.Helper()
	p, err := New(f.config())
	require.NoError(f.t, err)
	return p
}

func (f *fakeIdP) setClaims(fn func(c map[string]any)) {
	f.claims = func(issuer string) map[string]any {
		c := validClaims(issuer)
		fn(c)
		return c
	}
}

func exchange(t *testing.T, p *Provider) (*Identity, error) {
	t.Helper()
	return p.Exchange(context.Background(), "kode-1", NewCodeVerifier(), "nonce-1")
}

// ── Konfigurasi ──────────────────────────────────────────────────────────

func TestConfigFromEnvBawaanDanTrim(t *testing.T) {
	env := map[string]string{
		"SSO_GOOGLE_CLIENT_ID": " id ", "SSO_GOOGLE_CLIENT_SECRET": " rahasia ",
		"SSO_GOOGLE_REDIRECT_URL": " https://app.id/api/auth/sso/google/callback ", "SSO_GOOGLE_HOSTED_DOMAIN": " Kemenkeu.GO.id ",
	}
	cfg := ConfigFromEnv(func(k string) string { return env[k] })
	assert.Equal(t, DefaultGoogleIssuer, cfg.Issuer, "penerbit bawaan Google")
	assert.Equal(t, "id", cfg.ClientID)
	assert.Equal(t, "rahasia", cfg.ClientSecret)
	assert.Equal(t, "https://app.id/api/auth/sso/google/callback", cfg.RedirectURL)
	assert.Equal(t, "kemenkeu.go.id", cfg.HostedDomain, "domain dinormalkan huruf kecil")
	assert.True(t, cfg.Enabled())
	assert.NoError(t, cfg.Validate())
}

func TestConfigTidakDiaktifkanTanpaClientID(t *testing.T) {
	cfg := ConfigFromEnv(func(string) string { return "" })
	assert.False(t, cfg.Enabled())
	assert.Equal(t, DefaultGoogleIssuer, cfg.Issuer)
}

func TestConfigValidate(t *testing.T) {
	ok := Config{Issuer: "https://accounts.google.com", ClientID: "a", ClientSecret: "b", RedirectURL: "https://x.id/cb"}
	require.NoError(t, ok.Validate())

	cases := map[string]func(c *Config){
		"SSO_GOOGLE_CLIENT_ID kosong":     func(c *Config) { c.ClientID = "" },
		"SSO_GOOGLE_CLIENT_SECRET kosong": func(c *Config) { c.ClientSecret = "" },
		"SSO_GOOGLE_REDIRECT_URL kosong":  func(c *Config) { c.RedirectURL = "" },
		"SSO_GOOGLE_REDIRECT_URL harus":   func(c *Config) { c.RedirectURL = "/relatif" },
		"SSO_GOOGLE_ISSUER harus":         func(c *Config) { c.Issuer = "bukan-url" },
		"SSO_GOOGLE_HOSTED_DOMAIN harus":  func(c *Config) { c.HostedDomain = "@kemenkeu.go.id" },
	}
	for want, mutate := range cases {
		cfg := ok
		mutate(&cfg)
		err := cfg.Validate()
		require.Error(t, err, want)
		assert.Contains(t, err.Error(), want)
		assert.NotContains(t, err.Error(), "rahasia", "galat tidak boleh memuat nilai rahasia")
	}
	_, err := New(Config{})
	assert.Error(t, err, "New menolak konfigurasi tidak valid")
}

// ── URL otorisasi ────────────────────────────────────────────────────────

func TestAuthCodeURLMemuatParameterWajibDanPKCE(t *testing.T) {
	f := newFakeIdP(t)
	p := f.provider()
	verifier := NewCodeVerifier()

	raw, err := p.AuthCodeURL(context.Background(), "state-1", "nonce-1", verifier)
	require.NoError(t, err)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, f.srv.URL+"/auth", u.Scheme+"://"+u.Host+u.Path)

	q := u.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, testClientID, q.Get("client_id"))
	assert.Equal(t, testRedirectURL, q.Get("redirect_uri"))
	assert.Equal(t, "state-1", q.Get("state"))
	assert.Equal(t, "nonce-1", q.Get("nonce"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	sum := sha256.Sum256([]byte(verifier))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), q.Get("code_challenge"), "tantangan = BASE64URL(SHA256(verifier))")
	assert.NotContains(t, raw, verifier, "verifier tidak boleh bocor ke URL otorisasi")
	assert.ElementsMatch(t, []string{"openid", "email", "profile"}, strings.Fields(q.Get("scope")))
	assert.Empty(t, q.Get("hd"), "tanpa domain terbatas tidak ada petunjuk hd")
}

func TestAuthCodeURLMenyertakanPetunjukDomain(t *testing.T) {
	f := newFakeIdP(t)
	cfg := f.config()
	cfg.HostedDomain = "kemenkeu.go.id"
	p, err := New(cfg)
	require.NoError(t, err)
	raw, err := p.AuthCodeURL(context.Background(), "s", "n", NewCodeVerifier())
	require.NoError(t, err)
	u, _ := url.Parse(raw)
	assert.Equal(t, "kemenkeu.go.id", u.Query().Get("hd"))
}

// ── Penukaran kode dan verifikasi ID token ───────────────────────────────

func TestExchangeBerhasilMemetakanIdentitasDanMengirimPKCE(t *testing.T) {
	f := newFakeIdP(t)
	p := f.provider()
	verifier := NewCodeVerifier()

	id, err := p.Exchange(context.Background(), "kode-1", verifier, "nonce-1")
	require.NoError(t, err)
	assert.Equal(t, &Identity{Subject: "google-sub-1", Email: "ani@kemenkeu.go.id", EmailVerified: true, Name: "Ani", HostedDomain: "kemenkeu.go.id"}, id)

	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "authorization_code", f.tokenForm.Get("grant_type"))
	assert.Equal(t, "kode-1", f.tokenForm.Get("code"))
	assert.Equal(t, verifier, f.tokenForm.Get("code_verifier"), "verifier PKCE dikirim ke endpoint token")
	assert.Equal(t, testRedirectURL, f.tokenForm.Get("redirect_uri"))
	creds := f.tokenForm.Get("client_id") + "/" + f.tokenForm.Get("client_secret")
	if f.tokenUser != "" {
		creds = f.tokenUser + "/" + f.tokenPass
	}
	assert.Equal(t, testClientID+"/"+testClientSecret, creds, "kredensial klien dikirim (Basic atau badan)")
}

func TestExchangeEmailVerifiedBerupaString(t *testing.T) {
	f := newFakeIdP(t)
	f.setClaims(func(c map[string]any) { c["email_verified"] = "true" })
	id, err := exchange(t, f.provider())
	require.NoError(t, err)
	assert.True(t, id.EmailVerified)

	f.setClaims(func(c map[string]any) { c["email_verified"] = "false" })
	id, err = exchange(t, f.provider())
	require.NoError(t, err)
	assert.False(t, id.EmailVerified)

	f.setClaims(func(c map[string]any) { delete(c, "email_verified") })
	id, err = exchange(t, f.provider())
	require.NoError(t, err)
	assert.False(t, id.EmailVerified, "tanpa klaim = tidak terverifikasi")
}

func TestExchangeMenolakIDTokenTidakValid(t *testing.T) {
	cases := map[string]func(f *fakeIdP){
		"audiens salah": func(f *fakeIdP) { f.setClaims(func(c map[string]any) { c["aud"] = "klien-lain" }) },
		"penerbit salah": func(f *fakeIdP) {
			f.setClaims(func(c map[string]any) { c["iss"] = "https://penyerang.example" })
		},
		"kedaluwarsa": func(f *fakeIdP) {
			f.setClaims(func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() })
		},
		"tanda tangan kunci lain": func(f *fakeIdP) {
			other, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(f.t, err)
			f.signWith = other
		},
		"alg none": func(f *fakeIdP) {
			f.rawIDToken = func(issuer string) string {
				header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
				raw, _ := json.Marshal(validClaims(issuer))
				return header + "." + base64.RawURLEncoding.EncodeToString(raw) + "."
			}
		},
		"tanpa id_token": func(f *fakeIdP) { f.omitIDToken = true },
		"sub kosong":     func(f *fakeIdP) { f.setClaims(func(c map[string]any) { c["sub"] = "" }) },
		"sub terlalu panjang": func(f *fakeIdP) {
			f.setClaims(func(c map[string]any) { c["sub"] = strings.Repeat("x", 256) })
		},
		"azp bukan klien ini": func(f *fakeIdP) {
			f.setClaims(func(c map[string]any) { c["aud"] = []string{testClientID, "lain"}; c["azp"] = "lain" })
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeIdP(t)
			setup(f)
			id, err := exchange(t, f.provider())
			require.Error(t, err)
			assert.Nil(t, id)
			assert.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

func TestExchangeMenerimaAudiensGandaDenganAzpBenar(t *testing.T) {
	f := newFakeIdP(t)
	f.setClaims(func(c map[string]any) { c["aud"] = []string{testClientID, "lain"}; c["azp"] = testClientID })
	_, err := exchange(t, f.provider())
	assert.NoError(t, err)
}

func TestExchangeMenolakNonceBerbeda(t *testing.T) {
	f := newFakeIdP(t)
	_, err := f.provider().Exchange(context.Background(), "kode", NewCodeVerifier(), "nonce-lain")
	assert.ErrorIs(t, err, ErrNonceMismatch, "ID token dengan nonce alur lain = replay")

	f.setClaims(func(c map[string]any) { delete(c, "nonce") })
	_, err = exchange(t, f.provider())
	assert.ErrorIs(t, err, ErrNonceMismatch, "token tanpa nonce ditolak")
}

func TestExchangeMenolakParameterKosong(t *testing.T) {
	p := newFakeIdP(t).provider()
	for _, args := range [][3]string{{"", "v", "n"}, {"c", "", "n"}, {"c", "v", ""}} {
		_, err := p.Exchange(context.Background(), args[0], args[1], args[2])
		assert.ErrorIs(t, err, ErrInvalidToken)
	}
}

func TestExchangeDomainTerbatas(t *testing.T) {
	f := newFakeIdP(t)
	cfg := f.config()
	cfg.HostedDomain = "kemenkeu.go.id"
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = exchange(t, p)
	assert.NoError(t, err, "hd cocok (tak peka huruf besar-kecil)")

	f.setClaims(func(c map[string]any) { c["hd"] = "lain.go.id" })
	_, err = exchange(t, p)
	assert.ErrorIs(t, err, ErrHostedDomain)

	f.setClaims(func(c map[string]any) { delete(c, "hd") })
	_, err = exchange(t, p)
	assert.ErrorIs(t, err, ErrHostedDomain, "akun pribadi tanpa hd ditolak bila domain dibatasi")
}

func TestExchangeGalatEndpointToken(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"kode ditolak", http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Bad Request"}`, ErrCodeRejected},
		{"klien salah", http.StatusUnauthorized, `{"error":"invalid_client"}`, ErrUnavailable},
		{"galat server", http.StatusBadGateway, `bad gateway`, ErrUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeIdP(t)
			f.tokenStatus, f.tokenBody = tc.status, tc.body
			_, err := exchange(t, f.provider())
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// ── Penemuan malas dengan ulang ──────────────────────────────────────────

func TestPenemuanMalasTidakMenghubungiPenyediaSaatNew(t *testing.T) {
	f := newFakeIdP(t)
	f.down.Store(true)
	_, err := New(f.config())
	require.NoError(t, err, "New tidak boleh gagal walau penyedia mati")
	assert.EqualValues(t, 0, f.discoveries.Load())
}

func TestPenemuanDiulangDenganJeda(t *testing.T) {
	f := newFakeIdP(t)
	now := time.Now()
	clock := &now
	cfg := f.config()
	cfg.RetryAfter = time.Minute
	cfg.Now = func() time.Time { return *clock }
	p, err := New(cfg)
	require.NoError(t, err)

	f.down.Store(true)
	_, err = p.AuthCodeURL(context.Background(), "s", "n", NewCodeVerifier())
	require.ErrorIs(t, err, ErrUnavailable)

	// Penyedia sudah pulih, tetapi jeda ulang belum lewat: tidak ada permintaan baru.
	f.down.Store(false)
	_, err = p.AuthCodeURL(context.Background(), "s", "n", NewCodeVerifier())
	require.ErrorIs(t, err, ErrUnavailable)
	assert.EqualValues(t, 0, f.discoveries.Load(), "tidak menghubungi penyedia sebelum jeda ulang lewat")

	*clock = clock.Add(2 * time.Minute)
	raw, err := p.AuthCodeURL(context.Background(), "s", "n", NewCodeVerifier())
	require.NoError(t, err, "penemuan berhasil setelah jeda")
	assert.NotEmpty(t, raw)

	// Berhasil sekali = tersimpan; tidak ditemukan ulang.
	_, err = p.AuthCodeURL(context.Background(), "s", "n", NewCodeVerifier())
	require.NoError(t, err)
	assert.EqualValues(t, 1, f.discoveries.Load())
}

func TestPenemuanPenerbitTidakCocokDitolak(t *testing.T) {
	f := newFakeIdP(t)
	cfg := f.config()
	cfg.Issuer = f.srv.URL + "/" // beda dengan penerbit yang diiklankan penyedia
	p, err := New(cfg)
	require.NoError(t, err)
	_, err = p.AuthCodeURL(context.Background(), "s", "n", NewCodeVerifier())
	assert.ErrorIs(t, err, ErrUnavailable)
}

// ── Pembangkit acak ──────────────────────────────────────────────────────

func TestRandomTokenUnikDanPanjangTepat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := RandomToken(32)
		require.NoError(t, err)
		assert.Len(t, tok, 43, "32 byte = 43 karakter base64url tanpa padding")
		assert.Regexp(t, `^[A-Za-z0-9_-]+$`, tok)
		assert.False(t, seen[tok], "token acak tidak boleh berulang")
		seen[tok] = true
	}
}

func TestNewCodeVerifierMemenuhiRFC7636(t *testing.T) {
	v := NewCodeVerifier()
	assert.GreaterOrEqual(t, len(v), 43)
	assert.LessOrEqual(t, len(v), 128)
	assert.Regexp(t, `^[A-Za-z0-9._~-]+$`, v)
	assert.NotEqual(t, v, NewCodeVerifier())
}
