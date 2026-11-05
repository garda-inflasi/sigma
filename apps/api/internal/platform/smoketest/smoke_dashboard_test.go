package smoketest

import "net/http"

func init() {
	// STRATEGIS_BI memegang dashboard.operasional; OPS_MAHASISWA tidak.
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "dashboard: kpi operasional", Method: http.MethodGet, Path: "/v1/dashboard/operasional/kpi", Email: berhak, MinItems: map[string]int{"": 4}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: heatmap harga", Method: http.MethodGet, Path: "/v1/dashboard/operasional/heatmap", Email: berhak, MinItems: map[string]int{"kabupaten": 10, "komoditas": 14, "sel": 100}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: aktivitas lapangan", Method: http.MethodGet, Path: "/v1/dashboard/operasional/aktivitas-lapangan", Email: berhak, MinItems: map[string]int{"": 10}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: siklus aktif", Method: http.MethodGet, Path: "/v1/siklus/aktif", Email: berhak, ForbiddenEmail: takBerhak},
	)
}
