package main

import (
	"context"
	"time"

	briefseeder "github.com/garda-inflasi/api/internal/policy-brief/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "brief.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"auth.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := briefseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"brief": sum.Brief}, err
		},
	})
}
