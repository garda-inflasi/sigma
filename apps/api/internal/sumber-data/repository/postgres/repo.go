package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/sumber-data/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]domain.SumberData, error) {
	var items []domain.SumberData
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*domain.SumberData, error) {
	var item domain.SumberData
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, fmt.Errorf("get sumber data by id: %w", err)
	}
	return &item, nil
}

func (r *Repository) BulkAction(ctx context.Context, ids []int64, action domain.BulkAction) (int64, error) {
	var status string
	switch action {
	case domain.BulkJalankan:
		status = "AKTIF"
	case domain.BulkJeda:
		status = "JEDA"
	case domain.BulkLanjutkan:
		status = "AKTIF"
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}

	tx := r.db.WithContext(ctx).Model(&domain.SumberData{}).
		Where("id IN ?", ids).
		Update("status", status)
	if tx.Error != nil {
		return 0, fmt.Errorf("bulk action %s: %w", action, tx.Error)
	}
	return tx.RowsAffected, nil
}
