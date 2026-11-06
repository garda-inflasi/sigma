package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/pkg/token"
)

// fakeResolver menjawab izin efektif per pengguna dan dapat dibuat gagal.
type fakeResolver struct {
	mu    sync.Mutex
	izin  map[string][]string
	err   error
	calls int
}

func (f *fakeResolver) Permissions(_ context.Context, userID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	izin, ok := f.izin[userID]
	if !ok {
		return nil, token.ErrPrincipalNotFound
	}
	return izin, nil
}

func (f *fakeResolver) set(userID string, izin ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.izin[userID] = izin
}

// izinRouter memasang JWTAuth lalu dua rute: satu tanpa syarat izin (mengembalikan izin yang dilihat handler)
// dan satu yang mensyaratkan kpi.lihat.
func izinRouter(tokenMgr *token.Manager) *gin.Engine {
	router := gin.New()
	router.Use(JWTAuth(tokenMgr))
	router.GET("/uji", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"permissions": c.GetStringSlice("permissions")})
	})
	router.GET("/kpi", RequirePermission("kpi.lihat"), func(c *gin.Context) { c.Status(http.StatusOK) })
	return router
}

func doGetPath(router *gin.Engine, path, tokenStr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func kodeGalat(t *testing.T, body []byte) string {
	t.Helper()
	var jawab struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(body, &jawab))
	return jawab.Code
}

func TestJWTAuth_PermissionResolver_IzinDitegakkanDariResolverBukanKlaim(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	resolver := &fakeResolver{izin: map[string][]string{uid("u1"): {"harga.lihat", "kpi.lihat"}}}
	tokenMgr.SetPermissionResolver(resolver)
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("u1"), "ADMIN_SISTEM", []string{"harga.lihat", "kpi.lihat"})
	require.NoError(t, err)

	w := doGet(router, tokenStr)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Permissions []string `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.ElementsMatch(t, []string{"harga.lihat", "kpi.lihat"}, body.Permissions)
}

func TestJWTAuth_PermissionResolver_UrutanDanDuplikatDiabaikan(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenMgr.SetPermissionResolver(&fakeResolver{izin: map[string][]string{uid("u1"): {"b.satu", "a.dua"}}})
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("u1"), "ADMIN_SISTEM", []string{"a.dua", "b.satu", "a.dua"})
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, doGet(router, tokenStr).Code)
}

func TestJWTAuth_PermissionResolver_IzinDicabutMenolakTokenDenganKodeKhusus(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	resolver := &fakeResolver{izin: map[string][]string{uid("u1"): {"kpi.lihat"}}}
	tokenMgr.SetPermissionResolver(resolver)
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("u1"), "ADMIN_SISTEM", []string{"kpi.lihat"})
	require.NoError(t, err)

	req := func(path string) int {
		return doGetPath(router, path, tokenStr).Code
	}
	require.Equal(t, http.StatusOK, req("/kpi"))

	resolver.set(uid("u1")) // izin dicabut di basis data; token masih membawa kpi.lihat

	w := doGetPath(router, "/kpi", tokenStr)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "izin tidak lagi berlaku: bukan 200 hanya karena klaim token masih memuatnya")
	assert.Equal(t, PermissionsChangedCode, kodeGalat(t, w.Body.Bytes()))
	assert.Contains(t, w.Body.String(), PermissionsChangedMessage)
	assert.NotContains(t, PermissionsChangedMessage, "password", "pesan tidak boleh terbaca sebagai salah kredensial oleh klien web")
	assert.NotContains(t, PermissionsChangedMessage, "kata sandi")
}

func TestJWTAuth_PermissionResolver_IzinBaruDiberikanMenolakTokenBasiAgarDiperbarui(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	resolver := &fakeResolver{izin: map[string][]string{uid("u1"): {"harga.lihat"}}}
	tokenMgr.SetPermissionResolver(resolver)
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("u1"), "OPS_MAHASISWA", []string{"harga.lihat"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, doGetPath(router, "/uji", tokenStr).Code)

	resolver.set(uid("u1"), "harga.lihat", "kpi.lihat") // izin langsung diberikan

	w := doGetPath(router, "/kpi", tokenStr)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, PermissionsChangedCode, kodeGalat(t, w.Body.Bytes()))

	baru, err := tokenMgr.CreateAccessToken(uid("u1"), "OPS_MAHASISWA", []string{"harga.lihat", "kpi.lihat"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, doGetPath(router, "/kpi", baru).Code, "token hasil refresh langsung berlaku")
}

func TestJWTAuth_PermissionResolver_AkunTidakAdaAtauNonaktifDijawab401(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenMgr.SetPermissionResolver(&fakeResolver{izin: map[string][]string{}})
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("hantu"), "ADMIN_SISTEM", []string{"kpi.lihat"})
	require.NoError(t, err)

	w := doGetPath(router, "/uji", tokenStr)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), SessionRevokedMessage)
}

func TestJWTAuth_PermissionResolver_GalatInfrastrukturGagalTutup503(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenMgr.SetPermissionResolver(&fakeResolver{err: errors.New("connection refused")})
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("u1"), "ADMIN_SISTEM", []string{"kpi.lihat"})
	require.NoError(t, err)

	w := doGetPath(router, "/kpi", tokenStr)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "izin yang tidak dapat dipastikan tidak boleh diloloskan lewat klaim")
}

func TestJWTAuth_PermissionResolver_TanpaResolverKlaimDipakaiApaAdanya(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("u1"), "ADMIN_SISTEM", []string{"kpi.lihat"})
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, doGetPath(router, "/kpi", tokenStr).Code)
}

func TestJWTAuth_PermissionResolver_NilMelepasResolver(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenMgr.SetPermissionResolver(&fakeResolver{err: errors.New("x")})
	tokenMgr.SetPermissionResolver(nil)
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("u1"), "ADMIN_SISTEM", []string{"kpi.lihat"})
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, doGetPath(router, "/kpi", tokenStr).Code)
}

func TestJWTAuth_PermissionResolver_SesiDicabutMenangSebelumPemeriksaanIzin(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	resolver := &fakeResolver{izin: map[string][]string{uid("u1"): {"kpi.lihat"}}}
	tokenMgr.SetPermissionResolver(resolver)
	tokenMgr.SetRevocationChecker(&fakeChecker{revokedSID: map[string]bool{sidUji: true}})
	router := izinRouter(tokenMgr)
	tokenStr, err := tokenMgr.CreateAccessToken(uid("u1"), "ADMIN_SISTEM", []string{"kpi.lihat"}, token.WithSession(sidUji))
	require.NoError(t, err)

	w := doGetPath(router, "/kpi", tokenStr)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), SessionRevokedMessage)
	assert.Zero(t, resolver.calls, "token yang sudah dicabut tidak memicu pencarian izin")
}
