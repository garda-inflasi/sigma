// Package sso adalah klien OpenID Connect (Authorization Code + PKCE S256) untuk masuk lewat penyedia
// identitas seperti Google. Paket ini hanya membuktikan identitas: membangun URL otorisasi, menukar kode,
// dan memverifikasi ID token (tanda tangan lewat JWKS, penerbit, audiens, kedaluwarsa, nonce). Keputusan
// akun (pemetaan, penautan, MFA) ada di usecase auth.
//
// Penemuan konfigurasi penyedia (/.well-known/openid-configuration) dilakukan malas pada pemakaian pertama
// dan diulang dengan jeda bila gagal, sehingga API tetap dapat start walau penyedia sedang tidak terjangkau.
package sso

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	// ProviderGoogle adalah pengenal penyedia untuk Google (disimpan di pengguna.sso_provider).
	ProviderGoogle = "google"
	// DefaultGoogleIssuer adalah penerbit Google bila SSO_GOOGLE_ISSUER kosong.
	DefaultGoogleIssuer = "https://accounts.google.com"

	// DefaultRetryAfter adalah jeda minimum antar upaya penemuan yang gagal.
	DefaultRetryAfter = 5 * time.Second
	// DefaultHTTPTimeout membatasi setiap permintaan ke penyedia (penemuan, JWKS, penukaran kode).
	DefaultHTTPTimeout = 10 * time.Second

	maxSubjectLen = 255
)

var (
	// ErrNotConfigured: SSO belum dikonfigurasi.
	ErrNotConfigured = errors.New("sso: belum dikonfigurasi")
	// ErrUnavailable: penyedia identitas tidak dapat dihubungi atau menjawab galat server.
	ErrUnavailable = errors.New("sso: penyedia identitas tidak dapat dihubungi")
	// ErrCodeRejected: penyedia menolak kode otorisasi (kedaluwarsa, sudah dipakai, atau PKCE tidak cocok).
	ErrCodeRejected = errors.New("sso: kode otorisasi ditolak penyedia")
	// ErrInvalidToken: ID token tidak lolos verifikasi (tanda tangan, penerbit, audiens, kedaluwarsa, klaim).
	ErrInvalidToken = errors.New("sso: ID token tidak valid")
	// ErrNonceMismatch: nonce di ID token tidak sama dengan nonce alur ini (indikasi replay).
	ErrNonceMismatch = errors.New("sso: nonce tidak cocok")
	// ErrHostedDomain: akun berasal dari domain yang tidak diizinkan (klaim hd).
	ErrHostedDomain = errors.New("sso: domain akun tidak diizinkan")
)

// Config adalah konfigurasi satu penyedia OIDC.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	// RedirectURL adalah alamat callback terdaftar di penyedia (rute BFF /api/auth/sso/google/callback).
	RedirectURL string
	// HostedDomain, bila diisi, membatasi masuk pada akun Google Workspace dengan klaim hd tersebut.
	HostedDomain string
	// Scopes bawaan: openid email profile.
	Scopes []string
	// HTTPClient bawaan: Timeout DefaultHTTPTimeout. Tes menyuntikkan klien lain.
	HTTPClient *http.Client
	// RetryAfter adalah jeda minimum antar penemuan yang gagal (bawaan DefaultRetryAfter).
	RetryAfter time.Duration
	// Now menggantikan time.Now untuk pemeriksaan kedaluwarsa dan jeda ulang (tes).
	Now func() time.Time
}

