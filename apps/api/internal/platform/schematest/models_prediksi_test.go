package schematest

import (
	prediksidomain "github.com/garda-inflasi/api/internal/prediksi/domain"
)

func init() {
	registerModels("prediksi", &prediksidomain.ProyeksiXai{})
}
