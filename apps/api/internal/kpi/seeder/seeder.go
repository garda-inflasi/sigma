// Package seeder mengisi KPI dalam dua lapis. Lapis referensi (aman di produksi)
// menanam definisi dan target menurut PRD §12, tanpa baseline dan tanpa nilai.
// Lapis demo tidak menulis angka tulis tangan: ia menjalankan job penghitung
// pada data yang ada (harga, cold storage, intervensi) untuk 12 bulan terakhir.
package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/audit"
	"github.com/garda-inflasi/api/internal/kpi/repository/postgres"
	"github.com/garda-inflasi/api/internal/kpi/usecase"
	"github.com/garda-inflasi/api/pkg/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// bulanDemo adalah jumlah bulan (termasuk bulan berjalan) yang dihitung seeder demo.
const bulanDemo = 12

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Definisi, Target int
}

// SeedReference meng-upsert definisi KPI dan menyisipkan target bawaan. Target
// yang sudah ada tidak ditimpa: setelah ditetapkan pemangku kepentingan, ia
// milik data, bukan seeder. Idempoten.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	definisi, target := Definisi(), Target()
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "kode"}},
			DoUpdates: clause.AssignmentColumns([]string{"kelompok", "nama", "deskripsi", "rumus", "satuan", "arah_baik", "frekuensi", "sumber_query", "urutan"}),
		}).Create(&definisi).Error
		if err != nil {
			return fmt.Errorf("definisi: %w", err)
		}
		for _, t := range target {
			err := tx.Exec(`
				INSERT INTO kpi_target (kpi_kode, wilayah_kode, periode_mulai, target, ambang_waspada)
				VALUES (?, NULL, ?, ?, ?)
				ON CONFLICT (kpi_kode, (COALESCE(wilayah_kode, '')), periode_mulai) DO NOTHING`,
				t.KpiKode, t.PeriodeMulai, t.Target, t.AmbangWaspada).Error
			if err != nil {
				return fmt.Errorf("target %s: %w", t.KpiKode, err)
			}
		}
		return nil
	})
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("seed kpi reference: %w", err)
	}
	return ReferenceSummary{Definisi: len(definisi), Target: len(target)}, nil
}

// DemoSummary melaporkan hasil job penghitung yang dijalankan SeedDemo.
type DemoSummary struct {
	Baru, Direvisi, Tetap, TanpaDasar int
}

// SeedDemo menjalankan job penghitung untuk 12 bulan yang berakhir di bulan now.
// KPI yang datanya belum ada (mis. harga belum diisi) dilewati, bukan diisi 0.
// Idempoten: nilai yang sama tidak ditulis ulang. Prasyarat: definisi KPI
// (kpi.reference); sumber data (harga, cold storage, intervensi) dihitung bila ada.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	uc := usecase.New(postgres.New(db), audit.NewWriter(db), logger.NewDefault())
	lap, err := uc.Hitung(ctx, now.AddDate(0, -(bulanDemo-1), 0), now, now)
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed kpi demo: %w", err)
	}
	return DemoSummary{Baru: lap.Baru, Direvisi: lap.Direvisi, Tetap: lap.Tetap, TanpaDasar: lap.TanpaDasar}, nil
}
