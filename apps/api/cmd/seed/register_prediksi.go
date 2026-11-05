package main

import (
	"context"
	"time"

	prediksiseeder "github.com/garda-inflasi/api/internal/prediksi/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "prediksi.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		After:     []string{"harga.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := prediksiseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"model": sum.Model, "proyeksi": sum.Proyeksi}, err
		},
	})
}
