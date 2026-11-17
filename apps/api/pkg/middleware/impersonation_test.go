package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/pkg/token"
)

// guardTiruan menjawab validasi impersonasi dan merekam mutasi yang dicatat JWTAuth.
type guardTiruan struct {
	izin    []string
	err     error
	mutasi  []token.MutationInfo
	konteks []token.ImpersonationContext
}

func (g *guardTiruan) ValidateImpersonation(_ context.Context, _ *token.Claims) ([]string, error) {
	return g.izin, g.err
}

func (g *guardTiruan) RecordImpersonationMutation(_ context.Context, ic token.ImpersonationContext, info token.MutationInfo) {
	g.mutasi = append(g.mutasi, info)
	g.konteks = append(g.konteks, ic)
}

func tokenImp(t *testing.T, m *token.Manager, izin []string) string {
	t.Helper()
	str, err := m.CreateAccessToken(uid("sasaran"), "OPS_MAHASISWA", izin,
		token.WithSession(uid("sid")),
		token.WithImpersonation(token.Impersonation{
			ID: uid("imp-1"), By: token.ImpersonatedBy{UserID: uid("admin"), Name: "Super Admin", Email: "a@x.id", Role: "ADMIN_SISTEM"}, StartedAt: 1763370000,
		}))
	require.NoError(t, err)
	return str
}

