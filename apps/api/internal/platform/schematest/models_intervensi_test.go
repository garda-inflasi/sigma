package schematest

import (
	intervensidomain "github.com/garda-inflasi/api/internal/intervensi/domain"
)

func init() {
	registerModels("intervensi", &intervensidomain.Intervensi{}, &intervensidomain.IntervensiPJ{}, &intervensidomain.TahapDef{}, &intervensidomain.IntervensiRiwayat{})
}
