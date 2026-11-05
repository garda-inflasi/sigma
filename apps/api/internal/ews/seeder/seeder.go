// Package seeder mengisi aturan ambang EWS (referensi, aman di produksi) dan
// alert contoh (demo, hanya pengembangan). Komoditas dan kabupaten berasal dari
// seeder master.reference; pengguna pengakui dari auth.demo.
package seeder

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Aturan int
}

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Alert int
}

// pengakuiDemo adalah akun demo (auth.demo) yang mengakui alert contoh, bergantian.
var pengakuiDemo = []string{"manajer_risiko", "manajer_data", "koordinator_lapangan"}

// SeedReference meng-upsert aturan ambang awal. Idempoten. Hanya kolom ambang
// yang ditimpa; status aktif dan kanal yang sudah diatur admin dibiarkan.
//
// Prasyarat: seeder master.reference sudah dijalankan (komoditas ada).
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	aturan := BuildAturan()
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"komoditas_kode", "jenis", "ambang_waspada", "ambang_siaga", "ambang_kritis", "jendela_hari", "keterangan",
		}),
	}).Create(&aturan).Error
	if err != nil {
		return ReferenceSummary{}, fmt.Errorf("seed aturan ews: %w", err)
	}
	return ReferenceSummary{Aturan: len(aturan)}, nil
}

// SeedDemo meng-upsert alert contoh dengan waktu relatif terhadap now. Menjalankannya
// ulang mengembalikan alert ke keadaan awal tanpa menggandakan baris. Hanya untuk demo.
//
// Prasyarat: SeedReference, master.reference, dan auth.demo sudah dijalankan.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	alerts := BuildAlerts(now)
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"severity", "jenis", "judul", "pesan", "komoditas_kode", "kabupaten_kode", "nilai", "satuan_nilai",
			"skor_dampak", "status", "sumber", "dibuat_pada", "diakui_oleh_id", "diakui_pada", "aturan_id",
		}),
	}).Create(&alerts).Error
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed alert ews: %w", err)
	}
	return DemoSummary{Alert: len(alerts)}, nil
}

// BuildAturan menyusun baris aturan_ews.
func BuildAturan() []domain.AturanEws {
	out := make([]domain.AturanEws, len(aturanSeeds))
	for i, a := range aturanSeeds {
		kode := a.komoditas
		out[i] = domain.AturanEws{
			ID: aturanID(a.jenis, a.komoditas), KomoditasKode: &kode, Jenis: a.jenis,
			AmbangWaspada: a.ambang.waspada, AmbangSiaga: a.ambang.siaga, AmbangKritis: a.ambang.kritis,
			JendelaHari: a.jendela, Kanal: domain.Kanal{"DASHBOARD", "PUSH"}, Aktif: true, Keterangan: keteranganAmbang,
		}
	}
	return out
}

// BuildAlerts menyusun baris ews_alert dengan waktu relatif terhadap now. Severity
// mengikuti aturan ambang yang berlaku untuk komoditas dan jenisnya.
func BuildAlerts(now time.Time) []domain.EwsAlert {
	now = now.UTC()
	out := make([]domain.EwsAlert, len(alertSeeds))
	for i, s := range alertSeeds {
		dibuat := now.Add(-time.Duration(s.menitLalu) * time.Minute)
		a := domain.EwsAlert{
			ID: s.id, Severity: severity(s), Jenis: s.jenis, Judul: s.judul, Pesan: s.pesan,
			KabupatenKode: s.kabupaten, Nilai: s.nilai, SatuanNilai: satuan(s.jenis),
			SkorDampak: s.skor, Status: s.status, Sumber: s.sumber, DibuatPada: dibuat,
		}
		if s.komoditas != "" {
			kode := s.komoditas
			a.KomoditasKode = &kode
			if _, ok := cariAturan(s.jenis, s.komoditas); ok {
				id := aturanID(s.jenis, s.komoditas)
				a.AturanID = &id
			}
		}
		if s.status != domain.StatusBaru {
			pengakui := pengakuiDemo[i%len(pengakuiDemo)]
			diakui := dibuat.Add(12 * time.Minute)
			a.DiakuiOlehID, a.DiakuiPada = &pengakui, &diakui
		}
		out[i] = a
	}
	return out
}

func satuan(jenis string) string {
	if jenis == jenisCuaca {
		return "mm"
	}
	return "%"
}

func cariAturan(jenis, komoditas string) (aturanSeed, bool) {
	for _, a := range aturanSeeds {
		if a.jenis == jenis && a.komoditas == komoditas {
			return a, true
		}
	}
	return aturanSeed{}, false
}

// severity menurunkan tingkat dari nilai dan ambang aturan (komoditas tanpa aturan
// memakai ambang bawaan). Surplus dan anomali data selalu INFO. Kelangkaan stok
// diukur dari besar penurunan, jadi nilainya dibandingkan secara mutlak.
func severity(s alertSeed) string {
	if s.jenis == jenisSurplus || s.jenis == jenisData {
		return "INFO"
	}
	a := ambangBawaan
	if rule, ok := cariAturan(s.jenis, s.komoditas); ok {
		a = rule.ambang
	}
	v := math.Abs(s.nilai)
	switch {
	case v >= a.kritis:
		return "KRITIS"
	case v >= a.siaga:
		return "SIAGA"
	case v >= a.waspada:
		return "WASPADA"
	default:
		return "INFO"
	}
}
