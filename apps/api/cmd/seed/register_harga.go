package main

import (
	"context"
	"time"

	hargaseeder "github.com/garda-inflasi/api/internal/harga/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "harga.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := hargaseeder.SeedReference(ctx, db)
			return seedkit.Result{"batas": sum.Batas}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "harga.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"auth.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := hargaseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"entri": sum.Entri, "riwayat": sum.Riwayat}, err
		},
	})
}
