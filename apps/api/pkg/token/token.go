package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT claims
type Claims struct {
	UserID      string   `json:"user_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// Manager handles JWT token operations
type Manager struct {
	secret      []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
	rememberTTL time.Duration
	issuer      string
}

// NewManager creates a new token manager
func NewManager(secret string, opts ...Option) *Manager {
	m := &Manager{
		secret:      []byte(secret),
		accessTTL:   15 * time.Minute,
		refreshTTL:  24 * time.Hour,
		rememberTTL: 30 * 24 * time.Hour,
		issuer:      "garda-inflasi",
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Option configures the token manager
type Option func(*Manager)

// WithAccessTTL sets the access token TTL
func WithAccessTTL(ttl time.Duration) Option {
	return func(m *Manager) {
		m.accessTTL = ttl
	}
}

// WithRefreshTTL sets the refresh token TTL
func WithRefreshTTL(ttl time.Duration) Option {
	return func(m *Manager) {
		m.refreshTTL = ttl
	}
}

// WithRememberTTL sets the remember-me token TTL
func WithRememberTTL(ttl time.Duration) Option {
	return func(m *Manager) {
		m.rememberTTL = ttl
	}
}

// CreateAccessToken creates a new access token
func (m *Manager) CreateAccessToken(userID string, role string, permissions []string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:      userID,
		Role:        role,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    m.issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// VerifyToken verifies a JWT token and returns the claims
func (m *Manager) VerifyToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("unexpected signing algorithm")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// GetAccessTTL returns the access token TTL
func (m *Manager) GetAccessTTL() time.Duration {
	return m.accessTTL
}

// GetRefreshTTL returns the refresh token TTL
func (m *Manager) GetRefreshTTL() time.Duration {
	return m.refreshTTL
}

// GetRememberTTL returns the remember-me token TTL
func (m *Manager) GetRememberTTL() time.Duration {
	return m.rememberTTL
}
