package smoketest

import "net/http"

func init() {
	registerSmoke(smokeCase{
		Name:   "audit: jejak audit",
		Method: http.MethodGet,
		Path:   "/v1/audit?limit=50",
		// ADMIN_SISTEM membawa audit.lihat; mahasiswa tidak.
		Email:          "superadmin@gardainflasi.id",
		MinItems:       map[string]int{"items": 50},
		ForbiddenEmail: "mahasiswa.1@gardainflasi.id",
	})
}
