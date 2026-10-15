package postgres

import (
	"context"

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