// ConfigFromEnv membaca SSO_GOOGLE_CLIENT_ID, SSO_GOOGLE_CLIENT_SECRET, SSO_GOOGLE_ISSUER (bawaan Google),
// SSO_GOOGLE_REDIRECT_URL, dan SSO_GOOGLE_HOSTED_DOMAIN (opsional).
func ConfigFromEnv(getenv func(string) string) Config {
	issuer := strings.TrimSpace(getenv("SSO_GOOGLE_ISSUER"))
	if issuer == "" {
		issuer = DefaultGoogleIssuer
	}
	return Config{
		Issuer:       issuer,
		ClientID:     strings.TrimSpace(getenv("SSO_GOOGLE_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(getenv("SSO_GOOGLE_CLIENT_SECRET")),
		RedirectURL:  strings.TrimSpace(getenv("SSO_GOOGLE_REDIRECT_URL")),
		HostedDomain: strings.ToLower(strings.TrimSpace(getenv("SSO_GOOGLE_HOSTED_DOMAIN"))),
	}
}

// Enabled melaporkan apakah operator berniat menyalakan SSO (client id terisi). Konfigurasi yang
// tidak lengkap tetap Enabled tetapi gagal di Validate.
func (c Config) Enabled() bool { return c.ClientID != "" }

// Validate memeriksa kelengkapan dan bentuk konfigurasi tanpa jaringan.
func (c Config) Validate() error {
	var problems []string
	if c.ClientID == "" {
		problems = append(problems, "SSO_GOOGLE_CLIENT_ID kosong")
	}
	if c.ClientSecret == "" {
		problems = append(problems, "SSO_GOOGLE_CLIENT_SECRET kosong")
	}
	if err := checkURL(c.Issuer); err != nil {
		problems = append(problems, "SSO_GOOGLE_ISSUER "+err.Error())
	}
	if c.RedirectURL == "" {
		problems = append(problems, "SSO_GOOGLE_REDIRECT_URL kosong")
	} else if err := checkURL(c.RedirectURL); err != nil {
		problems = append(problems, "SSO_GOOGLE_REDIRECT_URL "+err.Error())
	}
	if c.HostedDomain != "" && (strings.ContainsAny(c.HostedDomain, "@/: ") || !strings.Contains(c.HostedDomain, ".")) {
		problems = append(problems, "SSO_GOOGLE_HOSTED_DOMAIN harus berupa nama domain (mis. kemenkeu.go.id)")
	}
	if len(problems) > 0 {
		return errors.New("konfigurasi SSO Google tidak valid: " + strings.Join(problems, "; "))
	}
	return nil
}

func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.Fragment != "" {
		return errors.New("harus berupa URL http(s) absolut tanpa fragmen")
	}
	return nil
}

// Identity adalah identitas terverifikasi dari ID token.
type Identity struct {
	// Subject adalah klaim sub: pengenal stabil akun di penyedia (bukan email).
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	// HostedDomain adalah klaim hd (domain Workspace), kosong untuk akun pribadi.
	HostedDomain string
}

// Provider adalah klien OIDC satu penyedia. Aman dipakai bersamaan.
type Provider struct {
	cfg    Config
	client *http.Client
	now    func() time.Time

	mu       sync.Mutex
	disc     *discovered
	failedAt time.Time
	failErr  error
}

type discovered struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// New membuat Provider tanpa menghubungi penyedia.
func New(cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	if cfg.RetryAfter <= 0 {
		cfg.RetryAfter = DefaultRetryAfter
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Provider{cfg: cfg, client: client, now: now}, nil
}

// Issuer mengembalikan penerbit yang dikonfigurasi.
func (p *Provider) Issuer() string { return p.cfg.Issuer }

// ensure melakukan penemuan bila belum berhasil. Setelah gagal, upaya berikutnya baru dilakukan setelah
// RetryAfter supaya penyedia yang mati tidak dibanjiri permintaan.
func (p *Provider) ensure(ctx context.Context) (*discovered, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.disc != nil {
		return p.disc, nil
	}
	if !p.failedAt.IsZero() && p.now().Sub(p.failedAt) < p.cfg.RetryAfter {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, p.failErr)
	}

	oidcProvider, err := oidc.NewProvider(oidc.ClientContext(ctx, p.client), p.cfg.Issuer)
	if err != nil {
		p.failedAt, p.failErr = p.now(), err
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	p.failedAt, p.failErr = time.Time{}, nil
	p.disc = &discovered{
		oauth: &oauth2.Config{
			ClientID:     p.cfg.ClientID,
			ClientSecret: p.cfg.ClientSecret,
			RedirectURL:  p.cfg.RedirectURL,
			Endpoint:     oidcProvider.Endpoint(),
			Scopes:       p.cfg.Scopes,
		},
		verifier: oidcProvider.Verifier(&oidc.Config{ClientID: p.cfg.ClientID, Now: p.now}),
	}
	return p.disc, nil
}

// AuthCodeURL membangun URL otorisasi: response_type=code, state, nonce, dan tantangan PKCE S256 dari verifier.
func (p *Provider) AuthCodeURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	d, err := p.ensure(ctx)
	if err != nil {
		return "", err
	}
	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce)}
	if p.cfg.HostedDomain != "" {
		// Hanya petunjuk tampilan bagi Google; penegakan sebenarnya ada pada pemeriksaan klaim hd di Exchange.
		opts = append(opts, oauth2.SetAuthURLParam("hd", p.cfg.HostedDomain))
	}
	return d.oauth.AuthCodeURL(state, opts...), nil
}

