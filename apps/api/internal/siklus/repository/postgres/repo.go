package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/siklus/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetActiveByWilayah returns the currently active cycle for a region.
// Active means AkhirPada IS NULL (the cycle has not ended yet).
func (r *Repository) GetActiveByWilayah(ctx context.Context, wilayahKode string) (*domain.SiklusEkonomi, error) {
	var siklus domain.SiklusEkonomi
	if err := r.db.WithContext(ctx).
		Where("wilayah_kode = ? AND akhir_pada IS NULL", wilayahKode).
		Order("mulai_pada DESC").
		First(&siklus).Error; err != nil {
		return nil, fmt.Errorf("get active siklus by wilayah: %w", err)
	}
	return &siklus, nil
}
