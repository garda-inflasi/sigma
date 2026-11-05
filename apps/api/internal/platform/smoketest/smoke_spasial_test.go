package smoketest

import "net/http"

func init() {
	// MANAJER_PROGRAM membawa spasial.lihat; mahasiswa tidak.
	const berhak, takBerhak = "manajer.program@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "spasial: marker wilayah", Method: http.MethodGet, Path: "/v1/spasial/wilayah", Email: berhak, MinItems: map[string]int{"": 23}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "spasial: neraca wilayah", Method: http.MethodGet, Path: "/v1/spasial/neraca", Email: berhak, MinItems: map[string]int{"": 184}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "spasial: rute distribusi", Method: http.MethodGet, Path: "/v1/spasial/rute", Email: berhak, MinItems: map[string]int{"": 5}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "spasial: kandidat cold storage", Method: http.MethodGet, Path: "/v1/spasial/kandidat-cold-storage", Email: berhak, MinItems: map[string]int{"": 3}, ForbiddenEmail: takBerhak},
	)
}
