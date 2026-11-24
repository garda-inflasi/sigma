package schematest

import (
	mldomain "github.com/garda-inflasi/api/internal/ml/domain"
)

func init() {
	registerModels("ml", &mldomain.EwsSinyalML{})
}
