package schematest

import (
	ingestiondomain "github.com/garda-inflasi/api/internal/ingestion/domain"
)

func init() {
	registerModels("ingestion",
		&ingestiondomain.EksekusiSumber{},
		&ingestiondomain.DataMentah{},
		&ingestiondomain.PemetaanSumber{},
		&ingestiondomain.HargaEksternal{},
	)
}
