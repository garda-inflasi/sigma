package token

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ── Constants ─────────────────────────────────────────────────────────

const (
	issuer   = "garda-auth"
	audience = "garda-api"
	// ClockSkew is the allowed clock skew for JWT validation (30 seconds).
	ClockSkew = 30 * time.Second
	// mfaAudience memisahkan token tantangan MFA dari access token: VerifyToken menolaknya karena audiens beda.
	mfaAudience = "garda-mfa"
	// MFAChallengeTTL adalah masa berlaku token tantangan MFA setelah kata sandi benar.
	MFAChallengeTTL = 5 * time.Minute
)

// ── Claims ────────────────────────────────────────────────────────────

// Claims represents JWT claims with all required fields.
type Claims struct {
	UserID      string   `json:"user_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	// PasswordChangeRequired menandai sesi akun yang wajib mengganti kata sandi;
	// middleware menolak semua endpoint di luar daftar kecil selama klaim ini aktif.
	PasswordChangeRequired bool `json:"wajib_ganti_password,omitempty"`
	// MFASetupRequired menandai sesi akun yang wajib mengaktifkan MFA tetapi belum; middleware hanya
	// membuka endpoint setup MFA, keluar, dan profil selama klaim ini aktif.
	MFASetupRequired bool `json:"wajib_setup_mfa,omitempty"`
	// SessionID (`sid`) adalah id sesi tempat token ini terbit (family refresh token). Access token mati
	// seketika saat sesinya dicabut; lihat RevocationChecker. Kosong pada token sebelum fitur sesi ada.
	SessionID string `json:"sid,omitempty"`
	// Profil untuk tampilan (nama, email, lembaga, kabupaten tugas); dibaca sesi server web.
	// Otorisasi tidak pernah bergantung pada klaim ini.
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Org   string `json:"org,omitempty"`
	Kab   string `json:"kab,omitempty"`
	jwt.RegisteredClaims
}

// ── Manager ───────────────────────────────────────────────────────────

// Manager handles JWT token operations.
type Manager struct {
	secret      []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
	rememberTTL time.Duration
	issuer      string
	audience    string
	revocation  atomic.Pointer[RevocationChecker]
}

// NewManager creates a new token manager.
func NewManager(secret string, opts ...Option) *Manager {
	m := &Manager{
		secret:      []byte(secret),
		accessTTL:   15 * time.Minute,
		refreshTTL:  24 * time.Hour,
		rememberTTL: 30 * 24 * time.Hour,
		issuer:      issuer,
		audience:    audience,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// ── Options ───────────────────────────────────────────────────────────

// Option configures the token manager.
type Option func(*Manager)

// WithAccessTTL sets the access token TTL.
func WithAccessTTL(ttl time.Duration) Option {
	return func(m *Manager) {
		m.accessTTL = ttl
	}
}

// WithRefreshTTL sets the refresh token TTL.
func WithRefreshTTL(ttl time.Duration) Option {
	return func(m *Manager) {
		m.refreshTTL = ttl
	}
}

// WithRememberTTL sets the remember-me token TTL.
func WithRememberTTL(ttl time.Duration) Option {
	return func(m *Manager) {
		m.rememberTTL = ttl
	}
}

// WithIssuer sets a custom issuer.
func WithIssuer(iss string) Option {
	return func(m *Manager) {
		m.issuer = iss
	}
}

// WithAudience sets a custom audience.
func WithAudience(aud string) Option {
	return func(m *Manager) {
		m.audience = aud
	}
}

// ── Token Creation ────────────────────────────────────────────────────

// ClaimOption mengatur klaim tambahan pada access token.
type ClaimOption func(*Claims)

// WithPasswordChangeRequired menandai token milik akun yang wajib mengganti kata sandi.
func WithPasswordChangeRequired(required bool) ClaimOption {
	return func(c *Claims) {
		c.PasswordChangeRequired = required
	}
}

// WithMFASetupRequired menandai token milik akun yang wajib mengaktifkan MFA.
func WithMFASetupRequired(required bool) ClaimOption {
	return func(c *Claims) {
		c.MFASetupRequired = required
	}
}

// WithProfile mengisi klaim profil tampilan: nama, email, lembaga, dan kode kabupaten tugas
// (kosong untuk pengguna tingkat provinsi).
func WithProfile(nama, email, lembaga, kabupatenKode string) ClaimOption {
	return func(c *Claims) {
		c.Name, c.Email, c.Org, c.Kab = nama, email, lembaga, kabupatenKode
	}
}

// WithSession mengikat access token ke sesi (family refresh token) sumbernya lewat klaim `sid`.
func WithSession(sessionID string) ClaimOption {
	return func(c *Claims) {
		c.SessionID = sessionID
	}
}

// CreateMFAChallengeToken menerbitkan token tantangan MFA berumur pendek: bukti bahwa kata sandi sudah
// benar, hanya berguna untuk menukar kode OTP dengan sesi. Bukan access token (audiens berbeda).
func (m *Manager) CreateMFAChallengeToken(userID string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   userID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{mfaAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-ClockSkew)),
			ExpiresAt: jwt.NewNumericDate(now.Add(MFAChallengeTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// VerifyMFAChallengeToken memeriksa token tantangan MFA dan mengembalikan id pengguna.
func (m *Manager) VerifyMFAChallengeToken(tokenString string) (string, error) {
	claims, err := m.parse(tokenString, mfaAudience)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// CreateAccessToken creates a new signed JWT access token.
// Includes jti (unique ID), sub (user ID), role, permissions, iss, aud, iat, exp, nbf.
func (m *Manager) CreateAccessToken(userID string, role string, permissions []string, opts ...ClaimOption) (string, error) {
	now := time.Now()
	jti := uuid.New().String()

	claims := &Claims{
		UserID:      userID,
		Role:        role,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-ClockSkew)),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	for _, opt := range opts {
		opt(claims)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// CreateRefreshToken creates an opaque refresh token (random string)
// and returns both the raw token and its SHA-256 hash.
func (m *Manager) CreateRefreshToken() (rawToken string, tokenHash string, err error) {
	raw := uuid.New().String() + uuid.New().String()
	hash := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(hash[:]), nil
}

// ── Token Verification ────────────────────────────────────────────────

// VerifyToken verifies a JWT token string and returns the claims.
// Validates: signature, algorithm (HS256 only), issuer, audience, expiry, not-before.
func (m *Manager) VerifyToken(tokenString string) (*Claims, error) {
	return m.parse(tokenString, m.audience)
}

// parse memeriksa tanda tangan, algoritma, penerbit, audiens, kedaluwarsa, dan nbf.
func (m *Manager) parse(tokenString, audience string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Enforce algorithm — reject "none" and any non-HMAC algorithm
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method: " + token.Method.Alg())
		}
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing algorithm: " + token.Method.Alg())
		}
		return m.secret, nil
	},
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(audience),
		jwt.WithLeeway(ClockSkew),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// ── Revocation ────────────────────────────────────────────────────────

// RevocationChecker menjawab apakah access token yang tanda tangannya sah sudah dicabut: jti masuk daftar
// cabut (logout), atau sesinya (sid) tidak lagi hidup (dicabut, diganti, atau dipaksa keluar).
// Implementasi harus aman dipakai bersamaan dan murah: dipanggil pada setiap request terotentikasi.
type RevocationChecker interface {
	Revoked(ctx context.Context, jti, sessionID string) (bool, error)
}

// SetRevocationChecker memasang pemeriksa pencabutan yang dipakai middleware JWTAuth. Dipasang sekali saat
// rute auth didaftarkan (internal/auth/routes) pada Manager yang dibagi seluruh modul, sehingga tanda tangan
// JWTAuth(tokenMgr) tidak berubah dan setiap router yang memasang modul auth otomatis ikut menegakkannya.
// Nil melepas pemeriksa.
func (m *Manager) SetRevocationChecker(c RevocationChecker) {
	if c == nil {
		m.revocation.Store(nil)
		return
	}
	m.revocation.Store(&c)
}

// IsRevoked memeriksa klaim yang sudah terverifikasi terhadap pemeriksa pencabutan. Tanpa pemeriksa
// (router yang tidak memasang modul auth, mis. tes modul lain) tidak ada yang dicabut. Galat dikembalikan
// apa adanya; kebijakan gagal-tutup ada di middleware.
func (m *Manager) IsRevoked(ctx context.Context, claims *Claims) (bool, error) {
	checker := m.revocation.Load()
	if checker == nil || claims == nil {
		return false, nil
	}
	return (*checker).Revoked(ctx, claims.ID, claims.SessionID)
}

// ── Getters ───────────────────────────────────────────────────────────

// GetAccessTTL returns the access token TTL.
func (m *Manager) GetAccessTTL() time.Duration {
	return m.accessTTL
}

// GetRefreshTTL returns the refresh token TTL.
func (m *Manager) GetRefreshTTL() time.Duration {
	return m.refreshTTL
}

// GetRememberTTL returns the remember-me token TTL.
func (m *Manager) GetRememberTTL() time.Duration {
	return m.rememberTTL
}

// GetIssuer returns the configured issuer.
func (m *Manager) GetIssuer() string {
	return m.issuer
}

// GetAudience returns the configured audience.
func (m *Manager) GetAudience() string {
	return m.audience
}
