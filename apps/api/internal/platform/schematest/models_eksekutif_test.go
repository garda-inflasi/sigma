package schematest

import (
	eksekutifdomain "github.com/garda-inflasi/api/internal/eksekutif/domain"
)

func init() {
	registerModels("eksekutif",
		&eksekutifdomain.InflasiBulanan{},
		&eksekutifdomain.AndilInflasi{},
		&eksekutifdomain.NeracaPangan{},
		&eksekutifdomain.TargetInflasi{},
		&eksekutifdomain.RuangFiskal{},
	)
}
