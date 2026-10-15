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
