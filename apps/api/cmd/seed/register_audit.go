package main

import (
	"context"
	"time"

	auditseeder "github.com/garda-inflasi/api/internal/audit/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "audit.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"auth.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := auditseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"entri": sum.Entri}, err
		},
	})
}
