package schematest

import (
	masterdomain "github.com/garda-inflasi/api/internal/master/domain"
)

func init() {
	registerModels("master", &masterdomain.Komoditas{}, &masterdomain.Kabupaten{}, &masterdomain.Pasar{})
}
