package postgres

import (
	"context"
	"errors"
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
		return nil, fmt.Errorf("list sumber data: %w", err)
	}
	return items, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.SumberData, error) {
	var item domain.SumberData
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get sumber data by id: %w", err)
	}
	return &item, nil
}

// Dilanjutkan, sumber yang pernah berjalan menunggu eksekusi berikutnya (TERTUNDA);
// yang belum pernah berjalan kembali BELUM_AKTIF. Status SEHAT tidak dipulihkan
// karena tidak ada eksekusi yang membuktikannya.
const (
	sqlJeda = `UPDATE sumber_data SET status = 'DIJEDA'
		WHERE id IN ? AND status <> 'DIJEDA' RETURNING id`
	sqlLanjutkan = `UPDATE sumber_data
		SET status = (CASE WHEN terakhir_jalan IS NULL THEN 'BELUM_AKTIF' ELSE 'TERTUNDA' END)::status_sumber
		WHERE id IN ? AND status = 'DIJEDA' RETURNING id`
)

func (r *Repository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction) ([]string, error) {
	var query string
	switch action {
	case domain.BulkJeda:
		query = sqlJeda
	case domain.BulkLanjutkan:
		query = sqlLanjutkan
	default:
		return nil, fmt.Errorf("aksi tidak dikenal: %s", action)
	}

	changed := []string{}
	if err := r.db.WithContext(ctx).Raw(query, ids).Scan(&changed).Error; err != nil {
		return nil, fmt.Errorf("bulk %s: %w", action, err)
	}
	return changed, nil
}
