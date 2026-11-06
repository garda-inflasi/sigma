package token

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubResolver struct {
	izin   []string
	err    error
	gotUID string
}

func (s *stubResolver) Permissions(_ context.Context, userID string) ([]string, error) {
	s.gotUID = userID
	return s.izin, s.err
}

func TestResolvePermissions_DelegatesUserIDToResolver(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	stub := &stubResolver{izin: []string{"kpi.lihat"}}
	m.SetPermissionResolver(stub)

	perms, resolved, err := m.ResolvePermissions(context.Background(), &Claims{UserID: "u1", Permissions: []string{"lama"}})

	require.NoError(t, err)
	assert.True(t, resolved)
	assert.Equal(t, []string{"kpi.lihat"}, perms, "jawaban resolver, bukan klaim token")
	assert.Equal(t, "u1", stub.gotUID)
}

func TestResolvePermissions_PropagatesErrors(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	m.SetPermissionResolver(&stubResolver{err: ErrPrincipalNotFound})

	_, resolved, err := m.ResolvePermissions(context.Background(), &Claims{UserID: "u1"})

	assert.ErrorIs(t, err, ErrPrincipalNotFound)
	assert.True(t, resolved)

	m.SetPermissionResolver(&stubResolver{err: errors.New("db down")})
	_, _, err = m.ResolvePermissions(context.Background(), &Claims{UserID: "u1"})
	assert.Error(t, err)
}

func TestResolvePermissions_NotResolvedWithoutResolverOrClaims(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")

	perms, resolved, err := m.ResolvePermissions(context.Background(), &Claims{UserID: "u1"})
	require.NoError(t, err)
	assert.False(t, resolved)
	assert.Nil(t, perms)

	m.SetPermissionResolver(&stubResolver{})
	_, resolved, err = m.ResolvePermissions(context.Background(), nil)
	require.NoError(t, err)
	assert.False(t, resolved, "tanpa klaim tidak ada yang dipecahkan")

	m.SetPermissionResolver(nil)
	_, resolved, _ = m.ResolvePermissions(context.Background(), &Claims{UserID: "u1"})
	assert.False(t, resolved, "nil melepas resolver")
}

func TestSamePermissions(t *testing.T) {
	assert.True(t, SamePermissions(nil, nil))
	assert.True(t, SamePermissions(nil, []string{}))
	assert.True(t, SamePermissions([]string{"a.b", "c.d"}, []string{"c.d", "a.b"}), "urutan tidak berarti")
	assert.True(t, SamePermissions([]string{"a.b", "a.b"}, []string{"a.b"}), "duplikat tidak berarti")
	assert.False(t, SamePermissions([]string{"a.b"}, []string{"a.b", "c.d"}))
	assert.False(t, SamePermissions([]string{"a.b", "c.d"}, []string{"a.b"}))
	assert.False(t, SamePermissions([]string{"a.b"}, nil))
	assert.False(t, SamePermissions([]string{"a.b"}, []string{"a.c"}))

	asli := []string{"z.z", "a.a"}
	SamePermissions(asli, []string{"a.a", "z.z"})
	assert.Equal(t, []string{"z.z", "a.a"}, asli, "masukan tidak diubah")
}
