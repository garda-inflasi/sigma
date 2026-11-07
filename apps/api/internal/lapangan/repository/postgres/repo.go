package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/lapangan/domain"
	"github.com/garda-inflasi/api/pkg/idx"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

const viewColumns = `t.id, t.kode, t.judul, t.pasar_id, p.nama AS pasar_nama, t.petugas_id, t.komoditas,
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

// GetByID mengembalikan satu tugas milik petugas, atau milik siapa saja bila semua true. ref berisi ID BIGINT atau
// kode lama (TGS-...).
func (r *Repository) GetByID(ctx context.Context, ref idx.RujukanID, petugasID string, semua bool) (*domain.TugasView, error) {
	kondisi, nilai := ref.Kondisi("t.id", "t.kode")
	q := r.tugasView(ctx).Where(kondisi, nilai)
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

// Rincian membaca nama komoditas dan apakah sudah ada entri harga yang tertaut ke tugas untuk tiap kode.
func (r *Repository) Rincian(ctx context.Context, tugasID idx.ID, kode []string) ([]domain.RincianKomoditas, error) {
	if len(kode) == 0 {
		return nil, nil
	}
	var items []domain.RincianKomoditas
	err := r.db.WithContext(ctx).Raw(`
		SELECT k.kode, k.nama,
		       EXISTS (SELECT 1 FROM tugas_entri te JOIN entri_harga e ON e.id = te.entri_id
		                WHERE te.tugas_id = @tugas AND e.komoditas_kode = k.kode) AS terisi
		  FROM komoditas k
		 WHERE k.kode IN @kode`,
		map[string]any{"tugas": tugasID, "kode": kode},
	).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("rincian komoditas tugas: %w", err)
	}
	return items, nil
}

// pisahRujukan memisahkan rujukan menjadi ID BIGINT dan kode lama untuk klausa WHERE id IN … OR kode IN ….
func pisahRujukan(refs []idx.RujukanID) (ids []idx.ID, kode []string) {
	for _, ref := range refs {
		if ref.ID != 0 {
			ids = append(ids, ref.ID)
		} else if ref.Kode != "" {
			kode = append(kode, ref.Kode)
		}
	}
	return ids, kode
}

// Aksesibel menyaring refs ke baris tugas yang ada dan boleh disentuh pemanggil.
func (r *Repository) Aksesibel(ctx context.Context, refs []idx.RujukanID, petugasID string, semua bool) ([]domain.TugasRef, error) {
	ids, kode := pisahRujukan(refs)
	q := r.db.WithContext(ctx).Table("tugas_lapangan").Select("id, kode").Where("(id IN ? OR kode IN ?)", ids, kode)
	if !semua {
		q = q.Where("petugas_id = ?", petugasID)
	}
	var found []domain.TugasRef
	if err := q.Scan(&found).Error; err != nil {
		return nil, fmt.Errorf("cek akses tugas lapangan: %w", err)
	}
	return found, nil
}

// TandaiSelesai menutup tugas yang belum selesai dan targetnya sudah terpenuhi.
func (r *Repository) TandaiSelesai(ctx context.Context, refs []idx.RujukanID, petugasID string, semua bool, waktu time.Time) ([]domain.TugasRef, error) {
	ids, kode := pisahRujukan(refs)
	var changed []domain.TugasRef
	err := r.db.WithContext(ctx).Raw(`
		UPDATE tugas_lapangan
		   SET status = 'SELESAI', selesai_pada = @waktu
		 WHERE (id IN @ids OR kode IN @kode)
		   AND status <> 'SELESAI'
		   AND jumlah_selesai >= target
		   AND (@semua OR petugas_id = @petugas)
		RETURNING id, kode`,
		map[string]any{"ids": ids, "kode": kode, "waktu": waktu, "semua": semua, "petugas": petugasID},
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
