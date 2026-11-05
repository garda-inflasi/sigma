// Package seeder mengisi data referensi ingestion: pemetaan alias sumber -> kode
// master (pemetaan_sumber). Pemetaan adalah konfigurasi milik operator, jadi
// SeedReference hanya menambah baris yang belum ada dan tidak menimpa pemetaan
// yang sudah diubah lewat `cmd/ingest pemetaan`.
package seeder

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Summary melaporkan baris yang diurus seeder.
type Summary struct {
	Pemetaan int
}

// SeedReference menanam pemetaan awal. Prasyarat: katalog sumber (sumber.reference).
// Idempoten: dijalankan ulang tidak menambah atau mengubah baris.
func SeedReference(ctx context.Context, db *gorm.DB) (Summary, error) {
	rows := Pemetaan()
	err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&rows, 200).Error
	if err != nil {
		return Summary{}, fmt.Errorf("seed pemetaan sumber: %w", err)
	}
	return Summary{Pemetaan: len(rows)}, nil
}
