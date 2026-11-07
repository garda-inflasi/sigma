// Package seeder mengisi contoh policy brief (hanya demo). Bentuk dan isinya
// meniru mockBriefs di apps/web/src/lib/api/mock.ts supaya halaman tampil sama
// dengan mode mock. Produksi tidak punya brief contoh.
package seeder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"github.com/garda-inflasi/api/internal/seedkit"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PenyusunID adalah akun demo (seeder auth.demo) yang menyusun semua brief contoh.
var PenyusunID = seedkit.IDPengguna("manajer_kebijakan")

// DemoSummary melaporkan baris yang diusulkan oleh SeedDemo.
type DemoSummary struct {
	Brief int
}

type briefContoh struct {
	Periode     time.Time
	Judul       string
	Status      string
	MenitLalu   int // selisih waktu "diperbarui" terhadap now
	Halaman     int
	Ringkasan   string
	Rekomendasi []string
}

func bulan(tahun int, m time.Month) time.Time {
	return time.Date(tahun, m, 1, 0, 0, 0, 0, time.UTC)
}

var briefContohSeeds = []briefContoh{
	{
		Periode: bulan(2026, time.October), Status: domain.StatusDraf, MenitLalu: 45, Halaman: 8,
		Judul:     "Meredam tekanan harga cabai pascacurah hujan ekstrem di dataran tinggi Gayo",
		Ringkasan: "Inflasi tahunan Aceh masih dalam koridor sasaran, namun curah hujan ekstrem di sentra Gayo menekan pasokan cabai merah dan mendorong harga naik.",
		Rekomendasi: []string{
			"Siapkan operasi pasar cabai di Pasar Peunayong dan Lamdingin",
			"Petakan pasokan alternatif dari Bireuen dan Sumatera Utara",
			"Ajukan dukungan subsidi ongkos angkut ke Kanwil DJPb",
			"Perkuat pemantauan harga harian di 11 pasar sentinel",
			"Koordinasikan jadwal tanam ulang dengan dinas pertanian dataran tinggi",
		},
	},
	{
		Periode: bulan(2026, time.September), Status: domain.StatusDisetujui, MenitLalu: 9800, Halaman: 7,
		Judul:     "Optimalisasi cold storage untuk serapan surplus kentang dan tomat",
		Ringkasan: "Surplus panen kentang dan tomat menekan harga di tingkat petani; kapasitas cold storage perlu diarahkan untuk menyerap kelebihan pasokan.",
		Rekomendasi: []string{
			"Alihkan surplus ke cold storage dengan kapasitas tersisa terbesar",
			"Hubungkan agregator dengan pembeli antardaerah",
			"Tetapkan harga serap minimum sementara untuk kentang",
			"Pantau risiko food loss pada lot berumur simpan pendek",
		},
	},
	{
		Periode: bulan(2026, time.August), Status: domain.StatusDidistribusikan, MenitLalu: 52000, Halaman: 6,
		Judul:     "Stabilisasi harga beras menjelang panen gadu",
		Ringkasan: "Stok beras memadai untuk beberapa bulan konsumsi; harga diperkirakan stabil hingga panen gadu mulai masuk pasar.",
		Rekomendasi: []string{
			"Jaga ketersediaan stok Bulog di pasar rawan gejolak",
			"Percepat penyerapan gabah petani saat panen raya",
			"Lanjutkan pemantauan harga mingguan",
		},
	},
	{
		Periode: bulan(2026, time.July), Status: domain.StatusDidistribusikan, MenitLalu: 95000, Halaman: 9,
		Judul:     "Pola distribusi ikan tongkol di wilayah pesisir timur",
		Ringkasan: "Harga ikan tongkol di pesisir timur dipengaruhi cuaca laut dan rantai distribusi yang panjang menuju wilayah tengah.",
		Rekomendasi: []string{
			"Perpendek rantai distribusi dari pelabuhan ke pasar tengah",
			"Sediakan rantai dingin di titik pendaratan utama",
			"Sinkronkan jadwal pasokan dengan prakiraan cuaca laut",
			"Libatkan koperasi nelayan dalam pelaporan harga",
		},
	},
	{
		Periode: bulan(2026, time.June), Status: domain.StatusDidistribusikan, MenitLalu: 138000, Halaman: 10,
		Judul:     "Kesiapan pasokan pangan Iduladha di 23 kabupaten/kota",
		Ringkasan: "Permintaan daging, cabai, dan bawang meningkat menjelang Iduladha; kesiapan pasokan tiap kabupaten/kota perlu dipantau.",
		Rekomendasi: []string{
			"Petakan kabupaten/kota dengan neraca pangan defisit",
			"Jadwalkan operasi pasar sebelum puncak permintaan",
			"Amankan pasokan daging sapi dan ayam antardaerah",
			"Aktifkan peringatan dini untuk komoditas pilot",
			"Siapkan kanal pengaduan harga bagi masyarakat",
			"Evaluasi dampak kebijakan segera setelah hari raya",
		},
	},
}

