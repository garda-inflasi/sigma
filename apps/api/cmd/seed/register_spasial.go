package main

import (
	"context"
	"time"

	"github.com/garda-inflasi/api/internal/seedkit"
	spasialseeder "github.com/garda-inflasi/api/internal/spasial/seeder"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "spasial.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"coldstorage.reference"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := spasialseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"neraca": sum.Neraca, "rute": sum.Rute, "kandidat": sum.Kandidat, "geometri": sum.Geometri}, err
		},
	})
}
