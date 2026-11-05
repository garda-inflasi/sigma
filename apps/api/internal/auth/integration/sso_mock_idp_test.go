package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

const (
	// mockIdPImage dipasang pada versi yang sama dengan compose.yaml (profil sso-mock).
	mockIdPRepository = "ghcr.io/navikt/mock-oauth2-server"
	mockIdPTag        = "6.1.4"
	// mockIdPIssuerID adalah segmen path penerbit; penerbit lengkap diturunkan mock dari header Host permintaan.
	mockIdPIssuerID = "default"
	// mockIdPTTLDetik membatasi umur container bila proses uji mati sebelum sempat Purge.
	mockIdPTTLDetik = 600
)

// mockIdP adalah mock-oauth2-server (penyedia OIDC sungguhan: JWKS, PKCE, nonce) di container Docker.
// Tes bertindak sebagai browser terhadap halaman login interaktifnya.
type mockIdP struct {
	hostPort string
	client   *http.Client
}

// startMockIdP menyalakan container dan menunggu dokumen penemuan siap. Container dibuang saat tes selesai.
func startMockIdP(t *testing.T) *mockIdP {
	t.Helper()
	if pg == nil {
		t.Skip("tes integrasi dilewati (-short)")
	}
	pool, err := dockertest.NewPool("")
	require.NoError(t, err, "tidak bisa terhubung ke Docker")

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: mockIdPRepository,
		Tag:        mockIdPTag,
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	require.NoError(t, err, "gagal menyalakan container mock OIDC")
	t.Cleanup(func() { _ = pool.Purge(resource) })
	require.NoError(t, resource.Expire(mockIdPTTLDetik))

	idp := &mockIdP{
		hostPort: resource.GetHostPort("8080/tcp"),
		client: &http.Client{
			Timeout: 10 * time.Second,
			// Tes membaca pengalihan 302 sendiri, seperti browser yang berhenti di callback.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	pool.MaxWait = 90 * time.Second
	require.NoError(t, pool.Retry(func() error {
		res, err := idp.client.Get(idp.Issuer() + "/.well-known/openid-configuration")
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("penemuan: HTTP %d", res.StatusCode)
		}
		return nil
	}), "mock OIDC tidak siap")
	return idp
}

// Issuer adalah URL penerbit; identik untuk API (penemuan) dan "browser" (halaman login).
func (m *mockIdP) Issuer() string { return "http://" + m.hostPort + "/" + mockIdPIssuerID }

// signIn mengisi halaman login interaktif mock untuk authorizeURL: username menjadi klaim sub dan claims
// ditambahkan ke ID token. Mengembalikan code dan state dari pengalihan ke redirect_uri.
func (m *mockIdP) signIn(t *testing.T, authorizeURL, username, claimsJSON string) (code, state string) {
	t.Helper()
	res, err := m.client.Get(authorizeURL)
	require.NoError(t, err)
	page, _ := io.ReadAll(res.Body)
	res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode, string(page))
	require.Contains(t, string(page), `name="username"`, "halaman login mock berubah: selector e2e perlu diperiksa")
	require.Contains(t, string(page), `name="claims"`)

	form := url.Values{"username": {username}, "claims": {claimsJSON}}
	res, err = m.client.Post(authorizeURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, http.StatusFound, res.StatusCode)

	location, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	q := location.Query()
	require.Empty(t, q.Get("error"), location.String())
	require.NotEmpty(t, q.Get("code"), location.String())
	return q.Get("code"), q.Get("state")
}
