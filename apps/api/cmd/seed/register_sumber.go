package main

import (
	"context"
	"time"

	"github.com/garda-inflasi/api/internal/seedkit"
	sumberseeder "github.com/garda-inflasi/api/internal/sumber-data/seeder"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name: "sumber.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := sumberseeder.SeedReference(ctx, db)
			return seedkit.Result{"sumber": sum.Sumber}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:  "sumber.demo",
		Tier:  seedkit.TierDemo,
		After: []string{"sumber.reference"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := sumberseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"sumber": sum.Sumber}, err
		},
	})
}
