package schematest

import (
	lapangandomain "github.com/garda-inflasi/api/internal/lapangan/domain"
)

func init() {
	registerModels("lapangan",
		&lapangandomain.TugasLapangan{},
		&lapangandomain.LampiranEntri{},
		&lapangandomain.KodeEtik{},
		&lapangandomain.PersetujuanKodeEtik{},
	)
}
