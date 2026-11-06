package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/garda-inflasi/api/internal/auth/domain"
	"gorm.io/gorm"
)

// Batas jumlah entri riwayat peran satu permintaan.
const (
	LimitBawaanRiwayatPeran = 50
	MaksLimitRiwayatPeran   = 200
)

// RiwayatPeranResult adalah riwayat peran satu pengguna, terbaru lebih dulu.
type RiwayatPeranResult struct {
	Items []domain.PerubahanPeran
	// Terpotong true bila ada entri lebih lama yang tidak ikut karena batas limit.
	Terpotong bool
}

// RiwayatPeran mengembalikan riwayat perubahan peran satu pengguna (pengguna.kelola), dibaca dari jejak
// audit: pembuatan akun dengan peran awalnya, PUT /roles, PATCH yang mengubah peran, dan aksi lain yang
// mencatat peran sebelum/sesudah pada entitas pengguna. Entri yang peran sesudahnya sama dengan sebelumnya
// (mis. PATCH nama saja) bukan perubahan peran dan tidak ikut. Pengguna yang tidak ada: ErrUserNotFound.
func (uc *UseCase) RiwayatPeran(ctx context.Context, id string, limit int) (*RiwayatPeranResult, error) {
	if _, err := uc.repo.GetUserByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if limit < 1 {
		limit = LimitBawaanRiwayatPeran
	}
	limit = min(limit, MaksLimitRiwayatPeran)

	// Satu baris ekstra menunjukkan masih ada entri yang lebih lama.
	items, err := uc.repo.RiwayatPeran(ctx, id, limit+1)
	if err != nil {
		return nil, fmt.Errorf("riwayat peran: %w", err)
	}
	hasil := &RiwayatPeranResult{Items: items}
	if len(items) > limit {
		hasil.Items, hasil.Terpotong = items[:limit], true
	}
	if hasil.Items == nil {
		hasil.Items = []domain.PerubahanPeran{}
	}
	return hasil, nil
}
