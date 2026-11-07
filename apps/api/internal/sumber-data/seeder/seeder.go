// Package seeder mengisi katalog sumber data dalam dua lapis. Lapis referensi
// (aman di produksi) mencatat identitas sumber dengan status jujur BELUM_AKTIF:
// belum ada pengambilan data nyata (05-B). Lapis demo menambahkan riwayat
// eksekusi tiruan agar tampilan setara mode mock.
package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Summary melaporkan baris yang ditulis seeder.
type Summary struct {
	Sumber int
}

// Kunci idempotensi katalog adalah kode (UNIQUE), bukan id: baris hasil migrasi data lama sudah berkode SRC-...
// dan ber-UUID yang sama dengan IDSumber.
//
// Kolom identitas katalog: satu-satunya yang boleh ditimpa SeedReference, supaya
// status dan statistik eksekusi hasil operasi tidak hilang.
var kolomReferensi = []string{"nama", "penyedia", "jenis", "cakupan", "jadwal_cron", "robots_txt_patuh", "atribusi", "url_dasar"}

// Kolom keadaan eksekusi: ditimpa SeedDemo.
var kolomDemo = []string{"status", "terakhir_jalan", "keberhasilan_7h", "latensi_ms"}

// SeedReference meng-upsert katalog sumber. Sumber baru berstatus BELUM_AKTIF
// tanpa statistik; sumber yang sudah ada hanya diperbarui identitasnya. Idempoten.
func SeedReference(ctx context.Context, db *gorm.DB) (Summary, error) {
	rows := Katalog()
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kode"}},
		DoUpdates: clause.AssignmentColumns(kolomReferensi),
	}).Create(&rows).Error
	if err != nil {
		return Summary{}, fmt.Errorf("seed sumber reference: %w", err)
	}
	return Summary{Sumber: len(rows)}, nil
}

// SeedDemo memberi tiap sumber katalog keadaan eksekusi tiruan (status, waktu
// jalan terakhir relatif terhadap now, keberhasilan 7 hari, latensi). Berdiri
// sendiri: sumber yang belum ada ikut dibuat. Idempoten dan deterministik.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (Summary, error) {
	rows := BuildDemo(now)
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kode"}},
		DoUpdates: clause.AssignmentColumns(kolomDemo),
	}).Create(&rows).Error
	if err != nil {
		return Summary{}, fmt.Errorf("seed sumber demo: %w", err)
	}
	return Summary{Sumber: len(rows)}, nil
}

// Katalog mengembalikan sumber dalam keadaan awal: BELUM_AKTIF, tanpa statistik.
func Katalog() []domain.SumberData {
	rows := make([]domain.SumberData, len(katalog))
	for i, k := range katalog {
		rows[i] = domain.SumberData{
			ID: IDSumber(k.Kode), Kode: k.Kode, Nama: k.Nama, Penyedia: k.Penyedia, Jenis: k.Jenis, Cakupan: k.Cakupan,
			JadwalCron: k.JadwalCron, Status: domain.StatusBelumAktif, RobotsTxtPatuh: k.RobotsTxtPatuh,
			Atribusi: k.Atribusi, URLDasar: k.URLDasar, Konfigurasi: k.Konfigurasi,
		}
	}
	return rows
}

// BuildDemo mengembalikan katalog lengkap dengan keadaan eksekusi demo.
func BuildDemo(now time.Time) []domain.SumberData {
	rows := Katalog()
	for i := range rows {
		d := demoKeadaan[rows[i].Kode]
		terakhir := now.Add(-time.Duration(d.MenitLalu) * time.Minute).UTC()
		rows[i].Status = d.Status
		rows[i].TerakhirJalan = &terakhir
		rows[i].Keberhasilan7h = new(d.Keberhasilan7h)
		rows[i].LatensiMs = d.LatensiMs
	}
	return rows
}