// idClaims adalah klaim ID token yang dipakai untuk memetakan akun.
type idClaims struct {
	Email         string   `json:"email"`
	EmailVerified flexBool `json:"email_verified"`
	Name          string   `json:"name"`
	HostedDomain  string   `json:"hd"`
	AuthorizedP   string   `json:"azp"`
}

// flexBool menerima boolean JSON maupun string "true"/"false" (sebagian penyedia mengirim string).
type flexBool bool

func (b *flexBool) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		*b = flexBool(strings.EqualFold(s, "true"))
		return nil
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	*b = flexBool(v)
	return nil
}

// Exchange menukar kode otorisasi (dengan code_verifier PKCE) lalu memverifikasi ID token terhadap
// nonce alur ini. Galat dikelompokkan: ErrUnavailable (jaringan atau galat server penyedia),
// ErrCodeRejected (kode ditolak), ErrNonceMismatch, ErrHostedDomain, selain itu ErrInvalidToken.
func (p *Provider) Exchange(ctx context.Context, code, verifier, nonce string) (*Identity, error) {
	if code == "" || verifier == "" || nonce == "" {
		return nil, fmt.Errorf("%w: kode, verifier, dan nonce wajib", ErrInvalidToken)
	}
	d, err := p.ensure(ctx)
	if err != nil {
		return nil, err
	}

	ctx = oidc.ClientContext(ctx, p.client)
	token, err := d.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		var retrieve *oauth2.RetrieveError
		if errors.As(err, &retrieve) && retrieve.Response != nil && retrieve.Response.StatusCode < 500 {
			// Klien ditolak = kesalahan konfigurasi (id/rahasia salah), bukan kesalahan pengguna.
			if retrieve.ErrorCode == "invalid_client" || retrieve.ErrorCode == "unauthorized_client" {
				return nil, fmt.Errorf("%w: klien ditolak penyedia (periksa client id dan rahasia): %v", ErrUnavailable, err)
			}
			return nil, fmt.Errorf("%w: %v", ErrCodeRejected, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	rawID, _ := token.Extra("id_token").(string)
	if rawID == "" {
		return nil, fmt.Errorf("%w: respons token tanpa id_token", ErrInvalidToken)
	}

	idToken, err := d.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return nil, ErrNonceMismatch
	}

	var claims idClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("%w: klaim tidak terbaca: %v", ErrInvalidToken, err)
	}
	// Dengan beberapa audiens, azp harus klien ini (OIDC Core 3.1.3.7 poin 4-5).
	if len(idToken.Audience) > 1 && claims.AuthorizedP != p.cfg.ClientID {
		return nil, fmt.Errorf("%w: azp tidak sesuai", ErrInvalidToken)
	}
	if idToken.Subject == "" || len(idToken.Subject) > maxSubjectLen {
		return nil, fmt.Errorf("%w: sub kosong atau terlalu panjang", ErrInvalidToken)
	}
	if p.cfg.HostedDomain != "" && !strings.EqualFold(claims.HostedDomain, p.cfg.HostedDomain) {
		return nil, ErrHostedDomain
	}
	return &Identity{
		Subject:       idToken.Subject,
		Email:         strings.TrimSpace(claims.Email),
		EmailVerified: bool(claims.EmailVerified),
		Name:          claims.Name,
		HostedDomain:  claims.HostedDomain,
	}, nil
}

// RandomToken menghasilkan n byte acak kriptografis sebagai base64url tanpa padding (untuk state, nonce, binding).
func RandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("acak: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// NewCodeVerifier menghasilkan code_verifier PKCE (RFC 7636, 43 karakter dari 32 byte acak).
func NewCodeVerifier() string { return oauth2.GenerateVerifier() }
