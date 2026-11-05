package schematest

import (
	kpidomain "github.com/garda-inflasi/api/internal/kpi/domain"
)

func init() {
	registerModels("kpi", &kpidomain.Definisi{}, &kpidomain.Target{}, &kpidomain.Nilai{})
}
