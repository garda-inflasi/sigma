package prodcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ssoEnv adalah konfigurasi produksi sah dengan SSO Google menyala.
func ssoEnv() map[string]string {
	kv := goodEnv()
	kv["SSO_GOOGLE_CLIENT_ID"] = "123456789012-abcdefghijklmnop.apps.googleusercontent.com"
	kv["SSO_GOOGLE_CLIENT_SECRET"] = "GOCSPX-k7Qm2Vx9Lp4Zr8Tn3Wb6Yc1Hd5"
	kv["SSO_GOOGLE_REDIRECT_URL"] = "https://garda.example.go.id/api/auth/sso/google/callback"
	return kv
}

func TestSSOOpsionalTanpaKonfigurasi(t *testing.T) {
	require.NoError(t, ValidateAPI(env(goodEnv())), "produksi tanpa SSO tetap sah")
	require.NoError(t, ValidateSSO(env(goodEnv())))
}

func TestSSOKonfigurasiLengkapSah(t *testing.T) {
	require.NoError(t, ValidateAPI(env(ssoEnv())))

	kv := ssoEnv()
	kv["SSO_GOOGLE_ISSUER"] = "https://accounts.google.com"
	kv["SSO_GOOGLE_HOSTED_DOMAIN"] = "Kemenkeu.go.id"
	kv["APP_ALLOWED_ORIGINS"] = "https://garda.example.go.id/, https://www.example.go.id"
	require.NoError(t, ValidateAPI(env(kv)), "issuer https, domain Workspace, dan origin dengan garis miring penutup")
}

func TestSSODiLuarProduksiTidakDiperiksa(t *testing.T) {
	kv := map[string]string{
		"APP_ENV": "development", "SSO_GOOGLE_CLIENT_ID": "id", "SSO_GOOGLE_ISSUER": "http://sso.localhost:8089/default",
		"SSO_GOOGLE_REDIRECT_URL": "http://localhost:8090/api/auth/sso/google/callback",
	}
	require.NoError(t, ValidateAPI(env(kv)), "mock OIDC lokal sah di luar produksi")
}

func TestSSOProduksiMenolakKonfigurasiBermasalah(t *testing.T) {
	cases := []struct {
		name, key, value, want string
	}{
		{"secret kosong", "SSO_GOOGLE_CLIENT_SECRET", "", "SSO_GOOGLE_CLIENT_SECRET wajib"},
		{"secret contoh", "SSO_GOOGLE_CLIENT_SECRET", "ganti_dengan_secret_dari_google", "SSO_GOOGLE_CLIENT_SECRET mengandung penanda"},
		{"redirect kosong", "SSO_GOOGLE_REDIRECT_URL", "", "SSO_GOOGLE_REDIRECT_URL wajib"},
		{"redirect http", "SSO_GOOGLE_REDIRECT_URL", "http://garda.example.go.id/api/auth/sso/google/callback", "harus memakai https"},
		{"redirect localhost", "SSO_GOOGLE_REDIRECT_URL", "https://localhost/api/auth/sso/google/callback", "localhost"},
		{"redirect relatif", "SSO_GOOGLE_REDIRECT_URL", "/api/auth/sso/google/callback", "URL absolut"},
		{"redirect path salah", "SSO_GOOGLE_REDIRECT_URL", "https://garda.example.go.id/auth/callback", "harus tepat menunjuk ke /api/auth/sso/google/callback"},
		{"redirect dengan query", "SSO_GOOGLE_REDIRECT_URL", "https://garda.example.go.id/api/auth/sso/google/callback?x=1", "tanpa query"},
		{"redirect origin tak terdaftar", "SSO_GOOGLE_REDIRECT_URL", "https://lain.example.go.id/api/auth/sso/google/callback", "APP_ALLOWED_ORIGINS"},
		{"issuer http", "SSO_GOOGLE_ISSUER", "http://accounts.example.com", "SSO_GOOGLE_ISSUER harus memakai https"},
		{"issuer mock lokal", "SSO_GOOGLE_ISSUER", "https://sso.localhost/default", "SSO_GOOGLE_ISSUER tidak boleh menunjuk ke localhost"},
		{"domain berupa email", "SSO_GOOGLE_HOSTED_DOMAIN", "@kemenkeu.go.id", "SSO_GOOGLE_HOSTED_DOMAIN harus berupa nama domain"},
		{"domain tanpa titik", "SSO_GOOGLE_HOSTED_DOMAIN", "kemenkeu", "SSO_GOOGLE_HOSTED_DOMAIN harus berupa nama domain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kv := ssoEnv()
			kv[tc.key] = tc.value
			err := ValidateAPI(env(kv))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			if secret := kv["SSO_GOOGLE_CLIENT_SECRET"]; secret != "" {
				assert.NotContains(t, err.Error(), secret, "pesan galat tidak boleh membocorkan rahasia")
			}
		})
	}
}

func TestSSOKonfigurasiSetengahJadiDitolak(t *testing.T) {
	for _, key := range []string{"SSO_GOOGLE_CLIENT_SECRET", "SSO_GOOGLE_REDIRECT_URL"} {
		kv := goodEnv()
		kv[key] = ssoEnv()[key]
		err := ValidateAPI(env(kv))
		require.Error(t, err, key)
		assert.Contains(t, err.Error(), "SSO_GOOGLE_CLIENT_ID kosong", key)
	}
}

func TestSSOMasalahDigabungDenganMasalahLain(t *testing.T) {
	kv := ssoEnv()
	kv["SSO_GOOGLE_CLIENT_SECRET"] = ""
	kv["JWT_SECRET"] = ""
	err := ValidateAPI(env(kv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
	assert.Contains(t, err.Error(), "SSO_GOOGLE_CLIENT_SECRET")
}
