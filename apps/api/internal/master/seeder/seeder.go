// Package seeder mengisi data referensi master (provinsi, kabupaten/kota, pasar,
// komoditas). Aman dijalankan di produksi: hanya menimpa kolom referensi dan
// tidak menyentuh status, versi, atau kolom operasional.
package seeder

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/master/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Provinsi, Kabupaten, Pasar, Komoditas int
}

// SeedReference meng-upsert seluruh data referensi menurut kunci masing-masing.
// Idempoten. Urutan mengikuti kunci asing: provinsi, kabupaten, pasar.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	provinsi, kabupaten, pasar, komoditas := []domain.Provinsi{ProvinsiAceh}, Kabupaten(), Pasar(), Komoditas()

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		steps := []struct {
			name    string
			rows    any
			key     string
			columns []string
		}{
			{"provinsi", &provinsi, "kode", []string{"nama"}},
			{"kabupaten", &kabupaten, "kode", []string{"nama", "jenis", "ibukota", "lat", "lng", "provinsi_kode"}},
			{"pasar", &pasar, "id", []string{"nama", "kabupaten_kode", "lat", "lng", "radius_meter"}},
			{"komoditas", &komoditas, "kode", []string{"nama", "satuan", "kategori", "harga_acuan", "pilot", "shelf_life_hari"}},
		}
		for _, step := range steps {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: step.key}},
				DoUpdates: clause.AssignmentColumns(step.columns),
			}).Create(step.rows).Error
			if err != nil {
				return fmt.Errorf("seed %s: %w", step.name, err)
			}
		}
		return nil
	})
	return ReferenceSummary{Provinsi: len(provinsi), Kabupaten: len(kabupaten), Pasar: len(pasar), Komoditas: len(komoditas)}, err
}

// Kabupaten mengembalikan salinan data kabupaten/kota dengan kode provinsi terisi.
func Kabupaten() []domain.Kabupaten {
	rows := append([]domain.Kabupaten(nil), kabupatenAceh...)
	for i := range rows {
		rows[i].ProvinsiKode = ProvinsiAceh.Kode
	}
	return rows
}

// Pasar mengembalikan salinan data pasar.
func Pasar() []domain.Pasar {
	rows := append([]domain.Pasar(nil), pasarAceh...)
	for i := range rows {
		rows[i].Aktif = true
	}
	return rows
}

// Komoditas mengembalikan salinan data komoditas.
func Komoditas() []domain.Komoditas {
	return append([]domain.Komoditas(nil), komoditasAceh...)
}
