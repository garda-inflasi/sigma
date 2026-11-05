package main

import (
	"context"
	"time"

	intervensiseeder "github.com/garda-inflasi/api/internal/intervensi/seeder"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
)

func init() {
	seedkit.Register(seedkit.Seeder{
		Name: "intervensi.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := intervensiseeder.SeedReference(ctx, db)
			return seedkit.Result{"tahap": sum.Tahap, "alur": sum.Alur, "langkah_alur": sum.LangkahAlur}, err
		},
	})

	seedkit.Register(seedkit.Seeder{
		Name:      "intervensi.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"intervensi.reference", "master.reference", "auth.demo", "ews.demo"},
		Run: func(ctx context.Context, db *gorm.DB, now time.Time) (seedkit.Result, error) {
			sum, err := intervensiseeder.SeedDemo(ctx, db, now)
			return seedkit.Result{"kartu": sum.Kartu, "penanggung_jawab": sum.PenanggungJawab, "riwayat": sum.Riwayat, "persetujuan": sum.Persetujuan}, err
		},
	})
}
