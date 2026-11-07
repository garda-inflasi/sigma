// Package seeder mengisi aturan ambang EWS (referensi, aman di produksi) dan
// alert contoh (demo, hanya pengembangan). Komoditas dan kabupaten berasal dari
// seeder master.reference; pengguna pengakui dari auth.demo.
//
// ID baris (BIGINT) dibuat basis data. Idempotensi memakai kolom kode: ATR-... untuk aturan, EWS-... untuk
// alert, dan RW-D-<kode alert>-<n> untuk riwayat demo. Basis data hasil migrasi 000112 sudah memuat kode itu
// (ID lama), jadi seed tidak menggandakan baris.
package seeder

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"github.com/garda-inflasi/api/internal/seedkit"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReferenceSummary melaporkan baris yang ditulis oleh SeedReference.
type ReferenceSummary struct {
	Aturan int
}

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Alert, Riwayat int
}

// Selang waktu antar peristiwa pada riwayat alert demo.
const (
	selangEskalasi = 30 * time.Minute
	selangSelesai  = 60 * time.Minute
)

// cooldownBawaanJam adalah peredaman bawaan setelah alert ditutup.
const cooldownBawaanJam = 24

// pengakuiDemo adalah akun demo (auth.demo) yang mengakui alert contoh, bergantian.
var pengakuiDemo = []string{
	seedkit.IDPengguna("manajer_risiko"), seedkit.IDPengguna("manajer_data"), seedkit.IDPengguna("koordinator_lapangan"),
}

// SeedReference meng-upsert aturan ambang awal. Idempoten. Aturan yang sudah
// diubah lewat UI (versi > 1) tidak disentuh sama sekali; yang belum diubah
// mengikuti nilai seed, kecuali status aktif dan kanal yang dibiarkan.
//
// Prasyarat: seeder master.reference sudah dijalankan (komoditas ada).
func SeedReference(ctx context.Context, db *gorm.DB) (ReferenceSummary, error) {
	aturan := BuildAturan()
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "kode"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"komoditas_kode", "jenis", "ambang_waspada", "ambang_siaga", "ambang_kritis", "jendela_hari", "cooldown_jam", "keterangan",
		}),
		Where: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "aturan_ews.versi = 1"}}},
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
	var jumlahRiwayat int
	alerts := BuildAlerts(now)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := isiAturanAlert(tx, alerts); err != nil {
			return err
		}
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "kode"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"severity", "jenis", "judul", "pesan", "komoditas_kode", "kabupaten_kode", "nilai", "satuan_nilai",
				"skor_dampak", "status", "sumber", "dibuat_pada", "diakui_oleh_id", "diakui_pada", "aturan_id",
				"dieskalasi_pada", "dieskalasi_oleh_id", "selesai_pada",
			}),
		}).Create(&alerts).Error
		if err != nil {
			return fmt.Errorf("seed alert ews: %w", err)
		}
		// Create mengisi ID alert (RETURNING, juga untuk baris yang sudah ada), dipakai riwayat di bawah.
		riwayat := BuildRiwayat(alerts)
		err = tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "kode"}},
			DoUpdates: clause.AssignmentColumns([]string{"alert_id", "dari", "ke", "oleh", "waktu", "catatan"}),
		}).Create(&riwayat).Error
		if err != nil {
			return fmt.Errorf("seed riwayat alert ews: %w", err)
		}
		jumlahRiwayat = len(riwayat)
		return nil
	})
	if err != nil {
		return DemoSummary{}, err
	}
	return DemoSummary{Alert: len(alerts), Riwayat: jumlahRiwayat}, nil
}

// isiAturanAlert mengisi alert.AturanID dari kode aturan yang memicunya (BuildAlerts tidak tahu ID basis data).
// Aturan harus sudah ada (SeedReference).
func isiAturanAlert(tx *gorm.DB, alerts []domain.EwsAlert) error {
	var rows []struct {
		ID   idx.ID
		Kode string
	}
	if err := tx.Raw("SELECT id, kode FROM aturan_ews WHERE kode IS NOT NULL").Scan(&rows).Error; err != nil {
		return fmt.Errorf("baca aturan ews: %w", err)
	}
	peta := make(map[string]idx.ID, len(rows))
	for _, r := range rows {
		peta[r.Kode] = r.ID
	}
	for i := range alerts {
		kode, ada := AturanKodeAlert(alerts[i])
		if !ada {
			continue
		}
		id, ok := peta[kode]
		if !ok {
			return fmt.Errorf("seed alert ews: aturan %s belum ada (jalankan ews.reference lebih dulu)", kode)
		}
		alerts[i].AturanID = &id
	}
	return nil
}

