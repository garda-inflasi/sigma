package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/auth/domain"
)

var _ domain.EksporRepository = (*Repository)(nil)

// ListUsersUntukEkspor memakai penyaring daftar (terapkanFilterPengguna di repo_daftar_pengguna.go).
func (r *Repository) ListUsersUntukEkspor(ctx context.Context, filter domain.FilterPengguna, sekarang time.Time, batas int) ([]domain.Pengguna, bool, error) {
	if batas < 1 {
		return nil, false, fmt.Errorf("ekspor pengguna: batas harus minimal 1")
	}
	var users []domain.Pengguna
	q := terapkanFilterPengguna(r.db.WithContext(ctx).Model(&domain.Pengguna{}), filter, sekarang)
	// Satu baris lebih dari batas hanya untuk mengetahui apakah hasilnya terpotong.
	if err := q.Order("dibuat_pada DESC, id").Limit(batas + 1).Find(&users).Error; err != nil {
		return nil, false, fmt.Errorf("ekspor pengguna: %w", err)
	}
	if len(users) > batas {
		return users[:batas], true, nil
	}
	return users, false, nil
}
