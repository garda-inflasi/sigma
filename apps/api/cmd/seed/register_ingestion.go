package main

import (
	"context"
	"time"

	ingestionseeder "github.com/garda-inflasi/api/internal/ingestion/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "ingestion.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"sumber.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := ingestionseeder.SeedReference(ctx, db)
			return seedkit.Result{"pemetaan": sum.Pemetaan}, err
		},
	})
}
