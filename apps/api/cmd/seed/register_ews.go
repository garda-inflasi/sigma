package main

import (
	"context"
	"time"

	ewsseeder "github.com/garda-inflasi/api/internal/ews/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "ews.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := ewsseeder.SeedReference(ctx, db)
			return seedkit.Result{"aturan": sum.Aturan}, err
		},
	})

	seedkit.Register(seedkit.Seeder{
		Name:      "ews.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"ews.reference", "auth.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := ewsseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"alert": sum.Alert}, err
		},
	})
}
