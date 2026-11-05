package smoketest

import "net/http"

func init() {
	// STRATEGIS_BI membawa kpi.lihat; mahasiswa tidak.
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "kpi: daftar indikator", Method: http.MethodGet, Path: "/v1/kpi", Email: berhak, MinItems: map[string]int{"": 17}, ForbiddenEmail: takBerhak},
	)
}
