package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/garda-inflasi/api/pkg/token"
)

const sidUji = "3f1c8a52-0a57-4b9a-8f0e-0d1f6f3c9a11"

// fakeChecker mencatat panggilan dan menjawab sesuai tabel jti/sid yang dicabut.
type fakeChecker struct {
	revokedJTI map[string]bool
	revokedSID map[string]bool
	err        error
	calls      atomic.Int32
	lastJTI    atomic.Value
	lastSID    atomic.Value
}

func (f *fakeChecker) Revoked(_ context.Context, jti, sid string) (bool, error) {
	f.calls.Add(1)
	f.lastJTI.Store(jti)
	f.lastSID.Store(sid)
	if f.err != nil {
		return false, f.err
	}
	return f.revokedJTI[jti] || (sid != "" && f.revokedSID[sid]), nil
}

func revocationRouter(tokenMgr *token.Manager) *gin.Engine {
	router := gin.New()
	router.Use(JWTAuth(tokenMgr))
	router.GET("/uji", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"sid": SessionID(c), "jti": c.GetString("jti")})
	})
	return router
}

func doGet(router *gin.Engine, tokenStr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/uji", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestJWTAuth_RevokedSessionIsRejectedImmediately(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	checker := &fakeChecker{revokedSID: map[string]bool{}}
	tokenMgr.SetRevocationChecker(checker)
	router := revocationRouter(tokenMgr)

	tokenStr, err := tokenMgr.CreateAccessToken("u1", "ADMIN_SISTEM", nil, token.WithSession(sidUji))
	require.NoError(t, err)

	w := doGet(router, tokenStr)
	require.Equal(t, http.StatusOK, w.Code)
	var ok map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ok))
	assert.Equal(t, sidUji, ok["sid"], "sid tersedia bagi handler")

	checker.revokedSID[sidUji] = true // sesi dicabut di antara dua request

	w = doGet(router, tokenStr)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), SessionRevokedMessage)
}

func TestJWTAuth_BlacklistedJTIIsRejected(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenStr, err := tokenMgr.CreateAccessToken("u1", "ADMIN_SISTEM", nil)
	require.NoError(t, err)
	claims, err := tokenMgr.VerifyToken(tokenStr)
	require.NoError(t, err)
	tokenMgr.SetRevocationChecker(&fakeChecker{revokedJTI: map[string]bool{claims.ID: true}})

	w := doGet(revocationRouter(tokenMgr), tokenStr)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_TokenWithoutSidIsCheckedByJTIOnly(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	checker := &fakeChecker{revokedSID: map[string]bool{"": true}} // sid kosong tidak boleh dianggap dicabut
	tokenMgr.SetRevocationChecker(checker)
	tokenStr, err := tokenMgr.CreateAccessToken("u1", "ADMIN_SISTEM", nil)
	require.NoError(t, err)

	w := doGet(revocationRouter(tokenMgr), tokenStr)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", checker.lastSID.Load())
}

func TestJWTAuth_CheckerErrorFailsClosedWith503(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenMgr.SetRevocationChecker(&fakeChecker{err: errors.New("connection refused")})
	tokenStr, err := tokenMgr.CreateAccessToken("u1", "ADMIN_SISTEM", nil, token.WithSession(sidUji))
	require.NoError(t, err)

	w := doGet(revocationRouter(tokenMgr), tokenStr)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "503, bukan 401: klien tidak boleh menyangka sesinya berakhir")
}

func TestJWTAuth_NoCheckerInstalledAllowsToken(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenStr, err := tokenMgr.CreateAccessToken("u1", "ADMIN_SISTEM", nil, token.WithSession(sidUji))
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, doGet(revocationRouter(tokenMgr), tokenStr).Code)
}

func TestJWTAuth_CheckerIsNotConsultedForInvalidSignature(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	checker := &fakeChecker{}
	tokenMgr.SetRevocationChecker(checker)

	w := doGet(revocationRouter(tokenMgr), "bukan-jwt")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Zero(t, checker.calls.Load(), "tanda tangan diperiksa lebih dulu; basis data tidak disentuh")
}

func TestManager_SetRevocationCheckerNilRemovesChecker(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!")
	tokenMgr.SetRevocationChecker(&fakeChecker{err: errors.New("x")})
	tokenMgr.SetRevocationChecker(nil)
	tokenStr, err := tokenMgr.CreateAccessToken("u1", "ADMIN_SISTEM", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, doGet(revocationRouter(tokenMgr), tokenStr).Code)
}

func TestTokenLifetimeContext_EndsWhenAccessTokenExpires(t *testing.T) {
	tokenMgr := token.NewManager("test-secret-key-32-chars!!", token.WithAccessTTL(2*time.Hour))
	tokenStr, err := tokenMgr.CreateAccessToken("u1", "ADMIN_SISTEM", nil)
	require.NoError(t, err)
	claims, err := tokenMgr.VerifyToken(tokenStr)
	require.NoError(t, err)

	router := gin.New()
	router.Use(JWTAuth(tokenMgr))
	var deadline time.Time
	var hasDeadline bool
	router.GET("/uji", func(c *gin.Context) {
		ctx, cancel := TokenLifetimeContext(c)
		defer cancel()
		deadline, hasDeadline = ctx.Deadline()
		c.Status(http.StatusOK)
	})

	require.Equal(t, http.StatusOK, doGet(router, tokenStr).Code)
	require.True(t, hasDeadline)
	assert.WithinDuration(t, claims.ExpiresAt.Time, deadline, time.Second)
}

func TestTokenLifetimeContext_WithoutTokenExpiryStillCancellable(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	ctx, cancel := TokenLifetimeContext(c)
	_, hasDeadline := ctx.Deadline()
	assert.False(t, hasDeadline)
	cancel()
	assert.Error(t, ctx.Err())
}
