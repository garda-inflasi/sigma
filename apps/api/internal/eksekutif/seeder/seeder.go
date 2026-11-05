// Package seeder mengisi data dashboard eksekutif dalam dua lapis. Lapis
// referensi hanya memuat target inflasi (kebijakan, aman di produksi). Lapis
// demo memuat inflasi, andil, neraca pangan, dan ruang fiskal TIRUAN yang semuanya
// ber-sumber 'DEMO' dan tidak pernah menimpa baris resmi (BPS, BI, DJPB).
package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/eksekutif/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Target int
}

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Inflasi, Andil, Neraca, Fiskal int
}

// SeedReference meng-upsert target inflasi (koridor sasaran per tahun). Idempoten.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	rows := Target()
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tahun"}},
		DoUpdates: clause.AssignmentColumns([]string{"batas_bawah", "batas_atas"}),
	}).Create(&rows).Error
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("seed eksekutif reference: %w", err)
	}
	return ReferenceSummary{Target: len(rows)}, nil
}

// Target mengembalikan sasaran inflasi 2025–2027: 2,5% ± 1 (1,5–3,5%).
func Target() []domain.TargetInflasi {
	return []domain.TargetInflasi{
		{Tahun: 2025, BatasBawah: 1.5, BatasAtas: 3.5},
		{Tahun: 2026, BatasBawah: 1.5, BatasAtas: 3.5},
		{Tahun: 2027, BatasBawah: 1.5, BatasAtas: 3.5},
	}
}

// SeedDemo menulis data contoh relatif terhadap now: inflasi 12 bulan sampai
// bulan lalu, andil dan neraca bulan lalu, dan ruang fiskal per akhir bulan lalu.
// Prasyarat: komoditas (master.reference). Idempoten dan deterministik; baris
// yang sudah berasal dari sumber resmi tidak disentuh.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	inflasi, andil, neraca, fiskal := BuildInflasi(now), BuildAndil(now), BuildNeraca(now), BuildFiskal(now)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		steps := []struct {
			name  string
			table string
			rows  any
			key   []string
			cols  []string
		}{
			{"inflasi", "inflasi_bulanan", &inflasi, []string{"wilayah_kode", "periode"},
				[]string{"ihk", "inflasi_mtm", "inflasi_yoy", "inflasi_ytd"}},
			{"andil", "andil_inflasi", &andil, []string{"periode", "wilayah_kode", "komoditas_kode"},
				[]string{"andil", "inflasi_pct"}},
			{"neraca", "neraca_pangan", &neraca, []string{"komoditas_kode", "wilayah_kode", "periode"},
				[]string{"produksi_ton", "konsumsi_ton", "stok_ton"}},
			// Kunci alami, bukan id: baris resmi untuk instrumen dan tanggal yang sama
			// harus dilewati, bukan menabrak UNIQUE.
			{"fiskal", "ruang_fiskal", &fiskal, []string{"tahun", "instrumen", "per_tanggal"},
				[]string{"pagu", "realisasi"}},
		}
		for _, step := range steps {
			keys := make([]clause.Column, len(step.key))
			for i, k := range step.key {
				keys[i] = clause.Column{Name: k}
			}
			err := tx.Clauses(clause.OnConflict{
				Columns:   keys,
				DoUpdates: clause.AssignmentColumns(step.cols),
				// Angka resmi yang sudah diimpor tidak boleh tertimpa data contoh.
				Where: clause.Where{Exprs: []clause.Expression{
					clause.Expr{SQL: step.table + ".sumber = ?", Vars: []any{domain.SumberDemo}},
				}},
			}).Create(step.rows).Error
			if err != nil {
				return fmt.Errorf("seed %s: %w", step.name, err)
			}
		}
		return nil
	})
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed eksekutif demo: %w", err)
	}
	return DemoSummary{Inflasi: len(inflasi), Andil: len(andil), Neraca: len(neraca), Fiskal: len(fiskal)}, nil
}
