package main

import (
	"context"
	"time"

	masterseeder "github.com/garda-inflasi/api/internal/master/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name: "master.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := masterseeder.SeedReference(ctx, db)
			return seedkit.Result{
				"provinsi": sum.Provinsi, "kabupaten": sum.Kabupaten, "pasar": sum.Pasar, "komoditas": sum.Komoditas,
			}, err
		},
	})
}
