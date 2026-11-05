package smoketest

import "net/http"

func init() {
	// STRATEGIS_BI membawa sumber.lihat; mahasiswa tidak.
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "sumber-data: katalog", Method: http.MethodGet, Path: "/v1/sumber-data", Email: berhak, MinItems: map[string]int{"": 8}, ForbiddenEmail: takBerhak},
	)
}
