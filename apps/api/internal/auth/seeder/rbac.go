package seeder

import (
	"context"
	"fmt"
	"slices"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"github.com/garda-inflasi/api/internal/auth/usecase"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RBACSummary melaporkan baris yang ditulis SeedRBAC (0 pada eksekusi ulang tanpa perubahan katalog).
type RBACSummary struct {
	PeranBaru     int
	IzinBaru      int
	PenugasanBaru int
}

// SeedRBAC mengisi referensi RBAC (lapis reference, aman di produksi): 14 peran, 35 izin katalog, dan matriks
// bawaan peran x izin. Idempoten dan tidak pernah menimpa perubahan admin:
//
//   - roles dan permissions adalah katalog yang didefinisikan di kode (label, lapis, deskripsi, kelompok), jadi
//     dibaca ulang dari kode pada setiap seed; kolom milik admin (versi, diperbarui_*) tidak disentuh.
//   - role_permissions hanya diisi untuk peran yang BARU dibuat dan untuk izin yang BARU masuk katalog. Peran dan
//     izin yang sudah ada dibiarkan: baik izin yang ditambahkan admin maupun izin bawaan yang sengaja dicabut admin
//     tidak akan dikembalikan oleh seed berikutnya. ON CONFLICT DO NOTHING; tidak ada DELETE.
//
// Menambah izin ke katalog di rilis berikutnya karenanya otomatis memberikannya ke peran bawaannya sekali saja;
// sesudah itu matriks dikelola dari UI /pengguna.
func SeedRBAC(ctx context.Context, db *gorm.DB) (RBACSummary, error) {
	var sum RBACSummary
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var peranAda, izinAda []string
		if err := tx.Model(&domain.Peran{}).Pluck("kunci", &peranAda).Error; err != nil {
			return fmt.Errorf("baca peran: %w", err)
		}
		if err := tx.Model(&domain.Izin{}).Pluck("nama", &izinAda).Error; err != nil {
			return fmt.Errorf("baca izin: %w", err)
		}

		peran := usecase.DaftarPeran()
		peranBaru := map[string]bool{}
		baris := make([]domain.Peran, len(peran))
		for i, p := range peran {
			baris[i] = domain.Peran{Kunci: p.Kunci, Label: p.Label, Lapis: p.Lapis, Urutan: p.Urutan}
			if !slices.Contains(peranAda, p.Kunci) {
				peranBaru[p.Kunci] = true
				sum.PeranBaru++
			}
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "kunci"}},
			DoUpdates: clause.AssignmentColumns([]string{"label", "lapis", "urutan"}),
		}).Create(&baris).Error; err != nil {
			return fmt.Errorf("seed peran: %w", err)
		}

		katalog := usecase.KatalogIzin()
		izinBaru := map[string]bool{}
		izin := make([]domain.Izin, len(katalog))
		for i, k := range katalog {
			izin[i] = domain.Izin{Nama: k.Nama, Deskripsi: k.Deskripsi, Kelompok: k.Kelompok}
			if !slices.Contains(izinAda, k.Nama) {
				izinBaru[k.Nama] = true
				sum.IzinBaru++
			}
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "nama"}},
			DoUpdates: clause.AssignmentColumns([]string{"deskripsi", "kelompok"}),
		}).Create(&izin).Error; err != nil {
			return fmt.Errorf("seed izin: %w", err)
		}

		var penugasan []domain.PeranIzin
		for _, p := range peran {
			for _, nama := range usecase.IzinBawaan(p.Kunci) {
				if peranBaru[p.Kunci] || izinBaru[nama] {
					penugasan = append(penugasan, domain.PeranIzin{RoleKunci: p.Kunci, PermissionNama: nama})
				}
			}
		}
		if len(penugasan) == 0 {
			return nil
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&penugasan)
		if res.Error != nil {
			return fmt.Errorf("seed izin peran: %w", res.Error)
		}
		sum.PenugasanBaru = int(res.RowsAffected)
		return nil
	})
	if err != nil {
		return RBACSummary{}, err
	}
	return sum, nil
}
