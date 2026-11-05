package smoketest

import "net/http"

func init() {
	// STRATEGIS_BI membawa prediksi.lihat; mahasiswa tidak.
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "mahasiswa.1@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "prediksi: proyeksi seluruh komoditas", Method: http.MethodGet, Path: "/v1/prediksi", Email: berhak, MinItems: map[string]int{"": 14}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "prediksi: proyeksi cabai (XAI)", Method: http.MethodGet, Path: "/v1/prediksi/CBM-KRT", Email: berhak, MinItems: map[string]int{"seri": 2, "faktor": 3}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "prediksi: registri model", Method: http.MethodGet, Path: "/v1/prediksi/registri", Email: berhak, MinItems: map[string]int{"": 28}, ForbiddenEmail: takBerhak},
	)
}
