package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
)

var _ domain.EksporRepository = (*Repository)(nil)

// terapkanFilterPengguna menambahkan kriteria filter ke kueri atas tabel pengguna. Satu-satunya tempat
// kriteria daftar pengguna ditulis: ListUsers (GET /v1/users) dan ListUsersUntukEkspor (GET /v1/users/ekspor)
// sama-sama memakainya, jadi ekspor selalu memuat persis yang tampil di daftar dengan filter yang sama.
// Kriteria baru (peran, status, dst.) ditambahkan di sini.
func terapkanFilterPengguna(q *gorm.DB, f domain.FilterPengguna) *gorm.DB {
	if f.Search != "" {
		pattern := "%" + strings.ToLower(f.Search) + "%"
		// peran bertipe enum PostgreSQL, jadi harus di-cast ke text sebelum LOWER().
		q = q.Where(
			"LOWER(nama) LIKE ? OR LOWER(email) LIKE ? OR LOWER(peran::text) LIKE ? OR LOWER(lembaga) LIKE ?",
			pattern, pattern, pattern, pattern,
		)
	}
	if len(f.IDs) > 0 {
		q = q.Where("id IN ?", f.IDs)
	}
	return q
}

func (r *Repository) ListUsersUntukEkspor(ctx context.Context, filter domain.FilterPengguna, batas int) ([]domain.Pengguna, bool, error) {
	if batas < 1 {
		return nil, false, fmt.Errorf("ekspor pengguna: batas harus minimal 1")
	}
	var users []domain.Pengguna
	q := terapkanFilterPengguna(r.db.WithContext(ctx).Model(&domain.Pengguna{}), filter)
	// Satu baris lebih dari batas hanya untuk mengetahui apakah hasilnya terpotong.
	if err := q.Order("dibuat_pada DESC, id").Limit(batas + 1).Find(&users).Error; err != nil {
		return nil, false, fmt.Errorf("ekspor pengguna: %w", err)
	}
	if len(users) > batas {
		return users[:batas], true, nil
	}
	return users, false, nil
}
