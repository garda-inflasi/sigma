package postgres

import (
	"context"

	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListSaya(ctx context.Context, pic string) ([]domain.TugasLapangan, error) {
	var items []domain.TugasLapangan
	q := r.db.WithContext(ctx).Order("dibuat_pada DESC")
	if pic != "" {
		q = q.Where("pic = ?", pic)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
