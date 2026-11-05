package smoketest

import "net/http"

func init() {
	// STRATEGIS_BI membawa sumber.lihat; mahasiswa tidak. Riwayat eksekusi bisa kosong
	// pada seed demo: yang dikunci di sini adalah bentuk larik dan izinnya.
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "ingestion: riwayat eksekusi", Method: http.MethodGet, Path: "/v1/sumber-data/SRC-BI/eksekusi", Email: berhak, MinItems: map[string]int{"": 0}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "ingestion: sampel data mentah", Method: http.MethodGet, Path: "/v1/sumber-data/SRC-BI/sampel", Email: berhak, MinItems: map[string]int{"": 0}, ForbiddenEmail: takBerhak},
	)
}
