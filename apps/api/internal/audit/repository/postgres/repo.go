package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, log *domain.AuditLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("simpan audit log: %w", err)
	}
	return nil
}

func (r *Repository) FindActor(ctx context.Context, id string) (*domain.Actor, error) {
	var actor domain.Actor
	res := r.db.WithContext(ctx).Raw("SELECT id, nama, email, peran::text AS peran FROM pengguna WHERE id = ?", id).Scan(&actor)
	if res.Error != nil {
		return nil, fmt.Errorf("cari aktor: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrActorNotFound
	}
	return &actor, nil
}

func (r *Repository) List(ctx context.Context, filter domain.Filter, after *domain.Cursor, limit int) ([]domain.AuditLog, error) {
	query := r.db.WithContext(ctx).Model(&domain.AuditLog{})

	if filter.Dari != nil {
		query = query.Where("waktu >= ?", *filter.Dari)
	}
	if filter.Sampai != nil {
		query = query.Where("waktu < ?", *filter.Sampai)
	}
	if filter.Aktor != "" {
		pattern := "%" + escapeLike(strings.ToLower(filter.Aktor)) + "%"
		query = query.Where("aktor_id = ? OR LOWER(aktor_nama) LIKE ? OR LOWER(aktor_email) LIKE ?", filter.Aktor, pattern, pattern)
	}
	if filter.Aksi != "" {
		query = query.Where("aksi = ?", filter.Aksi)
	}
	if filter.Entitas != "" {
		query = query.Where("entitas = ?", filter.Entitas)
	}
	if filter.Hasil != "" {
		query = query.Where("hasil = ?", filter.Hasil)
	}
	if after != nil {
		query = query.Where("(waktu, id) < (?, ?)", after.Waktu, after.ID)
	}

	var items []domain.AuditLog
	if err := query.Order("waktu DESC, id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("daftar audit log: %w", err)
	}
	return items, nil
}

// escapeLike menetralkan karakter wildcard LIKE agar pencarian bersifat literal.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
