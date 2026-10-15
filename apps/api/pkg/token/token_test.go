package token

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	require.NotNil(t, m)
	assert.Equal(t, 15*time.Minute, m.accessTTL)
	assert.Equal(t, 24*time.Hour, m.refreshTTL)
	assert.Equal(t, 30*24*time.Hour, m.rememberTTL)
	assert.Equal(t, "garda-auth", m.issuer)
	assert.Equal(t, "garda-api", m.audience)
}

func TestNewManagerWithOptions(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!",
		WithAccessTTL(5*time.Minute),
		WithRefreshTTL(48*time.Hour),
		WithRememberTTL(7*24*time.Hour),
		WithIssuer("custom-issuer"),
		WithAudience("custom-audience"),
	)
	require.NotNil(t, m)
	assert.Equal(t, 5*time.Minute, m.accessTTL)
	assert.Equal(t, 48*time.Hour, m.refreshTTL)
	assert.Equal(t, 7*24*time.Hour, m.rememberTTL)
	assert.Equal(t, "custom-issuer", m.issuer)
	assert.Equal(t, "custom-audience", m.audience)
}

func TestCreateAccessToken(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	tokenStr, err := m.CreateAccessToken("user-123", "ADMIN_SISTEM", []string{"pengguna.kelola", "master.lihat"})
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	// Verify the token
	claims, err := m.VerifyToken(tokenStr)
	require.NoError(t, err)
	require.NotNil(t, claims)

	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "ADMIN_SISTEM", claims.Role)
	assert.Equal(t, []string{"pengguna.kelola", "master.lihat"}, claims.Permissions)
	assert.Equal(t, "garda-auth", claims.Issuer)
	assert.Equal(t, jwt.ClaimStrings{"garda-api"}, claims.Audience)
	assert.NotEmpty(t, claims.ID) // jti
	assert.NotEmpty(t, claims.Subject)
	assert.True(t, claims.ExpiresAt.Time.After(time.Now()))
	assert.True(t, claims.IssuedAt.Time.Before(time.Now().Add(time.Second)))
}

func TestVerifyToken_InvalidSignature(t *testing.T) {
	m1 := NewManager("secret-one-1234567890123456")
	m2 := NewManager("secret-two-1234567890123456")

	tokenStr, err := m1.CreateAccessToken("user-1", "OPS_MAHASISWA", nil)
	require.NoError(t, err)

	_, err = m2.VerifyToken(tokenStr)
	assert.Error(t, err)
}

func TestVerifyToken_Expired(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!", WithAccessTTL(-1*time.Hour))
	tokenStr, err := m.CreateAccessToken("user-1", "OPS_MAHASISWA", nil)
	require.NoError(t, err)

	_, err = m.VerifyToken(tokenStr)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "token is expired")
}

func TestVerifyToken_InvalidAlgorithm(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")

	// Create a token with a different algorithm (HS512)
	claims := &Claims{
		UserID: "user-1",
		Role:   "OPS_MAHASISWA",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti",
			Subject:   "user-1",
			Issuer:    "garda-auth",
			Audience:  jwt.ClaimStrings{"garda-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenStr, err := token.SignedString([]byte("test-secret-key-32-chars!!"))
	require.NoError(t, err)

	_, err = m.VerifyToken(tokenStr)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "signing method")
}

func TestVerifyToken_NoneAlgorithm(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")

	// Create a token with "none" algorithm (alg=none attack)
	claims := &Claims{
		UserID: "user-1",
		Role:   "ADMIN_SISTEM",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-jti",
			Subject:   "user-1",
			Issuer:    "garda-auth",
			Audience:  jwt.ClaimStrings{"garda-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = m.VerifyToken(tokenStr)
	assert.Error(t, err)
}

func TestVerifyToken_WrongIssuer(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!", WithIssuer("wrong-issuer"))
	tokenStr, err := m.CreateAccessToken("user-1", "OPS_MAHASISWA", nil)
	require.NoError(t, err)

	// Verify with default issuer
	m2 := NewManager("test-secret-key-32-chars!!")
	_, err = m2.VerifyToken(tokenStr)
	assert.Error(t, err)
}

func TestCreateRefreshToken(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")

	raw, hash, err := m.CreateRefreshToken()
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	require.NotEmpty(t, hash)

	// Hash should be SHA-256 of raw (64 hex chars)
	assert.Len(t, hash, 64)
	assert.NotEqual(t, raw, hash) // opaque != hash

	// Verify the hash matches using the same sha256Hex function
	h := sha256.Sum256([]byte(raw))
	expectedHash := hex.EncodeToString(h[:])
	assert.Equal(t, expectedHash, hash)
}

func TestCreateRefreshToken_Unique(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")

	raw1, _, _ := m.CreateRefreshToken()
	raw2, _, _ := m.CreateRefreshToken()

	assert.NotEqual(t, raw1, raw2)
}

func TestGetAccessTTL(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!", WithAccessTTL(10*time.Minute))
	assert.Equal(t, 10*time.Minute, m.GetAccessTTL())
}

func TestGetRefreshTTL(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!", WithRefreshTTL(48*time.Hour))
	assert.Equal(t, 48*time.Hour, m.GetRefreshTTL())
}

func TestGetRememberTTL(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!", WithRememberTTL(7*24*time.Hour))
	assert.Equal(t, 7*24*time.Hour, m.GetRememberTTL())
}

func TestGetIssuer(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!", WithIssuer("custom"))
	assert.Equal(t, "custom", m.GetIssuer())
}

func TestGetAudience(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!", WithAudience("custom"))
	assert.Equal(t, "custom", m.GetAudience())
}

func TestVerifyToken_ClockSkew(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!", WithAccessTTL(15*time.Second))
	tokenStr, err := m.CreateAccessToken("user-1", "OPS_MAHASISWA", nil)
	require.NoError(t, err)

	// Should still verify within clock skew
	claims, err := m.VerifyToken(tokenStr)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "user-1", claims.UserID)
}

func TestVerifyToken_EmptyToken(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	_, err := m.VerifyToken("")
	assert.Error(t, err)
}

func TestVerifyToken_MalformedToken(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	_, err := m.VerifyToken("not-a-jwt-token")
	assert.Error(t, err)
}

func TestCreateAccessToken_EmptyPermissions(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	tokenStr, err := m.CreateAccessToken("user-1", "OPS_MAHASISWA", []string{})
	require.NoError(t, err)

	claims, err := m.VerifyToken(tokenStr)
	require.NoError(t, err)
	assert.Empty(t, claims.Permissions)
}

func TestCreateAccessToken_NilPermissions(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	tokenStr, err := m.CreateAccessToken("user-1", "OPS_MAHASISWA", nil)
	require.NoError(t, err)

	claims, err := m.VerifyToken(tokenStr)
	require.NoError(t, err)
	assert.Nil(t, claims.Permissions)
}
