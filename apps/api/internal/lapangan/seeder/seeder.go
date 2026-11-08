// Package seeder mengisi tugas survei lapangan demo untuk tiap akun lapangan
// (koordinator, agregator, dan mahasiswa) di pasar utama kabupatennya, dengan
// campuran status dan tenggat relatif terhadap waktu seed. Pasar dan akun
// berasal dari seeder master.reference dan auth.demo.
package seeder

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/internal/seedkit"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DemoSummary melaporkan baris yang ditulis oleh SeedDemo.
type DemoSummary struct {
	Tugas int
}

// Petugas adalah akun lapangan beserta pasar utama di kabupatennya.
type Petugas struct {
	ID        string `gorm:"column:id"`
	PasarID   string `gorm:"column:pasar_id"`
	PasarNama string `gorm:"column:pasar_nama"`
}

// Input adalah data acuan yang dibaca dari basis data.
type Input struct {
	Petugas []Petugas
	// Pembuat adalah akun yang menugaskan; kosong bila belum ada.
	Pembuat string
}

type templat struct {
	Judul     string
	Komoditas []string
	Tipe      string
	// TenggatJam adalah tenggat relatif terhadap waktu seed (negatif = sudah lewat).
	TenggatJam    int
	Target        int
	JumlahSelesai int
	Status        string
}

// Lima tugas tiap petugas meniru contoh di mode mock: satu berjalan, dua belum
// mulai, satu terlambat, dan satu selesai.
var templates = []templat{
	{"Survei harga konsumen pagi", []string{"CBM-KRT", "CBR-RWT", "BWM", "BRS-MED", "TLR", "DAG-AYM"}, domain.TipeKonsumen, 2, 6, 4, domain.StatusBerjalan},
	{"Verifikasi ulang harga cabai", []string{"CBM-KRT", "CBR-RWT"}, domain.TipeKonsumen, 5, 2, 0, domain.StatusBelum},
	{"Harga produsen kelompok tani", []string{"CBM-KRT", "BWM", "TMT", "KTG"}, domain.TipeProdusen, 26, 4, 0, domain.StatusBelum},
	{"Survei harga konsumen sore", []string{"IKN-TGK", "TLR", "MYK", "GLA"}, domain.TipeKonsumen, -3, 4, 1, domain.StatusTerlambat},
	{"Survei harga konsumen pagi", []string{"CBM-KRT", "BWM", "BRS-MED", "TLR"}, domain.TipeKonsumen, -22, 4, 4, domain.StatusSelesai},
}

// SeedDemo mengisi tugas demo. Idempoten: kode tugas (TGS-<slug akun>-<urutan templat>) ditentukan oleh akun dan
// urutan templat, sehingga menjalankannya ulang menyegarkan tenggat relatif terhadap now tanpa menggandakan
// baris. Kodenya sama dengan ID lama, jadi basis data hasil migrasi 000111 dan basis data baru memberi hasil yang
// sama; ID BIGINT diberi basis data. Hanya untuk demo.
//
// Prasyarat: seeder master.reference dan auth.demo sudah dijalankan.
func SeedDemo(ctx context.Context, db *gorm.DB, now time.Time) (DemoSummary, error) {
	in, err := loadInput(ctx, db)
	if err != nil {
		return DemoSummary{}, err
	}
	tugas := Build(in, now)

	err = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kode"}},
		UpdateAll: true,
	}).Create(&tugas).Error
	if err != nil {
		return DemoSummary{}, fmt.Errorf("seed tugas lapangan: %w", err)
	}
	return DemoSummary{Tugas: len(tugas)}, nil
}

func loadInput(ctx context.Context, db *gorm.DB) (Input, error) {
	var in Input
	tx := db.WithContext(ctx)

	err := tx.Raw(`
		SELECT u.id, COALESCE(p.kode, p.id::text) AS pasar_id, p.nama AS pasar_nama
		  FROM pengguna u
		  JOIN LATERAL (
		       SELECT id, kode, nama FROM pasar
		        WHERE kabupaten_kode = u.kabupaten_kode AND aktif ORDER BY id LIMIT 1
		  ) p ON true
		 WHERE u.peran IN ('OPS_KOORDINATOR', 'OPS_AGREGATOR', 'OPS_MAHASISWA') AND u.status = 'AKTIF'
		 ORDER BY u.id`).Scan(&in.Petugas).Error
	if err != nil {
		return in, fmt.Errorf("baca petugas lapangan: %w", err)
	}
	if len(in.Petugas) == 0 {
		return in, fmt.Errorf("petugas lapangan belum ada; jalankan seeder master.reference dan auth.demo lebih dulu")
	}

	var pembuat []string
	if err := tx.Raw(`SELECT id FROM pengguna WHERE peran = 'MANAJER_PROGRAM' AND status = 'AKTIF' ORDER BY id LIMIT 1`).Scan(&pembuat).Error; err != nil {
		return in, fmt.Errorf("baca akun penugas: %w", err)
	}
	if len(pembuat) > 0 {
		in.Pembuat = pembuat[0]
	}
	return in, nil
}

// Build menyusun tugas demo. Hasilnya deterministik untuk Input dan now yang sama.
func Build(in Input, now time.Time) []domain.TugasLapangan {
	out := make([]domain.TugasLapangan, 0, len(in.Petugas)*len(templates))
	for _, p := range in.Petugas {
		for i, t := range templates {
			tenggat := now.Add(time.Duration(t.TenggatJam) * time.Hour).Truncate(time.Second)
			kode := fmt.Sprintf("TGS-%s-%d", seedkit.SlugPengguna(p.ID), i+1)
			tugas := domain.TugasLapangan{
				Kode:          &kode,
				Judul:         t.Judul,
				PasarID:       p.PasarID,
				PetugasID:     p.ID,
				Komoditas:     domain.KodeKomoditas(t.Komoditas),
				Tipe:          t.Tipe,
				Tenggat:       tenggat,
				Status:        t.Status,
				Target:        t.Target,
				DibuatPada:    tenggat.Add(-8 * time.Hour),
				JumlahSelesai: t.JumlahSelesai,
			}
			if in.Pembuat != "" {
				pembuat := in.Pembuat
				tugas.DibuatOlehID = &pembuat
			}
			if t.Status == domain.StatusSelesai {
				selesai := tenggat.Add(-time.Hour)
				tugas.SelesaiPada = &selesai
			}
			out = append(out, tugas)
		}
	}
	return out
}
