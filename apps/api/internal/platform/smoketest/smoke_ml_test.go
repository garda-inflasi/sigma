package smoketest

import (
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rute pipeline ML pada router penuh tanpa Mongo (tes asap tidak menyetel MONGO_URI): tetap 200 dengan tersedia=false.
func init() {
	registerSmoke(
		smokeCase{Name: "ml: riwayat siklus", Method: http.MethodGet, Path: "/v1/ml/cycles",
			Email: "strategis.bi@gardainflasi.id", ForbiddenEmail: "koordinator.lapangan@gardainflasi.id"},
		smokeCase{Name: "ml: kesegaran sumber", Method: http.MethodGet, Path: "/v1/ml/freshness",
			Email: "manajer.data@gardainflasi.id", ForbiddenEmail: "manajer.kebijakan@gardainflasi.id"},
	)
}

// mlRoutePermissions adalah izin rute /v1/ml/* (docs/PIPELINE-DATA-ML.md §5.2).
var mlRoutePermissions = map[string]string{
	"/v1/ml/cycles":    "prediksi.lihat",
	"/v1/ml/freshness": "sumber.lihat",
}

// Setiap peran demo (termasuk ADMIN_SISTEM) diperiksa terhadap matriks packages/contracts/permissions.json.
func TestSmoke_RuteMLPerPeran(t *testing.T) {
	s := shared(t)
	roles := rolePermissionsFromContract(t)
	for role, email := range demoEmailByRole() {
		token := s.login(t, email)
		for path, perm := range mlRoutePermissions {
			t.Run(role+" "+path, func(t *testing.T) {
				w := s.do(t, http.MethodGet, path, token, nil)
				if slices.Contains(roles[role], perm) {
					assert.Equal(t, http.StatusOK, w.Code, "%s memegang %s: %s", role, perm, w.Body.String())
				} else {
					assert.Equal(t, http.StatusForbidden, w.Code, "%s tidak memegang %s", role, perm)
				}
			})
		}
	}
	assert.True(t, slices.Contains(roles["ADMIN_SISTEM"], "prediksi.lihat") && slices.Contains(roles["ADMIN_SISTEM"], "sumber.lihat"))
}

// Endpoint lama pada router penuh membawa lampiran ml yang menyatakan Mongo tidak dikonfigurasi, tanpa mengubah data.
func TestSmoke_LampiranMLTanpaMongo(t *testing.T) {
	if os.Getenv("MONGO_URI") != "" || os.Getenv("MONGO_URL") != "" {
		t.Skip("MONGO_URI/MONGO_URL disetel di lingkungan uji: router penuh membaca Mongo itu")
	}
	s := shared(t)
	token := s.login(t, "strategis.bi@gardainflasi.id")
	for _, path := range []string{"/v1/prediksi", "/v1/ews", "/v1/dashboard/eksekutif", "/v1/sumber-data", "/v1/kpi"} {
		w := s.do(t, http.MethodGet, path, token, nil)
		require.Equal(t, http.StatusOK, w.Code, path)
		var body struct {
			ML struct {
				Tersedia  bool   `json:"tersedia"`
				Alasan    string `json:"alasan"`
				DataUtama string `json:"data_utama"`
			} `json:"ml"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.False(t, body.ML.Tersedia, path)
		assert.Equal(t, "TIDAK_DIKONFIGURASI", body.ML.Alasan, path)
		assert.Equal(t, "POSTGRES", body.ML.DataUtama, path)
	}
}
