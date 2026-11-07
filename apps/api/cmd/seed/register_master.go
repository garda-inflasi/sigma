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
				"satuan": sum.Satuan, "provinsi": sum.Provinsi, "kabupaten": sum.Kabupaten, "kecamatan": sum.Kecamatan,
				"pasar": sum.Pasar, "komoditas": sum.Komoditas,
			}, err
		},
	})

	// Pedagang demo diturunkan dari kode_pedagang pada entri harga demo, jadi harus sesudah harga.demo.
	seedkit.Register(seedkit.Seeder{
		Name:      "master.demo",
		Tier:      seedkit.TierDemo,
		DependsOn: []string{"master.reference", "harga.demo"},
		Run: func(ctx context.Context, db *gorm.DB, _ time.Time) (seedkit.Result, error) {
			sum, err := masterseeder.SeedDemo(ctx, db)
			return seedkit.Result{"pedagang": sum.Pedagang}, err
		},
	})
}
