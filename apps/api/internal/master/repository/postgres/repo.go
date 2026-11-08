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
	total, err := r.list(query, q, "id", []string{"kode", "nama"}, &items)
	if err != nil {
		return items, total, err
	}
	// Nama kecamatan bukan kolom pasar: diisi dengan satu kueri untuk halaman ini.
	return items, total, isiNamaKecamatan(r.db.WithContext(ctx), items)
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

func (r *Repository) StatusKomoditas(ctx context.Context, kode []string) (map[string]string, error) {
	var rows []struct {
		Kode   string
		Status string
	}
	err := r.db.WithContext(ctx).Model(&domain.Komoditas{}).
		Select("kode, status::text AS status").Where("kode IN ?", kode).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("status komoditas: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Kode] = row.Status
	}
	return out, nil
}

func (r *Repository) UbahStatusKomoditas(ctx context.Context, kode string, t domain.Transisi) (bool, error) {
	updates := map[string]any{"status": t.Ke}
	if t.NaikVersi {
		updates["versi"] = gorm.Expr("versi + 1")
	}
	tx := r.db.WithContext(ctx).Model(&domain.Komoditas{}).
		Where("kode = ? AND status::text IN ?", kode, t.Dari).
		Updates(updates)
	if tx.Error != nil {
		return false, fmt.Errorf("ubah status komoditas %s: %w", kode, tx.Error)
	}
	return tx.RowsAffected == 1, nil
}

// escapeLike menetralkan karakter wildcard LIKE agar pencarian bersifat literal.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
