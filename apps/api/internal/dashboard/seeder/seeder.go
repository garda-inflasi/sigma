// Package seeder mengisi data dashboard operasional dalam dua lapis: target
// indikator (referensi, aman di produksi) dan siklus ekonomi contoh beserta
// penyegaran agregat (demo). Agregat harga dan KPI tidak di-seed langsung; ia
// dihitung dari entri harga, stok cold storage, dan tugas yang sudah ada.
package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"github.com/garda-inflasi/api/internal/dashboard/refresher"
	siklusdomain "github.com/garda-inflasi/api/internal/siklus/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Target int
}

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Siklus int
}

// targetAwal adalah target indikator awal, berlaku sejak awal 2026. Angka ini
// asumsi pengembangan yang perlu dikonfirmasi pemilik program.
var targetAwal = []struct {
	indikator string
	target    float64
	arah      string
}{
	{domain.IndikatorValidasi, 85, domain.ArahNaik}, // persen entri tervalidasi
	{domain.IndikatorWaspada, 3, domain.ArahTurun},  // komoditas di atas ambang
	{domain.IndikatorCS, 75, domain.ArahNaik},       // persen kapasitas terisi
}

var periodeAwal = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// SeedReference mengisi target indikator. Idempoten: baris di-upsert menurut
// (indikator, periode_mulai). Aman dijalankan di produksi.
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	for _, t := range targetAwal {
		err := db.WithContext(ctx).Exec(`
			INSERT INTO target_kpi (indikator, periode_mulai, target, arah) VALUES (?, ?, ?, ?)
			ON CONFLICT (indikator, periode_mulai) DO UPDATE SET target = EXCLUDED.target, arah = EXCLUDED.arah`,
			t.indikator, periodeAwal, t.target, t.arah).Error
		if err != nil {
			return ReferenceSummary{}, fmt.Errorf("seed target kpi %s: %w", t.indikator, err)
		}
	}
	return ReferenceSummary{Target: len(targetAwal)}, nil
}

const (
	// kodeSiklusDemo adalah kunci idempotensi siklus contoh (kolom kode, UNIQUE).
	kodeSiklusDemo = "SE-11-DEMO"
	// hariTekanan: siklus TEKANAN contoh dimulai enam hari sebelum now, seperti skenario demo
	// (cabai melonjak sejak awal Oktober).
	hariTekanan = 6
)

// SeedDemo mengisi satu siklus TEKANAN aktif untuk Aceh lalu menyegarkan agregat
// dashboard dari data yang sudah di-seed. Idempoten: kode siklus tetap, sehingga
// tanggal mulai bergeser mengikuti now tanpa menggandakan baris. Hanya untuk demo.
//
// Prasyarat: master.reference; agar agregat terisi, jalankan juga harga.demo,
// coldstorage.demo, dan lapangan.demo lebih dulu.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	mulai := now.AddDate(0, 0, -hariTekanan).Truncate(time.Second)
	siklus := siklusdomain.SiklusEkonomi{
		Kode: new(kodeSiklusDemo), WilayahKode: siklusdomain.WilayahBawaan, Siklus: siklusdomain.SiklusTekanan, MulaiPada: mulai,
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kode"}},
		DoUpdates: clause.AssignmentColumns([]string{"wilayah_kode", "siklus", "mulai_pada", "akhir_pada"}),
	}).Create(&siklus).Error
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed siklus ekonomi: %w", err)
	}

	for _, view := range refresher.Views {
		if err := db.WithContext(ctx).Exec("SELECT garda_segarkan_mv(?, false)", view).Error; err != nil {
			return DemoSummary{}, fmt.Errorf("refresh %s: %w", view, err)
		}
	}
	return DemoSummary{Siklus: 1}, nil
}
