package smoketest

import (
	"net/http"
	"time"
)

func init() {
	// STRATEGIS_BI memegang dashboard.operasional; OPS_MAHASISWA tidak. Koordinator dan Agregator demo bertugas di
	// Kota Banda Aceh (1171).
	const berhak, takBerhak = "strategis.bi@gardainflasi.id", "mahasiswa.1@gardainflasi.id"
	const koordinator, agregator = "koordinator.lapangan@gardainflasi.id", "agregator.data@gardainflasi.id"

	registerSmoke(
		smokeCase{Name: "dashboard: kpi operasional", Method: http.MethodGet, Path: "/v1/dashboard/operasional/kpi", Email: berhak, MinItems: map[string]int{"": 4}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: heatmap harga", Method: http.MethodGet, Path: "/v1/dashboard/operasional/heatmap", Email: berhak, MinItems: map[string]int{"kabupaten": 10, "komoditas": 14, "sel": 100}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: aktivitas lapangan", Method: http.MethodGet, Path: "/v1/dashboard/operasional/aktivitas-lapangan", Email: berhak, MinItems: map[string]int{"": 10}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: pembanding harga lapangan vs eksternal", Method: http.MethodGet, Path: "/v1/dashboard/operasional/pembanding-harga", Email: berhak, MinItems: map[string]int{"items": 1}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: cuaca terbaru", Method: http.MethodGet, Path: "/v1/dashboard/operasional/cuaca", Email: berhak, MinItems: map[string]int{"": 20}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: siklus aktif", Method: http.MethodGet, Path: "/v1/siklus/aktif", Email: berhak, ForbiddenEmail: takBerhak},

		// Filter kabupaten, komoditas, dan periode pada KPI dan heatmap (08-B).
		smokeCase{Name: "dashboard: kpi terfilter kabupaten dan komoditas", Method: http.MethodGet, Path: "/v1/dashboard/operasional/kpi?kabupaten_kode=1171&komoditas_kode=CBM-KRT", Email: berhak, MinItems: map[string]int{"": 4}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: kpi periode 30 hari", Method: http.MethodGet, Path: "/v1/dashboard/operasional/kpi?dari=" + hariLampau(29), Email: berhak, MinItems: map[string]int{"": 4}},
		smokeCase{Name: "dashboard: heatmap terfilter kabupaten", Method: http.MethodGet, Path: "/v1/dashboard/operasional/heatmap?kabupaten_kode=1171", Email: berhak,
			MinItems: map[string]int{"kabupaten": 1, "komoditas": 10, "sel": 10}, MaxItems: map[string]int{"kabupaten": 1}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: heatmap tanggal acuan lampau", Method: http.MethodGet, Path: "/v1/dashboard/operasional/heatmap?sampai=" + hariLampau(5), Email: berhak, MinItems: map[string]int{"kabupaten": 10, "sel": 100}},

		// Row scope (02-B): Koordinator dan Agregator (kabupaten tugas 1171) hanya melihat kabupatennya di kelima endpoint.
		smokeCase{Name: "dashboard: kpi koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/kpi", Email: koordinator, MinItems: map[string]int{"": 4}},
		smokeCase{Name: "dashboard: heatmap koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/heatmap", Email: koordinator,
			MinItems: map[string]int{"kabupaten": 1, "sel": 10}, MaxItems: map[string]int{"kabupaten": 1, "sel": 14}},
		smokeCase{Name: "dashboard: aktivitas lapangan koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/aktivitas-lapangan", Email: koordinator,
			MinItems: map[string]int{"": 1}, MaxItems: map[string]int{"": 1}},
		smokeCase{Name: "dashboard: pembanding harga koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/pembanding-harga", Email: koordinator,
			MinItems: map[string]int{"items": 1}, MaxItems: map[string]int{"items": 4}},
		smokeCase{Name: "dashboard: cuaca koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/cuaca", Email: koordinator,
			MinItems: map[string]int{"": 1}, MaxItems: map[string]int{"": 1}},
		smokeCase{Name: "dashboard: aktivitas lapangan agregator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/aktivitas-lapangan", Email: agregator,
			MinItems: map[string]int{"": 1}, MaxItems: map[string]int{"": 1}},
		smokeCase{Name: "dashboard: cuaca agregator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/cuaca", Email: agregator,
			MinItems: map[string]int{"": 1}, MaxItems: map[string]int{"": 1}},

		// Ringkasan operasional (08-B). Seed demo: sepuluh mahasiswa aktif, 21 lot berlaku di tiga fasilitas operasional
		// (enam komoditas), harga produsen empat komoditas dan neraca provinsi delapan komoditas, empat penerimaan stok
		// sepekan terakhir, dan lima rute usulan. Aktivitas per petugas memuat nama petugas, jadi memerlukan harga.lihat
		// di samping dashboard.operasional (STRATEGIS_BI memegang keduanya).
		smokeCase{Name: "dashboard: aktivitas per petugas", Method: http.MethodGet, Path: "/v1/dashboard/operasional/aktivitas-petugas", Email: berhak,
			MinItems: map[string]int{"petugas": 10}, MaxItems: map[string]int{"petugas": 10}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: stok real-time", Method: http.MethodGet, Path: "/v1/dashboard/operasional/stok", Email: berhak,
			MinItems: map[string]int{"komoditas": 6}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: agregasi produsen", Method: http.MethodGet, Path: "/v1/dashboard/operasional/produsen", Email: berhak,
			MinItems: map[string]int{"komoditas": 8}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: pergerakan logistik", Method: http.MethodGet, Path: "/v1/dashboard/operasional/logistik", Email: berhak,
			MinItems: map[string]int{"komoditas": 2, "terbaru": 4, "rute.items": 5}, ForbiddenEmail: takBerhak},
		smokeCase{Name: "dashboard: stok terfilter komoditas", Method: http.MethodGet, Path: "/v1/dashboard/operasional/stok?komoditas_kode=CBM-KRT", Email: berhak,
			MinItems: map[string]int{"komoditas": 1}, MaxItems: map[string]int{"komoditas": 1}},
		smokeCase{Name: "dashboard: produsen terfilter kabupaten", Method: http.MethodGet, Path: "/v1/dashboard/operasional/produsen?kabupaten_kode=1171", Email: berhak,
			MinItems: map[string]int{"komoditas": 4}},

		// Row scope (02-B) pada ringkasan baru: Koordinator dan Agregator (kabupaten tugas 1171, Kota Banda Aceh) hanya
		// melihat kabupatennya. Banda Aceh tidak punya cold storage: stok dan pergerakan kosong, tetapi harga produsen,
		// petugas, dan satu rute yang bertujuan ke sana tetap ada.
		smokeCase{Name: "dashboard: aktivitas per petugas koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/aktivitas-petugas", Email: koordinator,
			MinItems: map[string]int{"petugas": 1}, MaxItems: map[string]int{"petugas": 1}},
		smokeCase{Name: "dashboard: stok koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/stok", Email: koordinator,
			MaxItems: map[string]int{"komoditas": 0}},
		smokeCase{Name: "dashboard: produsen koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/produsen", Email: koordinator,
			MinItems: map[string]int{"komoditas": 4}, MaxItems: map[string]int{"komoditas": 4}},
		smokeCase{Name: "dashboard: logistik koordinator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/logistik", Email: koordinator,
			MinItems: map[string]int{"rute.items": 1}, MaxItems: map[string]int{"terbaru": 0, "komoditas": 0, "rute.items": 1}},
		smokeCase{Name: "dashboard: aktivitas per petugas agregator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/aktivitas-petugas", Email: agregator,
			MinItems: map[string]int{"petugas": 1}, MaxItems: map[string]int{"petugas": 1}},
		smokeCase{Name: "dashboard: stok agregator", Method: http.MethodGet, Path: "/v1/dashboard/operasional/stok", Email: agregator,
			MaxItems: map[string]int{"komoditas": 0}},
	)
}

// hariLampau adalah tanggal Aceh n hari sebelum hari ini (YYYY-MM-DD), untuk parameter periode.
func hariLampau(n int) string {
	return time.Now().In(time.FixedZone("WIB", 7*3600)).AddDate(0, 0, -n).Format("2006-01-02")
}
