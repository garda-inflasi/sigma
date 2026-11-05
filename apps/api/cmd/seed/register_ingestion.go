package main

import (
	"context"
	"time"

	ingestionseeder "github.com/garda-inflasi/api/internal/ingestion/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name:      "ingestion.reference",
		Tier:      seedkit.TierReference,
		DependsOn: []string{"sumber.reference"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := ingestionseeder.SeedReference(ctx, db)
			return seedkit.Result{"pemetaan": sum.Pemetaan}, err
		},
	})
	// Riwayat eksekusi dan data silver contoh (pemicu DEMO); tidak pernah menimpa hasil ingestion nyata.
	// Berjalan setelah harga contoh agar agregat gold memuat kedua sisi.
	seedkit.Register(seedkit.Seeder{
		Name:      "ingestion.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference", "sumber.reference"},
		After:     []string{"sumber.demo", "harga.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := ingestionseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"eksekusi": sum.Eksekusi, "mentah": sum.Mentah, "harga": sum.Harga, "cuaca": sum.Cuaca}, err
		},
	})
}
