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
//
// Kunci upsert adalah kode (ON CONFLICT (kode)), termasuk pasar sejak 000122. id BIGINT identity pada keenam tabel
// milik basis data: seeder tidak pernah menulisnya, dan baris yang sudah ada mempertahankan id-nya.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	provinsi, kabupaten, pasar, komoditas := []domain.Provinsi{ProvinsiAceh}, Kabupaten(), Pasar(), Komoditas()
	satuan := Satuan()

	kecamatan, err := Kecamatan()
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("seed kecamatan: %w", err)
	}

	ditulis := map[string]int{}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Seeder juga dijalankan tes migrasi pada skema lama (sebelum 000120/000121/000122): tabel satuan dan kecamatan
		// serta kolom pasar.versi/kecamatan_kode/kode baru dipakai bila sudah ada.
		var adaSatuan, adaKecamatan, versiPasar, versiKomoditas, adaKodePasar bool
		err := tx.Raw(`SELECT to_regclass('public.satuan') IS NOT NULL, to_regclass('public.kecamatan') IS NOT NULL,
		       EXISTS (SELECT 1 FROM information_schema.columns
		                WHERE table_schema = 'public' AND table_name = 'pasar' AND column_name = 'versi'),
		       EXISTS (SELECT 1 FROM information_schema.columns
		                WHERE table_schema = 'public' AND table_name = 'komoditas' AND column_name = 'versi'),
		       EXISTS (SELECT 1 FROM information_schema.columns
		                WHERE table_schema = 'public' AND table_name = 'pasar' AND column_name = 'kode')`).
			Row().Scan(&adaSatuan, &adaKecamatan, &versiPasar, &versiKomoditas, &adaKodePasar)
		if err != nil {
			return fmt.Errorf("periksa skema master: %w", err)
		}
		// Sebelum 000122 tabel master belum punya kolom id (identity) dan pasar.id berisi kode. Entitas ber-ID membuat
		// GORM menambahkan RETURNING id, jadi pada skema lama RETURNING diganti kunci tabel dan pasar memakai tipe lama.
		pasarKey, pasarRows := "kode", any(&pasar)
		if !adaKodePasar {
			pasarKey, pasarRows = "id", pasarLamaDari(pasar)
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
			{"kabupaten", &kabupaten, "kode", []string{"nama", "provinsi_kode", "jenis", "ibukota", "lat", "lng"}, false},
			{"kecamatan", &kecamatan, "kode", []string{"nama", "kabupaten_kode", "kode_dagri"}, false},
			{"pasar", pasarRows, pasarKey, []string{"nama", "kabupaten_kode", "lat", "lng", "radius_meter"}, versiPasar},
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
			if !adaKodePasar {
				q = q.Clauses(clause.Returning{Columns: []clause.Column{{Name: step.key}}})
			}
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

// pasarLama memetakan pasar sebelum 000122: id varchar berisi kode pasar dan belum ada kolom kode.
type pasarLama struct {
	ID            string  `gorm:"column:id;primaryKey"`
	Nama          string  `gorm:"column:nama"`
	KabupatenKode string  `gorm:"column:kabupaten_kode"`
	KecamatanKode *string `gorm:"column:kecamatan_kode"`
	Lat           float64 `gorm:"column:lat"`
	Lng           float64 `gorm:"column:lng"`
	RadiusMeter   int     `gorm:"column:radius_meter;default:400"`
	Aktif         bool    `gorm:"column:aktif;default:true"`
	Versi         int     `gorm:"column:versi;default:1"`
}

func (pasarLama) TableName() string { return "pasar" }

// pasarLamaDari mengubah pasar seed ke bentuk skema lama (kode menjadi id).
func pasarLamaDari(pasar []domain.Pasar) *[]pasarLama {
	rows := make([]pasarLama, len(pasar))
	for i, p := range pasar {
		rows[i] = pasarLama{
			ID: p.Kode, Nama: p.Nama, KabupatenKode: p.KabupatenKode, KecamatanKode: p.KecamatanKode,
			Lat: p.Lat, Lng: p.Lng, RadiusMeter: p.RadiusMeter, Aktif: p.Aktif, Versi: p.Versi,
		}
	}
	return &rows
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
