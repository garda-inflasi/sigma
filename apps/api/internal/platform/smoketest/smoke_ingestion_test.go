package smoketest

import "net/http"

func init() {
	// STRATEGIS_BI membawa sumber.lihat; mahasiswa tidak. Seed demo memuat 3 run PIHPS contoh.
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "ingestion: riwayat eksekusi", Method: http.MethodGet, Path: "/v1/sumber-data/SRC-BI/eksekusi", Email: berhak, MinItems: map[string]int{"": 3}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "ingestion: pipeline dan mutu data", Method: http.MethodGet, Path: "/v1/sumber-data/pipeline", Email: berhak, ForbiddenEmail: takBerhak},
		smokeCase{Name: "ingestion: sampel data mentah", Method: http.MethodGet, Path: "/v1/sumber-data/SRC-BI/sampel", Email: berhak, MinItems: map[string]int{"": 3}, ForbiddenEmail: takBerhak},
	)
}
