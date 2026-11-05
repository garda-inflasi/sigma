package seeder

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Rentang awal batas harga terhadap harga acuan komoditas: harga di bawah seperempat atau di atas
// tiga kali harga acuan hampir pasti salah satuan atau salah ketik.
const (
	batasMinRasio  = 0.25
	batasMaksRasio = 3.0
	// berlakuDariAwal adalah tanggal berlaku batas awal; tetap agar seeder deterministik.
	berlakuDariAwal = "2024-01-01"
)

// ReferenceSummary melaporkan hasil SeedReference.
type ReferenceSummary struct {
	// Batas adalah jumlah batas harga awal komoditas aktif setelah seeder berjalan.
	Batas int
}

// SeedReference menanam satu batas harga awal (berlaku untuk semua kabupaten) per komoditas aktif,
// diturunkan dari harga acuan master. Aman di produksi dan idempoten: batas yang sudah ada tidak
// ditimpa, sehingga perubahan oleh admin bertahan.
//
// Prasyarat: seeder master.reference sudah dijalankan.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	err := db.WithContext(ctx).Exec(`
		INSERT INTO batas_harga (id, komoditas_kode, kabupaten_kode, batas_min, batas_maks, berlaku_dari)
		SELECT 'BH-' || kode, kode, NULL,
		       GREATEST(100, round(harga_acuan * CAST(? AS numeric) / 100) * 100)::int,
		       (round(harga_acuan * CAST(? AS numeric) / 100) * 100)::int,
		       CAST(? AS date)
		  FROM komoditas
		 WHERE status = 'AKTIF'
		ON CONFLICT (id) DO NOTHING`, batasMinRasio, batasMaksRasio, berlakuDariAwal).Error
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("seed batas harga: %w", err)
	}

	var total int64
	err = db.WithContext(ctx).Raw(`
		SELECT count(*) FROM batas_harga b
		  JOIN komoditas k ON k.kode = b.komoditas_kode AND k.status = 'AKTIF'
		 WHERE b.kabupaten_kode IS NULL`).Scan(&total).Error
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("hitung batas harga: %w", err)
	}
	return ReferenceSummary{Batas: int(total)}, nil
}
