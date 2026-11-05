package smoketest

import "net/http"

func init() {
	// MANAJER_RISIKO membawa intervensi.lihat; mahasiswa tidak.
	const berhak, takBerhak = "manajer.risiko@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "intervensi: kartu kanban", Method: http.MethodGet, Path: "/v1/intervensi", Email: berhak, MinItems: map[string]int{"": 19}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "intervensi: definisi tahap", Method: http.MethodGet, Path: "/v1/intervensi/tahap", Email: berhak, MinItems: map[string]int{"": 16}, ForbiddenEmail: takBerhak},
	)
}