func routerImp(m *token.Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/v1", JWTAuth(m))
	v1.GET("/auth/me", func(c *gin.Context) {
		ic, ok := Impersonation(c)
		peranCtx, okCtx := token.ImpersonationFrom(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"imp": ok, "oleh": ic.By.UserID, "ctx": okCtx, "peran": peranCtx.EffectiveRole, "user": c.GetString("user_id")})
	})
	v1.POST("/harga", func(c *gin.Context) { c.Status(http.StatusCreated) })
	v1.POST("/auth/change-password", func(c *gin.Context) { c.Status(http.StatusOK) })
	v1.POST("/auth/impersonation/stop", func(c *gin.Context) { c.Status(http.StatusOK) })
	v1.GET("/admin/impersonation/policy", RequireRole("ADMIN_SISTEM"), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func kirimImp(r *gin.Engine, metode, jalur, tok string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metode, jalur, strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJWTAuth_TokenImpersonasiTanpaPenjagaDitolak(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	r := routerImp(m)

	w := kirimImp(r, http.MethodGet, "/v1/auth/me", tokenImp(t, m, []string{"harga.input"}))

	assert.Equal(t, http.StatusUnauthorized, w.Code, "gagal-tutup: tanpa penjaga token ber-imp tidak sah")
	assert.Equal(t, ImpersonationEndedCode, kodeGalat(t, w.Body.Bytes()))
}

func TestJWTAuth_TokenImpersonasi_MenyetelKonteksDanMencatatMutasi(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	g := &guardTiruan{izin: []string{"harga.input"}}
	m.SetImpersonationGuard(g)
	r := routerImp(m)
	tok := tokenImp(t, m, []string{"harga.input"})

	w := kirimImp(r, http.MethodGet, "/v1/auth/me", tok)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"imp":true,"oleh":"`+uid("admin")+`","ctx":true,"peran":"OPS_MAHASISWA","user":"`+uid("sasaran")+`"}`, w.Body.String())
	assert.Empty(t, g.mutasi, "pembacaan tidak dicatat")

	w = kirimImp(r, http.MethodPost, "/v1/harga", tok)
	require.Equal(t, http.StatusCreated, w.Code)
	require.Len(t, g.mutasi, 1)
	assert.Equal(t, "POST", g.mutasi[0].Method)
	assert.Equal(t, "/v1/harga", g.mutasi[0].Route)
	assert.Equal(t, http.StatusCreated, g.mutasi[0].Status)
	assert.Equal(t, uid("sasaran"), g.konteks[0].EffectiveUserID)
	assert.Equal(t, uid("admin"), g.konteks[0].By.UserID)

	// Berhenti/ganti punya audit sendiri: tidak dicatat dua kali.
	kirimImp(r, http.MethodPost, "/v1/auth/impersonation/stop", tok)
	assert.Len(t, g.mutasi, 1)
}

func TestJWTAuth_TokenImpersonasi_RuteTerlarangDitolakDanTetapTercatat(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	g := &guardTiruan{izin: []string{"harga.input"}}
	m.SetImpersonationGuard(g)
	r := routerImp(m)
	tok := tokenImp(t, m, []string{"harga.input"})

	w := kirimImp(r, http.MethodPost, "/v1/auth/change-password", tok)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, ImpersonationRestrictedCode, kodeGalat(t, w.Body.Bytes()))
	require.Len(t, g.mutasi, 1, "percobaan terlarang tercatat")
	assert.Equal(t, http.StatusForbidden, g.mutasi[0].Status)
}

func TestJWTAuth_TokenImpersonasi_PenjagaMenolak(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	g := &guardTiruan{err: token.ErrImpersonationEnded}
	m.SetImpersonationGuard(g)
	r := routerImp(m)
	tok := tokenImp(t, m, []string{"harga.input"})

	w := kirimImp(r, http.MethodGet, "/v1/auth/me", tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, ImpersonationEndedCode, kodeGalat(t, w.Body.Bytes()))

	g.err = token.ErrPrincipalNotFound
	assert.Equal(t, http.StatusUnauthorized, kirimImp(r, http.MethodGet, "/v1/auth/me", tok).Code)

	g.err = assert.AnError
	assert.Equal(t, http.StatusServiceUnavailable, kirimImp(r, http.MethodGet, "/v1/auth/me", tok).Code, "keadaan tak pasti: gagal-tutup 503")

	// Izin sasaran berubah dari yang dibawa token: refresh senyap seperti token biasa.
	g.err, g.izin = nil, []string{"kpi.lihat"}
	w = kirimImp(r, http.MethodGet, "/v1/auth/me", tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, PermissionsChangedCode, kodeGalat(t, w.Body.Bytes()))
}

func TestRequireRole(t *testing.T) {
	m := token.NewManager("test-secret-key-32-chars!!")
	r := routerImp(m)
	// Token biasa berperan OPS_MAHASISWA ditolak, ADMIN_SISTEM lolos.
	for peran, want := range map[string]int{"OPS_MAHASISWA": http.StatusForbidden, "ADMIN_SISTEM": http.StatusOK} {
		str, err := m.CreateAccessToken(uid("u"), peran, nil)
		require.NoError(t, err)
		assert.Equal(t, want, kirimImp(r, http.MethodGet, "/v1/admin/impersonation/policy", str).Code, peran)
	}
}

func TestImpersonasiDilarang(t *testing.T) {
	cases := []struct {
		metode, jalur string
		want          bool
	}{
		{"POST", "/v1/auth/change-password", true},
		{"POST", "/v1/auth/logout-all", true},
		{"POST", "/v1/auth/sessions/revoke-others", true},
		{"DELETE", "/v1/auth/sessions/:id", true},
		{"GET", "/v1/auth/sessions", false},
		{"PATCH", "/v1/auth/me", true},
		{"GET", "/v1/auth/me", false},
		{"POST", "/v1/auth/mfa/setup", true},
		{"POST", "/v1/auth/mfa/disable", true},
		{"POST", "/v1/auth/logout", false},
		{"POST", "/v1/auth/impersonation/start", false},
		{"POST", "/v1/auth/impersonation/stop", false},
		{"GET", "/v1/auth/impersonation/targets", false},
		{"GET", "/v1/admin/impersonation/policy", true},
		{"PUT", "/v1/admin/impersonation/policy", true},
		{"POST", "/v1/admin/impersonation/riwayat/:id/akhiri", true},
		{"GET", "/v1/users", false},
		{"POST", "/v1/users", true},
		{"PUT", "/v1/users/:id/roles", true},
		{"DELETE", "/v1/users/:id", true},
		{"GET", "/v1/roles", false},
		{"PUT", "/v1/roles/:id/permissions", true},
		{"POST", "/v1/harga", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, impersonasiDilarang(c.metode, c.jalur), "%s %s", c.metode, c.jalur)
	}
}
