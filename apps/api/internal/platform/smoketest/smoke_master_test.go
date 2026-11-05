package smoketest

import "net/http"

func init() {
	// MANAJER_RISIKO membawa master.lihat; mahasiswa tidak.
	const berhak, takBerhak = "manajer.risiko@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "master: komoditas", Method: http.MethodGet, Path: "/v1/master/komoditas", Email: berhak, MinItems: map[string]int{"": 14}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "master: wilayah", Method: http.MethodGet, Path: "/v1/master/wilayah", Email: berhak, MinItems: map[string]int{"": 23}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "master: pasar", Method: http.MethodGet, Path: "/v1/master/pasar", Email: berhak, MinItems: map[string]int{"": 24}, ForbiddenEmail: takBerhak},
	)
}
