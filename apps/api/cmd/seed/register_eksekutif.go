package main

import (
	"context"
	"time"

	eksekutifseeder "github.com/garda-inflasi/api/internal/eksekutif/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name: "eksekutif.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := eksekutifseeder.SeedReference(ctx, db)
			return seedkit.Result{"target": sum.Target}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "eksekutif.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"eksekutif.reference", "ews.demo", "intervensi.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := eksekutifseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"inflasi": sum.Inflasi, "andil": sum.Andil, "neraca": sum.Neraca, "fiskal": sum.Fiskal}, err
		},
	})
}
