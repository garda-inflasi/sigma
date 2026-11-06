package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/garda-inflasi/api/internal/audit/domain"
	"github.com/garda-inflasi/api/pkg/idx"
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
	// Aktor yang bukan UUID (mis. pengenal sistem) tidak mungkin ada di pengguna.id (uuid); dicatat tanpa aktor.
	if !idx.ValidUUID(id) {
		return nil, domain.ErrActorNotFound
	}
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

func (r *Repository) Get(ctx context.Context, id string) (*domain.AuditLog, error) {
	var row domain.AuditLog
	res := r.db.WithContext(ctx).Where("id = ?", id).Limit(1).Find(&row)
	if res.Error != nil {
		return nil, fmt.Errorf("ambil audit log: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrNotFound
	}
	return &row, nil
}

func (r *Repository) List(ctx context.Context, filter domain.Filter, after *domain.Cursor, limit int) ([]domain.AuditLog, error) {
	query := applyFilter(r.db.WithContext(ctx).Model(&domain.AuditLog{}), filter)
	if after != nil {
		query = query.Where("(waktu, id) < (?, ?)", after.Waktu, after.ID)
	}

	var items []domain.AuditLog
	if err := query.Order("waktu DESC, id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("daftar audit log: %w", err)
	}
	return items, nil
}

// applyFilter menambahkan syarat filter ke query; seluruh nilai terparameter.
func applyFilter(query *gorm.DB, filter domain.Filter) *gorm.DB {
	if filter.Dari != nil {
		query = query.Where("waktu >= ?", *filter.Dari)
	}
	if filter.Sampai != nil {
		query = query.Where("waktu < ?", *filter.Sampai)
	}
	if filter.Aktor != "" {
		pattern := likePattern(filter.Aktor)
		if id, ok := idx.NormalUUID(filter.Aktor); ok {
			query = query.Where("aktor_id = ? OR LOWER(aktor_nama) LIKE ? OR LOWER(aktor_email) LIKE ?", id, pattern, pattern)
		} else {
			// aktor_id berkolom uuid: teks bebas (nama, email) tidak boleh dibandingkan dengannya (22P02).
			query = query.Where("LOWER(aktor_nama) LIKE ? OR LOWER(aktor_email) LIKE ?", pattern, pattern)
		}
	}
	if filter.Aksi != "" {
		query = query.Where("aksi = ?", filter.Aksi)
	}
	if filter.Entitas != "" {
		query = query.Where("entitas = ?", filter.Entitas)
	}
	if filter.EntitasID != "" {
		query = query.Where("entitas_id = ?", filter.EntitasID)
	}
	if filter.Peran != "" {
		query = query.Where("aktor_peran = ?", filter.Peran)
	}
	if filter.CorrelationID != "" {
		query = query.Where("correlation_id = ?", filter.CorrelationID)
	}
	if filter.Hasil != "" {
		query = query.Where("hasil = ?", filter.Hasil)
	}
	if filter.Q != "" {
		pattern := likePattern(filter.Q)
		query = query.Where(
			"LOWER(aksi) LIKE ? OR LOWER(entitas) LIKE ? OR LOWER(entitas_id) LIKE ? OR LOWER(aktor_nama) LIKE ? OR LOWER(aktor_email) LIKE ? OR LOWER(correlation_id) LIKE ?",
			pattern, pattern, pattern, pattern, pattern, pattern)
	}
	return query
}

func likePattern(s string) string {
	return "%" + escapeLike(strings.ToLower(s)) + "%"
}

// escapeLike menetralkan karakter wildcard LIKE agar pencarian bersifat literal.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
