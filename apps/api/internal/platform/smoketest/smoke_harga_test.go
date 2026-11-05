package smoketest

import "net/http"

func init() {
	// OPS_KOORDINATOR membawa harga.lihat; OPS_MAHASISWA hanya harga.input.
	const berhak, takBerhak = "koordinator.lapangan@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "harga: daftar entri", Method: http.MethodGet, Path: "/v1/harga?limit=100", Email: berhak, MinItems: map[string]int{"": 100}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "harga: antrean validasi", Method: http.MethodGet, Path: "/v1/harga?status=DIAJUKAN&limit=100", Email: berhak, MinItems: map[string]int{"": 10}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "harga: tren", Method: http.MethodGet, Path: "/v1/harga/tren?komoditas=CBM-KRT&wilayah=1171", Email: berhak, MinItems: map[string]int{"": 20}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "harga: ringkasan", Method: http.MethodGet, Path: "/v1/harga/ringkasan", Email: berhak, ForbiddenEmail: takBerhak},
		// Riwayat entri milik petugas lapangan sendiri; peran lihat-saja tidak punya harga.input.
		smokeCase{Name: "harga: riwayat petugas", Method: http.MethodGet, Path: "/v1/harga/saya", Email: takBerhak, MinItems: map[string]int{"": 50}, ForbiddenEmail: "strategis.bi@gardainflasi.id"},
		// Entri HRG-D10-... ada di setiap seed (sepuluh hari lalu); riwayatnya dimulai dari pencatatan.
		smokeCase{Name: "harga: riwayat status entri", Method: http.MethodGet, Path: "/v1/harga/HRG-D10-00-00-K/riwayat", Email: berhak, MinItems: map[string]int{"": 1}, ForbiddenEmail: "superadmin@gardainflasi.id"},
	)
}
