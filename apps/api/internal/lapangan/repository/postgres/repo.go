package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

const viewColumns = `t.id, t.judul, t.pasar_id, p.nama AS pasar_nama, t.petugas_id, t.komoditas,
	t.tipe, t.tenggat, t.status, t.target, t.jumlah_selesai`

func (r *Repository) tugasView(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("tugas_lapangan AS t").
		Joins("JOIN pasar p ON p.id = t.pasar_id").
		Select(viewColumns)
}

// ListSaya mengembalikan tugas milik petugas; yang sudah SELESAI di urutan akhir.
func (r *Repository) ListSaya(ctx context.Context, petugasID string) ([]domain.TugasView, error) {
	var items []domain.TugasView
	err := r.tugasView(ctx).
		Where("t.petugas_id = ?", petugasID).
		Order("(t.status = 'SELESAI'), t.tenggat, t.id").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list tugas lapangan: %w", err)
	}
	return items, nil
}

// GetByID mengembalikan satu tugas milik petugas, atau milik siapa saja bila semua true.
func (r *Repository) GetByID(ctx context.Context, id, petugasID string, semua bool) (*domain.TugasView, error) {
	q := r.tugasView(ctx).Where("t.id = ?", id)
	if !semua {
		q = q.Where("t.petugas_id = ?", petugasID)
	}
	var items []domain.TugasView
	if err := q.Limit(1).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get tugas lapangan: %w", err)
	}
	if len(items) == 0 {
		return nil, domain.ErrNotFound
	}
	return &items[0], nil
}

// Aksesibel menyaring ids ke tugas yang ada dan boleh disentuh pemanggil.
func (r *Repository) Aksesibel(ctx context.Context, ids []string, petugasID string, semua bool) ([]string, error) {
	q := r.db.WithContext(ctx).Table("tugas_lapangan").Select("id").Where("id IN ?", ids)
	if !semua {
		q = q.Where("petugas_id = ?", petugasID)
	}
	var found []string
	if err := q.Scan(&found).Error; err != nil {
		return nil, fmt.Errorf("cek akses tugas lapangan: %w", err)
	}
	return found, nil
}

// TandaiSelesai menutup tugas yang belum selesai dan targetnya sudah terpenuhi.
func (r *Repository) TandaiSelesai(ctx context.Context, ids []string, petugasID string, semua bool, waktu time.Time) ([]string, error) {
	var changed []string
	err := r.db.WithContext(ctx).Raw(`
		UPDATE tugas_lapangan
		   SET status = 'SELESAI', selesai_pada = @waktu
		 WHERE id IN @ids
		   AND status <> 'SELESAI'
		   AND jumlah_selesai >= target
		   AND (@semua OR petugas_id = @petugas)
		RETURNING id`,
		map[string]any{"ids": ids, "waktu": waktu, "semua": semua, "petugas": petugasID},
	).Scan(&changed).Error
	if err != nil {
		return nil, fmt.Errorf("tandai tugas selesai: %w", err)
	}
	return changed, nil
}
