package postgres

import (
	"context"
	"fmt"

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

func (r *Repository) GetByID(ctx context.Context, id int64) (*domain.TugasLapangan, error) {
	var item domain.TugasLapangan
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, fmt.Errorf("get tugas lapangan by id: %w", err)
	}
	return &item, nil
}

func (r *Repository) BulkAction(ctx context.Context, ids []int64, action domain.BulkAction) (int64, error) {
	switch action {
	case domain.BulkUnduhDataAcuan:
		// Data acuan download is a read-only operation; report affected rows.
		return int64(len(ids)), nil
	case domain.BulkTandaiSelesai:
		tx := r.db.WithContext(ctx).Model(&domain.TugasLapangan{}).
			Where("id IN ?", ids).
			Update("status", "SELESAI")
		if tx.Error != nil {
			return 0, fmt.Errorf("tandai selesai: %w", tx.Error)
		}
		return tx.RowsAffected, nil
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}
}
