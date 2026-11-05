package schematest

import (
	prediksidomain "github.com/garda-inflasi/api/internal/prediksi/domain"
)

func init() {
	registerModels("prediksi", &prediksidomain.PrediksiHarga{}, &prediksidomain.ModelRegistry{}, &prediksidomain.PrediksiEvaluasi{}, &prediksidomain.ModelPromosi{}, &prediksidomain.SkenarioSimulasi{})
}
