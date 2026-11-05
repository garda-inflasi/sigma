package token

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubChecker struct {
	revoked bool
	err     error
	gotJTI  string
	gotSID  string
}

func (s *stubChecker) Revoked(_ context.Context, jti, sid string) (bool, error) {
	s.gotJTI, s.gotSID = jti, sid
	return s.revoked, s.err
}

func TestWithSession_PutsSidInClaims(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	tokenStr, err := m.CreateAccessToken("u1", "ADMIN_SISTEM", nil, WithSession("sesi-1"))
	require.NoError(t, err)

	claims, err := m.VerifyToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, "sesi-1", claims.SessionID)
}

func TestAccessTokenWithoutSessionHasNoSidClaim(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	tokenStr, err := m.CreateAccessToken("u1", "ADMIN_SISTEM", nil)
	require.NoError(t, err)

	claims, err := m.VerifyToken(tokenStr)
	require.NoError(t, err)
	assert.Empty(t, claims.SessionID)
}

func TestIsRevoked_DelegatesJTIAndSessionToChecker(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	stub := &stubChecker{revoked: true}
	m.SetRevocationChecker(stub)
	claims := &Claims{SessionID: "sesi-1"}
	claims.ID = "jti-1"

	revoked, err := m.IsRevoked(context.Background(), claims)

	require.NoError(t, err)
	assert.True(t, revoked)
	assert.Equal(t, "jti-1", stub.gotJTI)
	assert.Equal(t, "sesi-1", stub.gotSID)
}

func TestIsRevoked_PropagatesCheckerError(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	m.SetRevocationChecker(&stubChecker{err: errors.New("db down")})

	_, err := m.IsRevoked(context.Background(), &Claims{})

	assert.Error(t, err)
}

func TestIsRevoked_NothingRevokedWithoutChecker(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")

	revoked, err := m.IsRevoked(context.Background(), &Claims{SessionID: "x"})

	require.NoError(t, err)
	assert.False(t, revoked)
}
