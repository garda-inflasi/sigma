package schematest

import (
	hargadomain "github.com/garda-inflasi/api/internal/harga/domain"
)

func init() {
	registerModels("harga", &hargadomain.EntriHarga{}, &hargadomain.TugasEntri{}, &hargadomain.RiwayatStatusEntri{})
}
