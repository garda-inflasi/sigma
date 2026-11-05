package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func csrfRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFProtection([]string{"http://localhost:3000", "https://app.garda.id/"}))
	ok := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	r.GET("/data", ok)
	r.POST("/data", ok)
	r.DELETE("/data", ok)
	return r
}

type csrfRequest struct {
	method  string
	headers map[string]string
	cookies map[string]string
}

func (cr csrfRequest) status(t *testing.T) int {
	t.Helper()
	method := cr.method
	if method == "" {
		method = http.MethodPost
	}
	req := httptest.NewRequest(method, "http://api.test/data", nil)
	for k, v := range cr.headers {
		req.Header.Set(k, v)
	}
	for k, v := range cr.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	w := httptest.NewRecorder()
	csrfRouter().ServeHTTP(w, req)
	return w.Code
}

var authCookie = map[string]string{"garda_access": "jwt"}

func TestCSRF_SafeMethodsAreNeverChecked(t *testing.T) {
	assert.Equal(t, http.StatusNoContent, csrfRequest{method: http.MethodGet, cookies: authCookie}.status(t))
}

func TestCSRF_BearerAndAnonymousRequestsAreNotChecked(t *testing.T) {
	bearer := csrfRequest{headers: map[string]string{"Authorization": "Bearer abc"}, cookies: authCookie}
	assert.Equal(t, http.StatusNoContent, bearer.status(t), "Bearer tidak dikirim otomatis oleh browser")

	anonymous := csrfRequest{} // mis. POST /auth/login
	assert.Equal(t, http.StatusNoContent, anonymous.status(t), "tanpa cookie sesi tidak ada yang dibajak")
}

func TestCSRF_CookieRequestWithTrustedOriginPasses(t *testing.T) {
	for _, origin := range []string{"http://localhost:3000", "https://app.garda.id", "HTTP://LOCALHOST:3000"} {
		r := csrfRequest{headers: map[string]string{"Origin": origin}, cookies: authCookie}
		assert.Equal(t, http.StatusNoContent, r.status(t), origin)
	}
}

func TestCSRF_FallsBackToRefererWhenOriginIsAbsent(t *testing.T) {
	good := csrfRequest{headers: map[string]string{"Referer": "http://localhost:3000/cold-storage?x=1"}, cookies: authCookie}
	assert.Equal(t, http.StatusNoContent, good.status(t))

	bad := csrfRequest{headers: map[string]string{"Referer": "https://evil.example/halaman"}, cookies: authCookie}
	assert.Equal(t, http.StatusForbidden, bad.status(t))
}

func TestCSRF_SameHostOriginPasses(t *testing.T) {
	direct := csrfRequest{headers: map[string]string{"Origin": "http://api.test"}, cookies: authCookie}
	assert.Equal(t, http.StatusNoContent, direct.status(t), "Host request sama dengan host Origin")

	proxied := csrfRequest{
		headers: map[string]string{"Origin": "https://garda.example:8453", "X-Forwarded-Host": "garda.example:8453"},
		cookies: authCookie,
	}
	assert.Equal(t, http.StatusNoContent, proxied.status(t))
}

func TestCSRF_RejectsForeignMissingAndOpaqueOrigins(t *testing.T) {
	cases := map[string]map[string]string{
		"origin asing":     {"Origin": "https://evil.example"},
		"origin null":      {"Origin": "null"},
		"tanpa origin":     {},
		"port berbeda":     {"Origin": "http://localhost:4000"},
		"skema berbeda":    {"Origin": "https://localhost:3000"},
		"subdomain mirip":  {"Origin": "http://localhost:3000.evil.example"},
		"origin tak valid": {"Origin": "://"},
	}
	for name, headers := range cases {
		for _, method := range []string{http.MethodPost, http.MethodDelete} {
			assert.Equal(t, http.StatusForbidden, csrfRequest{method: method, headers: headers, cookies: authCookie}.status(t), name+" "+method)
		}
	}
}

func TestCSRF_DoubleSubmitTokenPassesWithoutOrigin(t *testing.T) {
	valid := csrfRequest{
		headers: map[string]string{"X-CSRF-Token": "abc123"},
		cookies: map[string]string{"garda_access": "jwt", "garda_csrf": "abc123"},
	}
	assert.Equal(t, http.StatusNoContent, valid.status(t))

	secureNames := csrfRequest{
		headers: map[string]string{"X-CSRF-Token": "abc123"},
		cookies: map[string]string{"__Host-access": "jwt", "__Host-csrf": "abc123"},
	}
	assert.Equal(t, http.StatusNoContent, secureNames.status(t))

	mismatch := csrfRequest{
		headers: map[string]string{"X-CSRF-Token": "lain"},
		cookies: map[string]string{"garda_access": "jwt", "garda_csrf": "abc123"},
	}
	assert.Equal(t, http.StatusForbidden, mismatch.status(t))

	headerOnly := csrfRequest{headers: map[string]string{"X-CSRF-Token": "abc123"}, cookies: authCookie}
	assert.Equal(t, http.StatusForbidden, headerOnly.status(t), "header tanpa cookie pasangannya tidak membuktikan apa pun")
}

func TestParseOrigins(t *testing.T) {
	assert.Equal(t, []string{"http://a", "https://b"}, ParseOrigins(" http://a , ,https://b ", "x"))
	assert.Equal(t, []string{"http://localhost:3000"}, ParseOrigins("", "http://localhost:3000"))
}
