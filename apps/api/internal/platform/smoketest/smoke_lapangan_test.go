package smoketest

import "net/http"

func init() {
	// OPS_MAHASISWA membawa tugas.lihat_sendiri; STRATEGIS_BI tidak punya izin tugas.
	const berhak, takBerhak = "mahasiswa.1@gardainflasi.id", "strategis.bi@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "lapangan: tugas saya", Method: http.MethodGet, Path: "/v1/lapangan/tugas/saya", Email: berhak, MinItems: map[string]int{"": 5}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "lapangan: tugas saya (koordinator)", Method: http.MethodGet, Path: "/v1/lapangan/tugas/saya", Email: "koordinator.lapangan@gardainflasi.id", MinItems: map[string]int{"": 5}, ForbiddenEmail: takBerhak},
	)
}
