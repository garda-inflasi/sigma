package prodcheck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }

// goodEnv adalah konfigurasi produksi yang sah.
func goodEnv() map[string]string {
	return map[string]string{
		"APP_ENV":             "production",
		"JWT_SECRET":          "k7Qm2Vx9Lp4Zr8Tn3Wb6Yc1Hd5Fg0JsXe9Ru2Pq",
		"AES_KEY":             "Zx3Cv7Bn1Mq9Wd5Er2Ty8Ui4Op6As0Lk", // 32
		"APP_ALLOWED_ORIGINS": "https://garda.example.go.id",
		"DB_PASSWORD":         "r4nd0m-Db-Pa55-xyz",
	}
}

func TestNonProductionAlwaysPasses(t *testing.T) {
	for _, appEnv := range []string{"", "development", "staging", "test"} {
		kv := map[string]string{"APP_ENV": appEnv, "JWT_SECRET": "garda-inflasi-dev-secret-key-change-in-production", "AES_KEY": "0123456789abcdef0123456789abcdef"}
		require.NoError(t, ValidateAPI(env(kv)), appEnv)
	}
}

func TestProductionAcceptsStrongConfig(t *testing.T) {
	require.NoError(t, ValidateAPI(env(goodEnv())))
	kv := goodEnv()
	kv["APP_ENV"] = " Production "
	require.NoError(t, ValidateAPI(env(kv)), "APP_ENV dibandingkan tanpa peduli huruf besar/spasi")
}

func TestProductionRejectsBadSecrets(t *testing.T) {
	cases := []struct {
		name, key, value, want string
	}{
		{"jwt kosong", "JWT_SECRET", "", "JWT_SECRET wajib"},
		{"jwt pendek", "JWT_SECRET", "pendek123", "terlalu pendek"},
		{"jwt bawaan", "JWT_SECRET", "garda-inflasi-dev-secret-key-change-in-production", "JWT_SECRET memakai nilai bawaan"},
		{"jwt penanda contoh", "JWT_SECRET", "GANTI_DENGAN_RANDOM_32_KARAKTER_xxxxx", "penanda"},
		{"jwt monoton", "JWT_SECRET", strings.Repeat("ab", 20), "monoton"},
		{"aes kosong", "AES_KEY", "", "AES_KEY wajib"},
		{"aes bawaan", "AES_KEY", "0123456789abcdef0123456789abcdef", "AES_KEY memakai nilai bawaan"},
		{"aes salah panjang", "AES_KEY", "Zx3Cv7Bn1Mq9Wd5Er2Ty8Ui4Op6As", "tepat 32 byte"},
		{"origin kosong", "APP_ALLOWED_ORIGINS", "", "origin publik"},
		{"origin hanya localhost", "APP_ALLOWED_ORIGINS", "http://localhost,https://localhost", "origin publik"},
		{"origin rusak", "APP_ALLOWED_ORIGINS", "garda.example.go.id", "tidak valid"},
		{"db password bawaan", "DB_PASSWORD", "garda_secret", "DB_PASSWORD"},
		{"db password kosong", "DB_PASSWORD", "", "DB_PASSWORD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kv := goodEnv()
			kv[tc.key] = tc.value
			err := ValidateAPI(env(kv))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			if strings.HasSuffix(tc.key, "_SECRET") || tc.key == "AES_KEY" || tc.key == "DB_PASSWORD" {
				if tc.value != "" {
					assert.NotContains(t, err.Error(), tc.value, "pesan galat tidak boleh membocorkan nilai rahasia")
				}
			}
		})
	}
}

func TestProductionReportsAllProblemsAtOnce(t *testing.T) {
	err := ValidateAPI(env(map[string]string{"APP_ENV": "production"}))
	require.Error(t, err)
	for _, name := range []string{"JWT_SECRET", "AES_KEY", "APP_ALLOWED_ORIGINS", "DB_PASSWORD"} {
		assert.Contains(t, err.Error(), name)
	}
}

func TestValidateWorkerChecksOnlyDatabase(t *testing.T) {
	require.NoError(t, ValidateWorker(env(map[string]string{"APP_ENV": "production", "DB_PASSWORD": "r4nd0m-Db-Pa55-xyz"})))
	require.Error(t, ValidateWorker(env(map[string]string{"APP_ENV": "production", "DB_PASSWORD": "garda"})))
	require.NoError(t, ValidateWorker(env(map[string]string{"DB_PASSWORD": "garda"})))
}

func TestProductionRejectsAESKeyEqualToJWTSecret(t *testing.T) {
	kv := goodEnv()
	kv["JWT_SECRET"] = kv["AES_KEY"]
	err := ValidateAPI(env(kv))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tidak boleh sama")
}
