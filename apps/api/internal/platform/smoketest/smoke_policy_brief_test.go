package smoketest

import "net/http"

func init() {
	registerSmoke(smokeCase{
		Name:   "policy-brief: daftar brief",
		Method: http.MethodGet,
		Path:   "/v1/policy-brief",
		// MANAJER_KEBIJAKAN membawa brief.lihat; mahasiswa tidak.
		Email:          "manajer.kebijakan@gardainflasi.id",
		MinItems:       map[string]int{"": 5},
		ForbiddenEmail: "mahasiswa.1@gardainflasi.id",
	})
}
