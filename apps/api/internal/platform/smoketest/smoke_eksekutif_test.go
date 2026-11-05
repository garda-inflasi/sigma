package smoketest

import "net/http"

func init() {
	// STRATEGIS_BI membawa dashboard.eksekutif; MANAJER_DATA tidak.
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "manajer.data@gardainflasi.id"

	registerSmoke(
		smokeCase{
			Name: "eksekutif: ringkasan", Method: http.MethodGet, Path: "/v1/dashboard/eksekutif", Email: berhak,
			MinItems:       map[string]int{"inflasi.seri": 12, "inflasi.penyumbang": 6, "neraca.items": 8, "fiskal.instrumen": 4},
			ForbiddenEmail: takBerhak,
		},
	)
}
