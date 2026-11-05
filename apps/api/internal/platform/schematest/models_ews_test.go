package schematest

import (
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
)

func init() {
	registerModels("ews", &ewsdomain.EwsAlert{}, &ewsdomain.AturanEws{})
}
