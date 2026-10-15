// Package postgres implements the master.Repository interface using PostgreSQL as the data store.
package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/master/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListKomoditas(ctx context.Context) ([]domain.Komoditas, error) {
	var items []domain.Komoditas
	if err := r.db.WithContext(ctx).Order("kode ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) ListWilayah(ctx context.Context, provinsiKode string) ([]domain.Kabupaten, error) {
	var items []domain.Kabupaten
	q := r.db.WithContext(ctx).Order("kode ASC")
	if provinsiKode != "" {
		q = q.Where("provinsi_kode = ?", provinsiKode)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) ListPasar(ctx context.Context, kabupatenKode string) ([]domain.Pasar, error) {
	var items []domain.Pasar
	q := r.db.WithContext(ctx).Order("id ASC")
	if kabupatenKode != "" {
		q = q.Where("kabupaten_kode = ?", kabupatenKode)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) BulkKomoditas(ctx context.Context, kode []string, action domain.BulkAction) (int64, error) {
	var status string
	switch action {
	case domain.BulkArsipkan:
		status = "ARSIP"
	case domain.BulkSetujui:
		status = "AKTIF"
	case domain.BulkAjukanPerubahan:
		// Ajukan perubahan is a workflow action; report affected rows only.
		return int64(len(kode)), nil
	default:
		return 0, fmt.Errorf("unknown bulk action: %s", action)
	}

	tx := r.db.WithContext(ctx).Model(&domain.Komoditas{}).
		Where("kode IN ?", kode).
		Update("status", status)
	if tx.Error != nil {
		return 0, fmt.Errorf("bulk komoditas %s: %w", action, tx.Error)
	}
	return tx.RowsAffected, nil
}