// BuildAturan menyusun baris aturan_ews (tanpa ID: basis data yang membuatnya; Kode = ATR-...).
func BuildAturan() []domain.AturanEws {
	out := make([]domain.AturanEws, len(aturanSeeds))
	for i, a := range aturanSeeds {
		komoditas := a.komoditas
		kode := aturanKode(a.jenis, a.komoditas)
		out[i] = domain.AturanEws{
			Kode: &kode, KomoditasKode: &komoditas, Jenis: a.jenis,
			AmbangWaspada: a.ambang.waspada, AmbangSiaga: a.ambang.siaga, AmbangKritis: a.ambang.kritis,
			JendelaHari: a.jendela, CooldownJam: cooldownBawaanJam, Kanal: domain.Kanal{"DASHBOARD", "PUSH"}, Aktif: true,
			Keterangan: keteranganAmbang, Versi: 1,
		}
	}
	return out
}

// BuildAlerts menyusun baris ews_alert dengan waktu relatif terhadap now. Severity
// mengikuti aturan ambang yang berlaku untuk komoditas dan jenisnya. ID dan AturanID dikosongkan: ID dibuat
// basis data, dan AturanID diisi SeedDemo dari kode aturan (AturanKodeAlert).
func BuildAlerts(now time.Time) []domain.EwsAlert {
	now = now.UTC()
	out := make([]domain.EwsAlert, len(alertSeeds))
	for i, s := range alertSeeds {
		dibuat := now.Add(-time.Duration(s.menitLalu) * time.Minute)
		kode := s.kode
		a := domain.EwsAlert{
			Kode: &kode, Severity: severity(s), Jenis: s.jenis, Judul: s.judul, Pesan: s.pesan,
			KabupatenKode: s.kabupaten, Nilai: s.nilai, SatuanNilai: satuan(s.jenis),
			SkorDampak: s.skor, Status: s.status, Sumber: s.sumber, DibuatPada: dibuat,
		}
		if s.komoditas != "" {
			komoditas := s.komoditas
			a.KomoditasKode = &komoditas
		}
		if s.status != domain.StatusBaru {
			pengakui := pengakuiDemo[i%len(pengakuiDemo)]
			diakui := dibuat.Add(12 * time.Minute)
			a.DiakuiOlehID, a.DiakuiPada = &pengakui, &diakui
			switch s.status {
			case domain.StatusDieskalasi:
				pada := diakui.Add(selangEskalasi)
				a.DieskalasiPada, a.DieskalasiOlehID = &pada, &pengakui
			case domain.StatusSelesai:
				pada := diakui.Add(selangSelesai)
				a.SelesaiPada = &pada
			}
		}
		out[i] = a
	}
	return out
}

// AturanKodeAlert mengembalikan kode aturan (ATR-...) yang memicu alert contoh itu menurut Kode alert;
// false bila alert tidak punya aturan (tanpa komoditas, atau jenis tanpa aturan seed).
func AturanKodeAlert(a domain.EwsAlert) (string, bool) {
	if a.Kode == nil {
		return "", false
	}
	for _, s := range alertSeeds {
		if s.kode != *a.Kode || s.komoditas == "" {
			continue
		}
		if _, ok := cariAturan(s.jenis, s.komoditas); ok {
			return aturanKode(s.jenis, s.komoditas), true
		}
	}
	return "", false
}

// BuildRiwayat menyusun riwayat status alert demo: dibuat model, lalu diakui, dieskalasi
// atau diselesaikan sesuai status akhirnya. alerts harus sudah punya ID (hasil insert). Kode deterministik
// (RW-D-<kode alert>-<n>) agar seed dapat diulang.
func BuildRiwayat(alerts []domain.EwsAlert) []domain.EwsRiwayat {
	var out []domain.EwsRiwayat
	urut := map[idx.ID]int{}
	tambah := func(a domain.EwsAlert, dari *string, ke string, oleh *string, waktu time.Time, catatan string) {
		kode := fmt.Sprintf("RW-D-%s-%d", *a.Kode, urut[a.ID])
		out = append(out, domain.EwsRiwayat{
			Kode: &kode, AlertID: a.ID, Dari: dari, Ke: ke, Oleh: oleh, Waktu: waktu, Catatan: catatan,
		})
		urut[a.ID]++
	}
	ptr := func(s string) *string { return &s }
	for _, a := range alerts {
		tambah(a, nil, domain.StatusBaru, nil, a.DibuatPada, "Dibuat oleh model EWS")
		if a.DiakuiPada != nil {
			tambah(a, ptr(domain.StatusBaru), domain.StatusDiakui, a.DiakuiOlehID, *a.DiakuiPada, "")
		}
		if a.DieskalasiPada != nil {
			tambah(a, ptr(domain.StatusDiakui), domain.StatusDieskalasi, a.DieskalasiOlehID, *a.DieskalasiPada, "")
		}
		if a.SelesaiPada != nil {
			tambah(a, ptr(domain.StatusDiakui), domain.StatusSelesai, a.DiakuiOlehID, *a.SelesaiPada, "")
		}
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
