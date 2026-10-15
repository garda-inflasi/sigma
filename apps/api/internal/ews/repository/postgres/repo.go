package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, limit int) ([]domain.EwsAlert, error) {
	var alerts []domain.EwsAlert
	if err := r.db.WithContext(ctx).
		Order("dibuat_pada DESC").
		Limit(limit).
		Find(&alerts).Error; err != nil {
		return nil, fmt.Errorf("list ews alerts: %w", err)
	}
	return alerts, nil
}
