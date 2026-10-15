package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/harga/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// List returns harga entries ordered by periode descending.
func (r *Repository) List(ctx context.Context, limit int, status string) ([]domain.EntriHarga, error) {
	var entries []domain.EntriHarga

	query := r.db.WithContext(ctx).
		Order("periode DESC, dibuat_pada DESC").
		Limit(limit)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("list entri harga: %w", err)
	}

	return entries, nil
}

// GetByID returns a harga entry by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.EntriHarga, error) {
	var entri domain.EntriHarga
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&entri).Error; err != nil {
		return nil, fmt.Errorf("get entri harga by id: %w", err)
	}
	return &entri, nil
}

// GetTren returns aggregated trend data grouped by periode.
func (r *Repository) GetTren(ctx context.Context, komoditas, wilayah string) ([]domain.TrenHarga, error) {
	var tren []domain.TrenHarga

	if err := r.db.WithContext(ctx).
		Model(&domain.EntriHarga{}).
		Select("periode, AVG(harga) AS harga_rerata, MIN(harga) AS harga_min, MAX(harga) AS harga_max, COUNT(*) AS jumlah_entri").
		Where("komoditas_kode = ? AND wilayah_kode = ?", komoditas, wilayah).
		Group("periode").
		Order("periode ASC").
		Find(&tren).Error; err != nil {
		return nil, fmt.Errorf("get tren harga: %w", err)
	}

	return tren, nil
}

// Create inserts a new harga entry.
func (r *Repository) Create(ctx context.Context, entri *domain.EntriHarga) error {
	return r.db.WithContext(ctx).Create(entri).Error
}

// BulkAction performs bulk operations on harga entries.
func (r *Repository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction, userID string) (int64, error) {
	now := time.Now()
	tx := r.db.WithContext(ctx).Model(&domain.EntriHarga{}).Where("id IN ?", ids)

	switch action {
	case domain.BulkValidasi:
		tx = tx.Updates(map[string]interface{}{
			"status":          "DIVALIDASI",
			"divalidasi_oleh": userID,
			"divalidasi_pada": now,
		})
	case domain.BulkPublikasi:
		tx = tx.Where("status = ?", "DIVALIDASI").
			Updates(map[string]interface{}{
				"status":           "DIPUBLIKASI",
				"dipublikasi_oleh": userID,
				"dipublikasi_pada": now,
			})
	case domain.BulkTolak:
		tx = tx.Updates(map[string]interface{}{
			"status": "DITOLAK",
		})
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}

	result := tx
	if result.Error != nil {
		return 0, fmt.Errorf("bulk action %s: %w", action, result.Error)
	}

	return result.RowsAffected, nil
}
