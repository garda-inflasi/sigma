package main

import (
	"context"
	"time"

	coldstorage "github.com/garda-inflasi/api/internal/cold-storage/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:  "coldstorage.reference",
		Tier:  seedkit.TierReference,
		After: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := coldstorage.SeedReference(ctx, db)
			return seedkit.Result{"fasilitas": sum.Fasilitas, "komoditas_baru": sum.Komoditas}, err
		},
	})

	seedkit.Register(seedkit.Seeder{
		Name:      "coldstorage.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"coldstorage.reference"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := coldstorage.SeedDemo(ctx, db, now)
			return seedkit.Result{"lot": sum.Stok}, err
		},
	})
}
