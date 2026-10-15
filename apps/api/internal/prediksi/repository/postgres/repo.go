package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/prediksi/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetByKode(ctx context.Context, kode string) (*domain.ProyeksiXai, error) {
	var proyeksi domain.ProyeksiXai
	if err := r.db.WithContext(ctx).Where("komoditas_kode = ?", kode).
		Order("dibuat_pada DESC").First(&proyeksi).Error; err != nil {
		return nil, fmt.Errorf("get proyeksi by kode: %w", err)
	}
	return &proyeksi, nil
}

func (r *Repository) List(ctx context.Context, page, limit int) ([]domain.ProyeksiXai, int64, error) {
	var items []domain.ProyeksiXai
	var total int64

	offset := (page - 1) * limit

	if err := r.db.WithContext(ctx).Model(&domain.ProyeksiXai{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count proyeksi: %w", err)
	}

	if err := r.db.WithContext(ctx).Offset(offset).Limit(limit).
		Order("dibuat_pada DESC").Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list proyeksi: %w", err)
	}

	return items, total, nil
}
