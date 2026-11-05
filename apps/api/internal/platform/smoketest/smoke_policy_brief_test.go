package smoketest

import "net/http"

func init() {
	// MANAJER_KEBIJAKAN membawa brief.lihat; mahasiswa tidak.
	const berhak, takBerhak = "manajer.kebijakan@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "policy-brief: daftar brief", Method: http.MethodGet, Path: "/v1/policy-brief", Email: berhak, MinItems: map[string]int{"": 5}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "policy-brief: detail brief contoh", Method: http.MethodGet, Path: "/v1/policy-brief/PB-2026-10", Email: berhak, MinItems: map[string]int{"konten.bagian": 2}, ForbiddenEmail: takBerhak},
	)
}
