package smoketest

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	authRoutes "github.com/garda-inflasi/api/internal/auth/routes"
	"github.com/garda-inflasi/api/pkg/middleware"
	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/garda-inflasi/api/pkg/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	registerSmoke(
		smokeCase{
			Name:           "auth: daftar pengguna",
			Method:         http.MethodGet,
			Path:           "/v1/users",
			Email:          "superadmin@gardainflasi.id",
			MinItems:       map[string]int{"items": 20},
			ForbiddenEmail: "mahasiswa.1@gardainflasi.id",
		},
		smokeCase{
			Name:           "auth: katalog peran",
			Method:         http.MethodGet,
			Path:           "/v1/roles",
			Email:          "superadmin@gardainflasi.id",
			MinItems:       map[string]int{"": 14},
			ForbiddenEmail: "manajer.program@gardainflasi.id",
		},
		smokeCase{
			Name:           "auth: katalog izin",
			Method:         http.MethodGet,
			Path:           "/v1/permissions",
			Email:          "superadmin@gardainflasi.id",
			MinItems:       map[string]int{"": 30},
			ForbiddenEmail: "koordinator.lapangan@gardainflasi.id",
		},
	)
}

type hasilLogin struct {
	AccessToken        string `json:"access_token"`
	MFARequired        bool   `json:"mfa_required"`
	MFAToken           string `json:"mfa_token"`
	MFASetupDiwajibkan bool   `json:"mfa_setup_diwajibkan"`
	User               *struct {
		Peran              string `json:"peran"`
		WajibGantiPassword bool   `json:"wajib_ganti_password"`
	} `json:"user"`
}

func (s *stack) loginDemo(t *testing.T, email string) (int, hasilLogin) {
	t.Helper()
	w := s.do(t, http.MethodPost, "/v1/auth/login", "", map[string]string{"email": email, "password": demoPassword})
	var data hasilLogin
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(readEnvelope(t, w).Data, &data))
	}
	return w.Code, data
}

// Setiap akun demo aktif dapat masuk dan menerima peran yang benar; akun NONAKTIF ditolak. Akun demo
// ber-MFA lebih dulu menerima tantangan dan masuk setelah kode OTP dari rahasia demo.
func TestSmoke_AllDemoAccountsCanLogIn(t *testing.T) {
	s := shared(t)

	for _, akun := range seed.BuildDemoAkun() {
		t.Run(akun.ID, func(t *testing.T) {
			code, data := s.loginDemo(t, akun.Email)

			if akun.Status == "NONAKTIF" {
				assert.Equal(t, http.StatusForbidden, code)
				return
			}
			require.Equal(t, http.StatusOK, code)

			if akun.MfaAktif {
				require.True(t, data.MFARequired, "akun ber-MFA ditantang kode OTP")
				assert.Empty(t, data.AccessToken)
				otp, err := totp.GenerateTOTP(seed.DemoMFASecret, time.Now())
				require.NoError(t, err)
				w := s.do(t, http.MethodPost, "/v1/auth/mfa/validate", "", map[string]string{"mfa_token": data.MFAToken, "code": otp})
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				data = hasilLogin{}
				require.NoError(t, json.Unmarshal(readEnvelope(t, w).Data, &data))
			}

			require.NotNil(t, data.User)
			assert.NotEmpty(t, data.AccessToken)
			assert.Equal(t, akun.Peran, data.User.Peran)
			assert.False(t, data.User.WajibGantiPassword)
			assert.False(t, data.MFASetupDiwajibkan, "penegakan MFA mati secara bawaan di lokal/demo")
		})
	}
}

// Dengan MFA_ENFORCE menyala, akun demo peran wajib-MFA yang belum setup tetap bisa masuk, tetapi
// sesinya hanya membuka endpoint setup MFA; akun peran lain tidak terpengaruh.
func TestSmoke_MFAEnforcementLeavesDemoLoginsUsableForSetup(t *testing.T) {
	t.Setenv("MFA_ENFORCE", "true")
	s := newRouterStack(shared(t).db, authRoutes.Options{UseCaseConfig: authRoutes.ConfigFromEnv(os.Getenv)})

	var wajib, bebas int
	for _, akun := range seed.BuildDemoAkun() {
		if akun.Status == "NONAKTIF" || akun.MfaAktif {
			continue
		}
		t.Run(akun.ID, func(t *testing.T) {
			code, data := s.loginDemo(t, akun.Email)
			require.Equal(t, http.StatusOK, code)
			require.False(t, data.MFARequired)

			me := s.do(t, http.MethodGet, "/v1/auth/me", data.AccessToken, nil)
			require.Equal(t, http.StatusOK, me.Code, "profil selalu terbuka")

			if data.MFASetupDiwajibkan {
				wajib++
				blocked := s.do(t, http.MethodGet, "/v1/harga/saya", data.AccessToken, nil)
				assert.Equal(t, http.StatusForbidden, blocked.Code)
				assert.Equal(t, middleware.MFASetupRequiredCode, readEnvelope(t, blocked).Code)
				setup := s.do(t, http.MethodPost, "/v1/auth/mfa/setup", data.AccessToken, nil)
				assert.Equal(t, http.StatusOK, setup.Code, "setup MFA terbuka: %s", setup.Body.String())
			} else {
				bebas++
				assert.NotEqual(t, middleware.MFASetupRequiredCode, readEnvelope(t, s.do(t, http.MethodGet, "/v1/harga/saya", data.AccessToken, nil)).Code)
			}
		})
	}
	assert.Equal(t, 12, wajib, "12 akun demo non-MFA berperan wajib: admin, 4 strategis, 6 manajerial, koordinator lapangan")
	assert.Equal(t, 11, bebas, "agregator dan 10 mahasiswa aktif tidak diwajibkan")
}
