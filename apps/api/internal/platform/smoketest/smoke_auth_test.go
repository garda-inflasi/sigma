package smoketest

import (
	"encoding/json"
	"net/http"
	"testing"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
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

// Setiap akun demo aktif dapat masuk dan menerima peran yang benar; akun NONAKTIF ditolak.
func TestSmoke_AllDemoAccountsCanLogIn(t *testing.T) {
	s := shared(t)

	for _, akun := range authseeder.BuildDemoAkun() {
		t.Run(akun.ID, func(t *testing.T) {
			w := s.do(t, http.MethodPost, "/v1/auth/login", "", map[string]string{"email": akun.Email, "password": demoPassword})

			if akun.Status == "NONAKTIF" {
				assert.Equal(t, http.StatusForbidden, w.Code)
				return
			}
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var data struct {
				User struct {
					Peran              string `json:"peran"`
					WajibGantiPassword bool   `json:"wajib_ganti_password"`
				} `json:"user"`
			}
			require.NoError(t, json.Unmarshal(readEnvelope(t, w).Data, &data))
			assert.Equal(t, akun.Peran, data.User.Peran)
			assert.False(t, data.User.WajibGantiPassword)
		})
	}
}
