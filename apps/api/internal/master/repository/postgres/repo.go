package postgres

import (
	"context"

	"github.com/garda-inflasi/api/internal/master/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListKomoditas(ctx context.Context) ([]domain.Komoditas, error) {
	var items []domain.Komoditas
	if err := r.db.WithContext(ctx).Order("kode ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
