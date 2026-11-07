// Package seeder mengisi data referensi master (satuan, provinsi, kabupaten/kota, kecamatan, pasar,
// komoditas). Kecamatan datang dari kecamatan_aceh.csv hasil impor BPS (lihat SUMBER.md). Aman
// dijalankan di produksi: hanya menimpa kolom referensi dan tidak menyentuh status, versi, atau kolom
// operasional. Komoditas dan pasar yang sudah diubah lewat persetujuan perubahan (versi > 1) tidak
// ditimpa lagi.
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
	Satuan, Provinsi, Kabupaten, Kecamatan, Pasar, Komoditas int
}

// SeedReference meng-upsert seluruh data referensi menurut kunci masing-masing.
// Idempoten. Urutan mengikuti kunci asing: satuan, provinsi, kabupaten, kecamatan, pasar, komoditas.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	provinsi, kabupaten, pasar, komoditas := []domain.Provinsi{ProvinsiAceh}, Kabupaten(), Pasar(), Komoditas()
	satuan := Satuan()

	kecamatan, err := Kecamatan()
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("seed kecamatan: %w", err)
	}

	ditulis := map[string]int{}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Seeder juga dijalankan tes migrasi pada skema lama (sebelum 000120/000121): tabel satuan dan kecamatan
		// serta kolom pasar.versi/kecamatan_kode baru dipakai bila sudah ada.
		var adaSatuan, adaKecamatan, versiPasar, versiKomoditas bool
		err := tx.Raw(`SELECT to_regclass('public.satuan') IS NOT NULL, to_regclass('public.kecamatan') IS NOT NULL,
		       EXISTS (SELECT 1 FROM information_schema.columns
		                WHERE table_schema = 'public' AND table_name = 'pasar' AND column_name = 'versi'),
		       EXISTS (SELECT 1 FROM information_schema.columns
		                WHERE table_schema = 'public' AND table_name = 'komoditas' AND column_name = 'versi')`).
			Row().Scan(&adaSatuan, &adaKecamatan, &versiPasar, &versiKomoditas)
		if err != nil {
			return fmt.Errorf("periksa skema master: %w", err)
		}
		steps := []struct {
			name    string
			rows    any
			key     string
			columns []string
			// hanyaVersiAwal: baris yang sudah diubah lewat persetujuan (versi > 1) tidak ditimpa seed.
			hanyaVersiAwal bool
		}{
			{"satuan", &satuan, "kode", []string{"nama"}, false},
			{"provinsi", &provinsi, "kode", []string{"nama"}, false},
			{"kabupaten", &kabupaten, "kode", []string{"nama", "jenis", "ibukota", "lat", "lng", "provinsi_kode"}, false},
			{"kecamatan", &kecamatan, "kode", []string{"nama", "kabupaten_kode", "kode_dagri"}, false},
			{"pasar", &pasar, "id", []string{"nama", "kabupaten_kode", "lat", "lng", "radius_meter"}, versiPasar},
			{"komoditas", &komoditas, "kode", []string{"nama", "satuan", "kategori", "harga_acuan", "pilot", "shelf_life_hari"}, versiKomoditas},
		}
		for _, step := range steps {
			if (step.name == "satuan" && !adaSatuan) || (step.name == "kecamatan" && !adaKecamatan) {
				continue
			}
			conflict := clause.OnConflict{
				Columns:   []clause.Column{{Name: step.key}},
				DoUpdates: clause.AssignmentColumns(step.columns),
			}
			if step.hanyaVersiAwal {
				conflict.Where = clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: step.name + ".versi = 1"}}}
			}
			q := tx.Clauses(conflict)
			if step.name == "pasar" && !versiPasar {
				q = q.Omit("kecamatan_kode", "versi") // kolom 000120 belum ada
			}
			err := q.Create(step.rows).Error
			if err != nil {
				return fmt.Errorf("seed %s: %w", step.name, err)
			}
			ditulis[step.name] = 1
		}
		return nil
	})
	jumlah := func(nama string, n int) int { return ditulis[nama] * n }
	return ReferenceSummary{
		Satuan: jumlah("satuan", len(satuan)), Provinsi: len(provinsi), Kabupaten: len(kabupaten),
		Kecamatan: jumlah("kecamatan", len(kecamatan)), Pasar: len(pasar), Komoditas: len(komoditas),
	}, err
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

// Satuan mengembalikan salinan katalog satuan.
func Satuan() []domain.Satuan {
	return append([]domain.Satuan(nil), satuanKomoditas...)
}
