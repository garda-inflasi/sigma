package seeder

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/garda-inflasi/api/internal/master/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DemoSummary melaporkan baris yang diusulkan oleh SeedDemo.
type DemoSummary struct {
	Pedagang int
}

// demoBatch menjaga satu INSERT jauh di bawah batas parameter PostgreSQL.
const demoBatch = 500

// namaDepanDemo dan namaBelakangDemo membentuk nama pedagang contoh. Hanya demo: nama rekaan, bukan orang nyata.
var (
	namaDepanDemo = []string{
		"Cut", "Teuku", "Tgk", "Nyak", "Ibu", "Bapak", "Pak", "Bu", "Abang", "Kak", "Dek", "Haji", "Hajjah", "Said", "Sofyan", "Zainal",
	}
	namaBelakangDemo = []string{
		"Aminah", "Bakri", "Dian", "Daud", "Erna", "Fauzi", "Gade", "Hasan", "Ismail", "Jamilah", "Kamal", "Latifah",
		"Mahdi", "Nurhayati", "Pocut", "Putra", "Qadri", "Rahmad", "Safwan", "Taufik", "Umar", "Yusnidar", "Zakiah", "Azhari",
	}
)

// NamaPedagangDemo memberi nama rekaan yang tetap untuk satu (pasar, kode): dua pemanggilan dengan masukan sama
// menghasilkan nama sama, jadi seeder deterministik.
func NamaPedagangDemo(pasarID, kode string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(pasarID + "|" + kode))
	v := h.Sum64()
	depan := namaDepanDemo[v%uint64(len(namaDepanDemo))]
	belakang := namaBelakangDemo[(v/uint64(len(namaDepanDemo)))%uint64(len(namaBelakangDemo))]
	return depan + " " + belakang
}

// SeedDemo mengisi tabel pedagang dari pasangan (pasar_id, kode_pedagang) yang berbeda pada entri_harga demo,
// dengan nama rekaan deterministik. Hanya menyisipkan (ON CONFLICT DO NOTHING): pedagang yang sudah ada, termasuk
// yang namanya diubah atau dinonaktifkan, tidak disentuh. Idempoten.
//
// Prasyarat: seeder harga.demo (yang mengisi entri_harga) sudah dijalankan.
func SeedDemo(ctx context.Context, db *gorm.DB) (DemoSummary, error) {
	var pasangan []struct {
		PasarID string `gorm:"column:pasar_id"`
		Kode    string `gorm:"column:kode"`
	}
	err := db.WithContext(ctx).Raw(`
		SELECT DISTINCT e.pasar_id AS pasar_id, e.kode_pedagang AS kode
		  FROM entri_harga e
		 WHERE e.kode_pedagang IS NOT NULL AND btrim(e.kode_pedagang) <> ''
		 ORDER BY 1, 2`).Scan(&pasangan).Error
	if err != nil {
		return DemoSummary{}, fmt.Errorf("baca pedagang dari entri harga: %w", err)
	}

	rows := make([]domain.Pedagang, len(pasangan))
	for i, p := range pasangan {
		nama := NamaPedagangDemo(p.PasarID, p.Kode)
		rows[i] = domain.Pedagang{PasarID: p.PasarID, Kode: p.Kode, Nama: &nama, Aktif: true}
	}
	if len(rows) == 0 {
		return DemoSummary{}, nil
	}

	err = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pasar_id"}, {Name: "kode"}},
		DoNothing: true,
	}).CreateInBatches(&rows, demoBatch).Error
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed pedagang: %w", err)
	}
	return DemoSummary{Pedagang: len(rows)}, nil
}
