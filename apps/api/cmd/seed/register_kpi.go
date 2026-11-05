package main

import (
	"context"
	"time"

	kpiseeder "github.com/garda-inflasi/api/internal/kpi/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name: "kpi.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := kpiseeder.SeedReference(ctx, db)
			return seedkit.Result{"definisi": sum.Definisi, "target": sum.Target}, err
		},
	})
	seedkit.Register(seedkit.Seeder{
		Name:      "kpi.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference"},
		// Job menghitung dari data seeder lain; yang belum terdaftar atau kosong dilewati.
		After: []string{"kpi.reference", "harga.demo", "coldstorage.demo", "ews.demo", "intervensi.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := kpiseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"baru": sum.Baru, "direvisi": sum.Direvisi, "tetap": sum.Tetap, "tanpa_dasar": sum.TanpaDasar}, err
		},
	})
}
