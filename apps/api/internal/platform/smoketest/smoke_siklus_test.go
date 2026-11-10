package smoketest

import "net/http"

func init() {
	// STRATEGIS_BI memegang ews.lihat (membaca aturan penentuan siklus); OPS_MAHASISWA tidak. Siklus aktif
	// (GET /v1/siklus/aktif) dicakup smoke_dashboard_test.go; evaluasi manual (POST) diuji di internal/siklus/integration.
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "siklus: aturan penentuan siklus otomatis", Method: http.MethodGet, Path: "/v1/siklus/aturan", Email: berhak, MinItems: map[string]int{"": 5}, ForbiddenEmail: takBerhak},
	)
}
