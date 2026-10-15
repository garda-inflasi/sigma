package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAllFasilitas(ctx context.Context) ([]domain.FasilitasColdStorage, error) {
	var fasilitas []domain.FasilitasColdStorage
	if err := r.db.WithContext(ctx).Order("nama ASC").Find(&fasilitas).Error; err != nil {
		return nil, err
	}
	return fasilitas, nil
}

func (r *Repository) GetAllStok(ctx context.Context) ([]domain.StokColdStorage, error) {
	var stok []domain.StokColdStorage
	if err := r.db.WithContext(ctx).Order("masuk_pada DESC").Find(&stok).Error; err != nil {
		return nil, err
	}
	return stok, nil
}

func (r *Repository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction) (int64, error) {
	// Cold storage bulk actions are advisory workflow operations (distribution
	// planning / processing recommendations). They do not mutate stored state;
	// report the number of items processed.
	switch action {
	case domain.BulkRencanakanDistribusi, domain.BulkRekomendasiOlahan:
		var count int64
		if err := r.db.WithContext(ctx).Model(&domain.StokColdStorage{}).
			Where("id IN ?", ids).
			Count(&count).Error; err != nil {
			return 0, fmt.Errorf("bulk action %s: %w", action, err)
		}
		return count, nil
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}
}
