package main

import (
	"context"
	"os"
	"time"

	authseeder "github.com/garda-inflasi/api/internal/auth/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:  "auth.admin",
		Tier:  seedkit.TierAdmin,
		After: []string{"master.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := authseeder.SeedAdmin(ctx, db, authseeder.AdminConfig{
				Email:    os.Getenv("SEED_ADMIN_EMAIL"),
				Password: os.Getenv("SEED_ADMIN_PASSWORD"),
				Output:   os.Stdout,
			})
			return seedkit.Result{"admin": sum.Dibuat}, err
		},
	})

	seedkit.Register(seedkit.Seeder{
		Name:  "auth.demo",
		Tier:  seedkit.TierDemo,
		After: []string{"master.reference", "auth.admin"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := authseeder.SeedDemo(ctx, db)
			return seedkit.Result{"akun": sum.Akun}, err
		},
	})
}
