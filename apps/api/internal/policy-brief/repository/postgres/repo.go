package postgres

import (
	"context"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]domain.PolicyBrief, error) {
	var items []domain.PolicyBrief
	if err := r.db.WithContext(ctx).Order("dibuat_pada DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) Create(ctx context.Context, pb *domain.PolicyBrief) error {
	return r.db.WithContext(ctx).Create(pb).Error
}
