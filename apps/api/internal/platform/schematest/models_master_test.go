package schematest

import (
	masterdomain "github.com/garda-inflasi/api/internal/master/domain"
)

func init() {
	registerModels("master", &masterdomain.Provinsi{}, &masterdomain.Komoditas{}, &masterdomain.Kabupaten{}, &masterdomain.Pasar{}, &masterdomain.Satuan{},
		&masterdomain.Kecamatan{}, &masterdomain.Pedagang{}, &masterdomain.KomoditasVersi{}, &masterdomain.PerubahanMaster{})
}