// SeedDemo menyimpan brief contoh. Idempoten menurut kode (PB-YYYY-MM, kunci unik): brief yang sudah ada
// dibiarkan apa adanya (status dan isi yang sudah berubah tidak ditimpa), juga pada basis data hasil migrasi 000114
// yang barisnya sudah ber-UUID stabil dan ber-kode; satu-satunya pengecualian, brief contoh dari sebelum kolom konten
// ada diberi konten dan penanda DEMO. ID brief seed = idx.UUIDStabil("policy_brief", kode), sama dengan hasil migrasi.
// Prasyarat: akun manajer_kebijakan sudah ada (auth.demo).
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	var penyusun int64
	if err := db.WithContext(ctx).Table("pengguna").Where("id = ?", PenyusunID).Count(&penyusun).Error; err != nil {
		return DemoSummary{}, fmt.Errorf("seed brief demo: cek penyusun: %w", err)
	}
	if penyusun == 0 {
		return DemoSummary{}, errors.New("seed brief demo: akun manajer_kebijakan (" + PenyusunID + ") belum ada; jalankan auth.demo lebih dulu")
	}

	rows, err := BuildBrief(now)
	if err != nil {
		return DemoSummary{}, err
	}
	err = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kode"}},
		DoUpdates: clause.AssignmentColumns([]string{"konten", "sumber_narasi"}),
		Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "policy_brief.konten = '{}'::jsonb"}}},
	}).Create(&rows).Error
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed brief demo: %w", err)
	}
	return DemoSummary{Brief: len(rows)}, nil
}

// BuildBrief menyusun baris brief contoh secara deterministik terhadap now.
func BuildBrief(now time.Time) ([]domain.PolicyBrief, error) {
	rows := make([]domain.PolicyBrief, len(briefContohSeeds))
	for i, c := range briefContohSeeds {
		rekomendasi, err := json.Marshal(c.Rekomendasi)
		if err != nil {
			return nil, fmt.Errorf("seed brief demo: rekomendasi %s: %w", domain.KodeBrief(c.Periode), err)
		}
		konten, err := json.Marshal(domain.Konten{
			Label: domain.LabelDemo, Penyusun: domain.PenyusunDemo,
			Bagian: []domain.BagianKonten{
				{Kode: domain.BagianRingkasan, Judul: "Ringkasan", Butir: []string{c.Ringkasan}},
				{Kode: "rekomendasi", Judul: "Rekomendasi", Butir: c.Rekomendasi},
			},
			Lampiran: []domain.Fakta{},
		})
		if err != nil {
			return nil, fmt.Errorf("seed brief demo: konten %s: %w", domain.KodeBrief(c.Periode), err)
		}
		diperbarui := now.UTC().Add(-time.Duration(c.MenitLalu) * time.Minute)
		rows[i] = domain.PolicyBrief{
			ID: idx.UUIDStabil("policy_brief", domain.KodeBrief(c.Periode)), Kode: domain.KodeBrief(c.Periode),
			Periode: domain.LabelPeriode(c.Periode), Judul: c.Judul, Status: c.Status,
			Ringkasan: c.Ringkasan, Rekomendasi: string(rekomendasi), Halaman: c.Halaman, DisusunOlehID: PenyusunID,
			Konten: string(konten), SnapshotData: "{}", SumberNarasi: domain.PenyusunDemo,
			DibuatPada: diperbarui.Add(-72 * time.Hour), Diperbarui: diperbarui,
		}
	}
	return rows, nil
}
