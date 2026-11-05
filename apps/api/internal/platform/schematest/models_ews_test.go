package schematest

import (
	ewsdomain "github.com/garda-inflasi/api/internal/ews/domain"
)

func init() {
	registerModels("ews", &ewsdomain.EwsAlert{}, &ewsdomain.EwsRiwayat{}, &ewsdomain.EwsEvent{}, &ewsdomain.AturanEws{}, &ewsdomain.AturanEwsRiwayat{})
}
