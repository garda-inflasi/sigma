package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/intervensi/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]domain.KartuIntervensi, error) {
	var kartu []domain.KartuIntervensi
	if err := r.db.WithContext(ctx).Order("urutan ASC, dibuat_pada DESC").Find(&kartu).Error; err != nil {
		return nil, fmt.Errorf("list kartu intervensi: %w", err)
	}
	return kartu, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*domain.KartuIntervensi, error) {
	var kartu domain.KartuIntervensi
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&kartu).Error; err != nil {
		return nil, fmt.Errorf("get kartu intervensi by id: %w", err)
	}
	return &kartu, nil
}

func (r *Repository) Create(ctx context.Context, kartu *domain.KartuIntervensi) error {
	return r.db.WithContext(ctx).Create(kartu).Error
}

func (r *Repository) UpdateTahap(ctx context.Context, id int64, tahap domain.TahapEnum, urutan int) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.KartuIntervensi{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"tahap":         tahap,
			"urutan":        urutan,
			"diupdate_pada": now,
		}).Error
}

func (r *Repository) BulkAction(ctx context.Context, ids []int64, action domain.BulkAction) (int64, error) {
	tx := r.db.WithContext(ctx).Model(&domain.KartuIntervensi{}).Where("id IN ?", ids)

	switch action {
	case domain.BulkArsip:
		tx = tx.Update("tahap", domain.TahapBatal)
	case domain.BulkHapus:
		tx = r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&domain.KartuIntervensi{})
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}

	if tx.Error != nil {
		return 0, fmt.Errorf("bulk action %s: %w", action, tx.Error)
	}

	return tx.RowsAffected, nil
}
