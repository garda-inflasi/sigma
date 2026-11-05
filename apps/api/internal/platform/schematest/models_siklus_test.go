package schematest

import (
	siklusdomain "github.com/garda-inflasi/api/internal/siklus/domain"
)

func init() {
	registerModels("siklus", &siklusdomain.SiklusEkonomi{})
}
