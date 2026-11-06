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
	// Referensi RBAC (Opsi B, 02-C): peran, katalog izin, dan matriks bawaan. Aman di produksi; tidak pernah
	// menimpa perubahan admin pada matriks (lihat authseeder.SeedRBAC).
	seedkit.Register(seedkit.Seeder{
		Name: "auth.reference",
		Tier: seedkit.TierReference,
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := authseeder.SeedRBAC(ctx, db)
			return seedkit.Result{"peran": sum.PeranBaru, "izin": sum.IzinBaru, "penugasan": sum.PenugasanBaru}, err
		},
	})

	seedkit.Register(seedkit.Seeder{
		Name: "auth.admin",
		Tier: seedkit.TierAdmin,
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
		Name:      "auth.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference", "auth.reference"},
		After:     []string{"auth.admin"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := authseeder.SeedDemo(ctx, db)
			return seedkit.Result{"akun": sum.Akun}, err
		},
	})
}
