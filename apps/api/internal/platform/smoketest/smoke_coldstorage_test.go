package smoketest

import "net/http"

func init() {
	registerSmoke(smokeCase{
		Name:   "coldstorage: fasilitas dan stok",
		Method: http.MethodGet,
		Path:   "/v1/cold-storage",
		// MANAJER_RISIKO membawa coldstorage.lihat; mahasiswa tidak.
		Email:          "manajer.risiko@gardainflasi.id",
		MinItems:       map[string]int{"fasilitas": 5, "stok": 10},
		ForbiddenEmail: "mahasiswa.1@gardainflasi.id",
	})

	registerSmoke(smokeCase{
		Name:           "coldstorage: detail fasilitas (telemetri, lot, transaksi)",
		Method:         http.MethodGet,
		Path:           "/v1/cold-storage/fasilitas/CS-01",
		Email:          "manajer.risiko@gardainflasi.id",
		MinItems:       map[string]int{"telemetri": 50, "stok": 5, "transaksi": 5},
		ForbiddenEmail: "mahasiswa.1@gardainflasi.id",
	})
}
