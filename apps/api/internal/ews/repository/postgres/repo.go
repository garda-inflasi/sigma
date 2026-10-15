package postgres

import (
	"context"
	"fmt"
	"time"

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

func (r *Repository) GetByID(ctx context.Context, id int64) (*domain.EwsAlert, error) {
	var alert domain.EwsAlert
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&alert).Error; err != nil {
		return nil, fmt.Errorf("get ews alert by id: %w", err)
	}
	return &alert, nil
}

func (r *Repository) BulkAction(ctx context.Context, ids []int64, action domain.BulkAction, userID string) (int64, error) {
	now := time.Now()
	tx := r.db.WithContext(ctx).Model(&domain.EwsAlert{}).Where("id IN ?", ids)

	switch action {
	case domain.BulkAkui:
		tx = tx.Updates(map[string]interface{}{
			"status":         "DIAKUI",
			"diakui_oleh_id": userID,
			"diakui_pada":    now,
		})
	case domain.BulkEskalasi:
		tx = tx.Updates(map[string]interface{}{
			"status": "DIESKALASI",
		})
	case domain.BulkSelesai:
		tx = tx.Updates(map[string]interface{}{
			"status": "SELESAI",
		})
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}

	if tx.Error != nil {
		return 0, fmt.Errorf("bulk action %s: %w", action, tx.Error)
	}

	return tx.RowsAffected, nil
}
