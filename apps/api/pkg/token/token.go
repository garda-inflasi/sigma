package token

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ── Constants ─────────────────────────────────────────────────────────

const (
	issuer  = "garda-auth"
	audience = "garda-api"
	// ClockSkew is the allowed clock skew for JWT validation (30 seconds).
	ClockSkew = 30 * time.Second
)

// ── Claims ────────────────────────────────────────────────────────────

// Claims represents JWT claims with all required fields.
type Claims struct {
	UserID      string   `json:"user_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
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

// CreateAccessToken creates a new signed JWT access token.
// Includes jti (unique ID), sub (user ID), role, permissions, iss, aud, iat, exp, nbf.
func (m *Manager) CreateAccessToken(userID string, role string, permissions []string) (string, error) {
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
		jwt.WithAudience(m.audience),
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
