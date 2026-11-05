package schematest

import (
	spasialdomain "github.com/garda-inflasi/api/internal/spasial/domain"
)

func init() {
	registerModels("spasial",
		&spasialdomain.Wilayah{}, &spasialdomain.NeracaWilayah{}, &spasialdomain.RuteDistribusi{}, &spasialdomain.KandidatColdStorage{}, &spasialdomain.WilayahGeometri{},
		&spasialdomain.JarakAntarWilayah{}, &spasialdomain.OptimasiRute{})
}
