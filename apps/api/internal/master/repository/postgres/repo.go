// Package postgres implements the master.Repository interface using PostgreSQL as the data store.
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/garda-inflasi/api/internal/master/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListKomoditas(ctx context.Context, q domain.ListQuery) ([]domain.Komoditas, int64, error) {
	var items []domain.Komoditas
	total, err := r.list(r.db.WithContext(ctx).Model(&domain.Komoditas{}), q, "kode", []string{"kode", "nama"}, &items)
	return items, total, err
}

func (r *Repository) ListWilayah(ctx context.Context, provinsiKode string, q domain.ListQuery) ([]domain.Kabupaten, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.Kabupaten{})
	if provinsiKode != "" {
		query = query.Where("provinsi_kode = ?", provinsiKode)
	}
	var items []domain.Kabupaten
	total, err := r.list(query, q, "kode", []string{"kode", "nama", "ibukota"}, &items)
	return items, total, err
}

func (r *Repository) ListPasar(ctx context.Context, kabupatenKode string, q domain.ListQuery) ([]domain.Pasar, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.Pasar{})
	if kabupatenKode != "" {
		query = query.Where("kabupaten_kode = ?", kabupatenKode)
	}
	var items []domain.Pasar
	total, err := r.list(query, q, "id", []string{"id", "nama"}, &items)
	return items, total, err
}

// list menerapkan pencarian dan paginasi pada query dasar, lalu mengisi dest.
// total adalah jumlah seluruh hasil pencarian, bukan hanya halaman ini.
func (r *Repository) list(query *gorm.DB, q domain.ListQuery, orderBy string, searchColumns []string, dest any) (int64, error) {
	if search := strings.TrimSpace(q.Search); search != "" {
		pattern := "%" + escapeLike(strings.ToLower(search)) + "%"
		conditions := make([]string, len(searchColumns))
		args := make([]any, len(searchColumns))
		for i, column := range searchColumns {
			conditions[i] = "LOWER(" + column + ") LIKE ?"
			args[i] = pattern
		}
		query = query.Where(strings.Join(conditions, " OR "), args...)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("hitung master: %w", err)
	}

	query = query.Order(orderBy + " ASC")
	if q.Limit > 0 {
		query = query.Limit(q.Limit).Offset((max(q.Page, 1) - 1) * q.Limit)
	}
	if err := query.Find(dest).Error; err != nil {
		return 0, fmt.Errorf("daftar master: %w", err)
	}
	return total, nil
}

func (r *Repository) BulkKomoditas(ctx context.Context, kode []string, action domain.BulkAction) (int64, error) {
	var status string
	switch action {
	case domain.BulkArsipkan:
		status = domain.StatusDiarsipkan
	case domain.BulkSetujui:
		status = domain.StatusAktif
	case domain.BulkAjukanPerubahan:
		status = domain.StatusMenungguPersetujuan
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

// escapeLike menetralkan karakter wildcard LIKE agar pencarian bersifat literal.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
