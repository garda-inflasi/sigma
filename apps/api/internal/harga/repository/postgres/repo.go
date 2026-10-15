package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// List returns harga entries ordered by periode descending.
func (r *Repository) List(ctx context.Context, limit int) ([]domain.EntriHarga, error) {
	var entries []domain.EntriHarga

	if err := r.db.WithContext(ctx).
		Order("periode DESC, dibuat_pada DESC").
		Limit(limit).
		Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("list entri harga: %w", err)
	}

	return entries, nil
}

// GetTren returns aggregated trend data grouped by periode.
func (r *Repository) GetTren(ctx context.Context, komoditas, wilayah string) ([]domain.TrenHarga, error) {
	var tren []domain.TrenHarga

	if err := r.db.WithContext(ctx).
		Model(&domain.EntriHarga{}).
		Select("periode, AVG(harga) AS harga_rerata, MIN(harga) AS harga_min, MAX(harga) AS harga_max, COUNT(*) AS jumlah_entri").
		Where("komoditas_kode = ? AND wilayah_kode = ?", komoditas, wilayah).
		Group("periode").
		Order("periode ASC").
		Find(&tren).Error; err != nil {
		return nil, fmt.Errorf("get tren harga: %w", err)
	}

	return tren, nil
}
