package schematest

import (
	coldstoragedomain "github.com/garda-inflasi/api/internal/cold-storage/domain"
)

func init() {
	registerModels("cold-storage",
		&coldstoragedomain.FasilitasColdStorage{}, &coldstoragedomain.StokColdStorage{},
		&coldstoragedomain.TelemetriSuhu{}, &coldstoragedomain.TransaksiStok{},
	)
}
