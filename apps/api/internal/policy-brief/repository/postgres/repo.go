package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/policy-brief/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// view memilih kolom BriefView dari policy_brief digabung nama penyusun.
func (r *Repository) view(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("policy_brief AS pb").
		Select(`pb.id, pb.periode, pb.judul, pb.status, pb.ringkasan, pb.halaman,
			u.nama AS disusun_oleh, jsonb_array_length(pb.rekomendasi) AS jumlah_rekomendasi,
			pb.dibuat_pada, pb.diperbarui`).
		Joins("JOIN pengguna u ON u.id = pb.disusun_oleh_id")
}

func (r *Repository) List(ctx context.Context) ([]domain.BriefView, error) {
	var items []domain.BriefView
	if err := r.view(ctx).Order("pb.id DESC").Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("list policy brief: %w", err)
	}
	return items, nil
}

func (r *Repository) Get(ctx context.Context, id string) (*domain.BriefView, error) {
	var item domain.BriefView
	res := r.view(ctx).Where("pb.id = ?", id).Limit(1).Scan(&item)
	if res.Error != nil {
		return nil, fmt.Errorf("get policy brief: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (r *Repository) Create(ctx context.Context, pb *domain.PolicyBrief) (bool, error) {
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(pb)
	if res.Error != nil {
		return false, fmt.Errorf("create policy brief: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *Repository) Transisi(ctx context.Context, id, dari, ke string) (bool, error) {
	res := r.db.WithContext(ctx).Model(&domain.PolicyBrief{}).
		Where("id = ? AND status = ?", id, dari).
		Updates(map[string]any{"status": ke, "diperbarui": gorm.Expr("now()")})
	if res.Error != nil {
		return false, fmt.Errorf("ubah status policy brief: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}
