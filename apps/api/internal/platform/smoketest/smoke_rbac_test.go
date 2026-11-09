package smoketest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/garda-inflasi/api/pkg/seed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routePermissions memetakan endpoint GET utama ke izin yang dituntutnya (docs/ARCHITECTURE.md,
// matriks rute x peran). Peran tanpa izin harus 403; peran dengan izin tidak boleh 403.
var routePermissions = map[string]string{
	"/v1/dashboard/operasional/kpi":    "dashboard.operasional",
	"/v1/dashboard/eksekutif":          "dashboard.eksekutif",
	"/v1/harga":                        "harga.lihat",
	"/v1/ews":                          "ews.lihat",
	"/v1/prediksi":                     "prediksi.lihat",
	"/v1/spasial/wilayah":              "spasial.lihat",
	"/v1/spasial/geometri":             "spasial.lihat",
	"/v1/spasial/optimasi-rute":        "spasial.lihat",
	"/v1/cold-storage":                 "coldstorage.lihat",
	"/v1/cold-storage/fasilitas/CS-01": "coldstorage.lihat",
	"/v1/intervensi":                   "intervensi.lihat",
	"/v1/policy-brief":                 "brief.lihat",
	"/v1/master/komoditas":             "master.lihat",
	"/v1/sumber-data":                  "sumber.lihat",
	"/v1/users":                        "pengguna.kelola",
	"/v1/audit":                        "audit.lihat",
	"/v1/audit/export":                 "audit.lihat",
}

// lapanganTugasRoles adalah peran yang boleh membuka /v1/lapangan/tugas/saya:
// pemegang tugas.lihat_sendiri atau tugas.kelola (sebelumnya selalu 403 karena izin yatim).
var lapanganTugasRoles = []string{"OPS_KOORDINATOR", "OPS_AGREGATOR", "OPS_MAHASISWA", "MANAJER_PROGRAM"}

// rolePermissionsFromContract membaca packages/contracts/permissions.json, yang dijaga sama dengan peta di kode oleh tes auth.
func rolePermissionsFromContract(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(apiRoot(), "..", "..", "packages", "contracts", "permissions.json"))
	require.NoError(t, err)
	var contract struct {
		Roles map[string][]string `json:"roles"`
	}
	require.NoError(t, json.Unmarshal(raw, &contract))
	return contract.Roles
}

// demoEmailByRole mengambil satu akun demo aktif untuk tiap peran.
func demoEmailByRole() map[string]string {
	out := map[string]string{}
	for _, akun := range seed.BuildDemoAkun() {
		if _, taken := out[akun.Peran]; !taken && akun.Status != "NONAKTIF" && !akun.MfaAktif {
			out[akun.Peran] = akun.Email
		}
	}
	return out
}

func TestSmoke_RoutesEnforcePermissionsPerRole(t *testing.T) {
	s := shared(t)
	roles := rolePermissionsFromContract(t)
	emails := demoEmailByRole()
	require.Len(t, emails, 14)

	for role, email := range emails {
		token := s.login(t, email)
		for path, perm := range routePermissions {
			t.Run(role+" "+path, func(t *testing.T) {
				w := s.do(t, http.MethodGet, path, token, nil)
				if slices.Contains(roles[role], perm) {
					assert.NotEqual(t, http.StatusForbidden, w.Code, "%s memegang %s", role, perm)
				} else {
					assert.Equal(t, http.StatusForbidden, w.Code, "%s tidak memegang %s", role, perm)
				}
			})
		}
	}
}

func TestSmoke_LapanganTugasNoLongerAlways403(t *testing.T) {
	s := shared(t)

	for role, email := range demoEmailByRole() {
		t.Run(role, func(t *testing.T) {
			w := s.do(t, http.MethodGet, "/v1/lapangan/tugas/saya", s.login(t, email), nil)
			if slices.Contains(lapanganTugasRoles, role) {
				assert.NotEqual(t, http.StatusForbidden, w.Code)
			} else {
				assert.Equal(t, http.StatusForbidden, w.Code)
			}
		})
	}
}
