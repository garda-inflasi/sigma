package smoketest

import "net/http"

func init() {
	// MANAJER_RISIKO membawa ews.lihat; mahasiswa tidak.
	const berhak, takBerhak = "manajer.risiko@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "ews: daftar alert", Method: http.MethodGet, Path: "/v1/ews?limit=200", Email: berhak, MinItems: map[string]int{"": 17}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "ews: aturan ambang", Method: http.MethodGet, Path: "/v1/ews/aturan", Email: berhak, MinItems: map[string]int{"": 9}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "ews: preferensi notifikasi", Method: http.MethodGet, Path: "/v1/ews/notifikasi/preferensi", Email: "koordinator.lapangan@gardainflasi.id", MinItems: map[string]int{"preferensi": 2}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "ews: riwayat alert", Method: http.MethodGet, Path: "/v1/ews/EWS-1004/riwayat", Email: berhak, MinItems: map[string]int{"": 2}, ForbiddenEmail: takBerhak},
	)
}
