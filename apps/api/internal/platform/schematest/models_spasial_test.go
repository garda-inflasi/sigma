package schematest

import (
	spasialdomain "github.com/garda-inflasi/api/internal/spasial/domain"
)

func init() {
	registerModels("spasial", &spasialdomain.Wilayah{})
}
