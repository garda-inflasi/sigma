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

// Acuan membaca komoditas dan pasar aktif serta median harga resmi (DIVALIDASI atau
// DIPUBLIKASI) per komoditas, kabupaten, dan tipe sejak waktu itu, hanya bila
// sampelnya minimal minSampel.
func (r *Repository) Acuan(ctx context.Context, sejak time.Time, minSampel int) (domain.DataAcuan, error) {
	var data domain.DataAcuan
	tx := r.db.WithContext(ctx)

	if err := tx.Raw(`SELECT kode, nama, satuan, harga_acuan FROM komoditas WHERE status = 'AKTIF' ORDER BY kode`).Scan(&data.Komoditas).Error; err != nil {
		return data, fmt.Errorf("baca komoditas acuan: %w", err)
	}
	if err := tx.Raw(`SELECT id, nama, kabupaten_kode, lat, lng, radius_meter FROM pasar WHERE aktif ORDER BY id`).Scan(&data.Pasar).Error; err != nil {
		return data, fmt.Errorf("baca pasar acuan: %w", err)
	}
	err := tx.Raw(`
		SELECT e.komoditas_kode, p.kabupaten_kode, e.tipe::text AS tipe,
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY e.harga) AS median, COUNT(*) AS sampel
		  FROM entri_harga e
		  JOIN pasar p ON p.id = e.pasar_id
		 WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI') AND e.dicatat_pada >= ?
		 GROUP BY e.komoditas_kode, p.kabupaten_kode, e.tipe
		HAVING COUNT(*) >= ?
		 ORDER BY e.komoditas_kode, p.kabupaten_kode, e.tipe`, sejak, minSampel).Scan(&data.Median).Error
	if err != nil {
		return data, fmt.Errorf("baca median acuan: %w", err)
	}
	return data, nil
}
