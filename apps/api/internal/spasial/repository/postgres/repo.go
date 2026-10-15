package postgres

import (
	"context"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListWilayah(ctx context.Context) ([]domain.Wilayah, error) {
	var items []domain.Wilayah
	if err := r.db.WithContext(ctx).Order("kode ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
