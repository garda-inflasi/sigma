package schematest

import (
	sumberdatadomain "github.com/garda-inflasi/api/internal/sumber-data/domain"
)

func init() {
	registerModels("sumber-data", &sumberdatadomain.SumberData{})
}
