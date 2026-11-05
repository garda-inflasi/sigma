package main

import (
	"context"
	"time"

	dashboardseeder "github.com/garda-inflasi/api/internal/dashboard/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "dashboard.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := dashboardseeder.SeedReference(ctx, db)
			return seedkit.Result{"target": sum.Target}, err
		},
	})

	// Agregat dihitung dari data lain, jadi seeder ini berjalan setelah semuanya.
	seedkit.Register(seedkit.Seeder{
		Name:      "dashboard.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"dashboard.reference"},
		After:     []string{"auth.demo", "harga.demo", "lapangan.demo", "coldstorage.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := dashboardseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"siklus": sum.Siklus}, err
		},
	})
}
