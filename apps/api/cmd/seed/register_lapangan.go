package main

import (
	"context"
	"time"

	lapanganseeder "github.com/garda-inflasi/api/internal/lapangan/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "lapangan.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"auth.demo", "harga.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := lapanganseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"tugas": sum.Tugas}, err
		},
	})
}
