package postgres

import (
	"context"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, limit int) ([]domain.AuditLog, error) {
	var items []domain.AuditLog
	if err := r.db.WithContext(ctx).Order("waktu DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
