// Package postgres adalah repository EWS di atas PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/ews/domain"
	"gorm.io/gorm"
)

// bulkSQL menerapkan satu aksi massal; WHERE status membatasi transisi yang sah
// (BARU -> DIAKUI, BARU/DIAKUI -> DIESKALASI, selain SELESAI -> SELESAI).
var bulkSQL = map[domain.BulkAction]string{
	domain.BulkAkui: `UPDATE ews_alert
		   SET status = 'DIAKUI', diakui_oleh_id = @user, diakui_pada = NOW()
		 WHERE id IN @ids AND status = 'BARU'
		RETURNING id`,
	domain.BulkEskalasi: `UPDATE ews_alert
		   SET status = 'DIESKALASI'
		 WHERE id IN @ids AND status IN ('BARU', 'DIAKUI')
		RETURNING id`,
	domain.BulkSelesai: `UPDATE ews_alert
		   SET status = 'SELESAI'
		 WHERE id IN @ids AND status <> 'SELESAI'
		RETURNING id`,
}

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// alertQuery menggabungkan nama komoditas dan kabupaten.
func (r *Repository) alertQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("ews_alert AS a").
		Select("a.*, k.nama AS komoditas_nama, w.nama AS kabupaten_nama").
		Joins("LEFT JOIN komoditas k ON k.kode = a.komoditas_kode").
		Joins("JOIN kabupaten w ON w.kode = a.kabupaten_kode")
}

func (r *Repository) List(ctx context.Context, q domain.ListQuery) ([]domain.Alert, int64, error) {
	filtered := r.db.WithContext(ctx).Table("ews_alert AS a")
	if q.Status != "" {
		filtered = filtered.Where("a.status = ?", q.Status)
	}
	if q.Severity != "" {
		filtered = filtered.Where("a.severity = ?", q.Severity)
	}
	if q.Jenis != "" {
		filtered = filtered.Where("a.jenis = ?", q.Jenis)
	}
	if q.KabupatenKode != "" {
		filtered = filtered.Where("a.kabupaten_kode = ?", q.KabupatenKode)
	}

	var total int64
	if err := filtered.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("hitung alert ews: %w", err)
	}

	var items []domain.Alert
	err := filtered.Session(&gorm.Session{}).
		Select("a.*, k.nama AS komoditas_nama, w.nama AS kabupaten_nama").
		Joins("LEFT JOIN komoditas k ON k.kode = a.komoditas_kode").
		Joins("JOIN kabupaten w ON w.kode = a.kabupaten_kode").
		Order("a.dibuat_pada DESC, a.id").
		Limit(q.Limit).Offset((q.Page - 1) * q.Limit).
		Scan(&items).Error
	if err != nil {
		return nil, 0, fmt.Errorf("daftar alert ews: %w", err)
	}
	return items, total, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Alert, error) {
	var item domain.Alert
	res := r.alertQuery(ctx).Where("a.id = ?", id).Limit(1).Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("ambil alert ews: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (r *Repository) ListAturan(ctx context.Context) ([]domain.Aturan, error) {
	var items []domain.Aturan
	err := r.db.WithContext(ctx).Table("aturan_ews AS t").
		Select("t.*, k.nama AS komoditas_nama").
		Joins("LEFT JOIN komoditas k ON k.kode = t.komoditas_kode").
		Order("t.jenis, t.komoditas_kode, t.id").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("daftar aturan ews: %w", err)
	}
	return items, nil
}

func (r *Repository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction, userID string) ([]string, error) {
	query, ok := bulkSQL[action]
	if !ok {
		return nil, fmt.Errorf("aksi massal tidak dikenal: %s", action)
	}
	var changed []string
	err := r.db.WithContext(ctx).Raw(query, map[string]any{"ids": ids, "user": userID}).Scan(&changed).Error
	if err != nil {
		return nil, fmt.Errorf("aksi massal %s: %w", action, err)
	}
	return changed, nil
}
