package token

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubGuard struct {
	izin    []string
	err     error
	gotID   string
	mutasi  []MutationInfo
	mutasiC []ImpersonationContext
}

func (s *stubGuard) ValidateImpersonation(_ context.Context, claims *Claims) ([]string, error) {
	s.gotID = claims.Imp.ID
	return s.izin, s.err
}

func (s *stubGuard) RecordImpersonationMutation(_ context.Context, ic ImpersonationContext, info MutationInfo) {
	s.mutasi = append(s.mutasi, info)
	s.mutasiC = append(s.mutasiC, ic)
}

func contohImp() Impersonation {
	return Impersonation{ID: uid("imp-1"), By: ImpersonatedBy{UserID: uid("admin"), Name: "Super Admin", Email: "a@x.id", Role: "ADMIN_SISTEM"}, StartedAt: 1763370000}
}

func TestImpersonation_KlaimImpMelewatiTandaTanganDanVerifikasi(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	str, err := m.CreateAccessToken(uid("sasaran"), "OPS_MAHASISWA", []string{"harga.input"},
		WithSession(uid("sid")), WithImpersonation(contohImp()))
	require.NoError(t, err)

	claims, err := m.VerifyToken(str)
	require.NoError(t, err)
	assert.Equal(t, uid("sasaran"), claims.UserID, "sub dan user_id milik sasaran")
	assert.Equal(t, "OPS_MAHASISWA", claims.Role)
	require.NotNil(t, claims.Imp)
	assert.Equal(t, contohImp(), *claims.Imp)

	biasa, err := m.CreateAccessToken(uid("u"), "OPS_MAHASISWA", nil)
	require.NoError(t, err)
	claims, err = m.VerifyToken(biasa)
	require.NoError(t, err)
	assert.Nil(t, claims.Imp, "token biasa tidak membawa klaim imp")
}

func TestResolvePermissions_TokenImpersonasiGagalTutup(t *testing.T) {
	m := NewManager("test-secret-key-32-chars!!")
	m.SetPermissionResolver(&stubResolver{izin: []string{"izin.pelaku"}})
	claims := &Claims{UserID: uid("sasaran"), Imp: func() *Impersonation { i := contohImp(); return &i }()}

	// Tanpa penjaga terpasang: ditolak, tidak jatuh ke resolver biasa (yang akan menjawab izin akun sasaran tanpa cek sesi).
	_, resolved, err := m.ResolvePermissions(context.Background(), claims)
	assert.True(t, resolved)
	assert.ErrorIs(t, err, ErrImpersonationEnded)

	// Klaim cacat ditolak walau penjaga terpasang, dan penjaga tidak dipanggil.
	g := &stubGuard{izin: []string{"harga.input"}}
	m.SetImpersonationGuard(g)
	for nama, imp := range map[string]*Impersonation{
		"tanpa id":     {By: ImpersonatedBy{UserID: uid("admin")}},
		"tanpa pelaku": {ID: uid("imp-1")},
	} {
		_, _, err = m.ResolvePermissions(context.Background(), &Claims{UserID: uid("sasaran"), Imp: imp})
		assert.ErrorIs(t, err, ErrImpersonationEnded, nama)
	}
	assert.Empty(t, g.gotID)

	// Penjaga terpasang: izin sasaran dari penjaga, galat diteruskan apa adanya.
	perms, resolved, err := m.ResolvePermissions(context.Background(), claims)
	require.NoError(t, err)
	assert.True(t, resolved)
	assert.Equal(t, []string{"harga.input"}, perms)
	assert.Equal(t, uid("imp-1"), g.gotID)

	g.err = ErrPrincipalNotFound
	_, _, err = m.ResolvePermissions(context.Background(), claims)
	assert.True(t, errors.Is(err, ErrPrincipalNotFound))

	m.SetImpersonationGuard(nil)
	assert.Nil(t, m.ImpersonationGuard())
}

func TestImpersonationContext_BolakBalik(t *testing.T) {
	_, ok := ImpersonationFrom(context.Background())
	assert.False(t, ok)

	ic := ImpersonationContext{Impersonation: contohImp(), EffectiveUserID: uid("sasaran"), EffectiveRole: "OPS_MAHASISWA"}
	got, ok := ImpersonationFrom(ContextWithImpersonation(context.Background(), ic))
	require.True(t, ok)
	assert.Equal(t, ic, got)
}

func TestValidImpersonationClaim(t *testing.T) {
	assert.False(t, ValidImpersonationClaim(nil))
	assert.False(t, ValidImpersonationClaim(&Impersonation{}))
	assert.False(t, ValidImpersonationClaim(&Impersonation{ID: "  ", By: ImpersonatedBy{UserID: "x"}}))
	i := contohImp()
	assert.True(t, ValidImpersonationClaim(&i))
}
