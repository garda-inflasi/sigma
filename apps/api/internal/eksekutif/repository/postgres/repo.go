package postgres

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/garda-inflasi/api/internal/eksekutif/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InflasiTerbaru(ctx context.Context, wilayah string) (*domain.InflasiBulanan, error) {
	var rows []domain.InflasiBulanan
	err := r.db.WithContext(ctx).Where("wilayah_kode = ?", wilayah).Order("periode DESC").Limit(1).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("inflasi terbaru: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (r *Repository) SeriInflasi(ctx context.Context, wilayah string, sampai time.Time, n int) ([]domain.InflasiBulanan, error) {
	var rows []domain.InflasiBulanan
	err := r.db.WithContext(ctx).Where("wilayah_kode = ? AND periode <= ?", wilayah, sampai).
		Order("periode DESC").Limit(n).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("seri inflasi: %w", err)
	}
	slices.Reverse(rows)
	return rows, nil
}

func (r *Repository) TargetBerlaku(ctx context.Context, tahun int) (*domain.TargetInflasi, error) {
	var rows []domain.TargetInflasi
	err := r.db.WithContext(ctx).Where("tahun <= ?", tahun).Order("tahun DESC").Limit(1).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("target inflasi: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (r *Repository) Penyumbang(ctx context.Context, wilayah string, periode time.Time, n int) ([]domain.Penyumbang, error) {
	var rows []domain.Penyumbang
	err := r.db.WithContext(ctx).Raw(`
		SELECT a.komoditas_kode AS kode, k.nama, a.andil::float8 AS andil
		  FROM andil_inflasi a
		  JOIN komoditas k ON k.kode = a.komoditas_kode
		 WHERE a.wilayah_kode = ? AND a.periode = ?
		 ORDER BY ABS(a.andil) DESC, a.komoditas_kode
		 LIMIT ?`, wilayah, periode, n).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("penyumbang inflasi: %w", err)
	}
	return rows, nil
}

func (r *Repository) NeracaTerbaru(ctx context.Context, wilayah string) ([]domain.NeracaItem, error) {
	var rows []domain.NeracaItem
	err := r.db.WithContext(ctx).Raw(`
		SELECT n.komoditas_kode, k.nama, n.periode,
		       n.produksi_ton::float8 AS produksi_ton, n.konsumsi_ton::float8 AS konsumsi_ton,
		       n.stok_ton::float8 AS stok_ton, n.sumber
		  FROM neraca_pangan n
		  JOIN komoditas k ON k.kode = n.komoditas_kode
		 WHERE n.wilayah_kode = ?
		   AND n.periode = (SELECT MAX(periode) FROM neraca_pangan WHERE wilayah_kode = ?)
		 ORDER BY n.komoditas_kode`, wilayah, wilayah).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("neraca pangan: %w", err)
	}
	return rows, nil
}

func (r *Repository) RuangFiskalTerbaru(ctx context.Context) ([]domain.RuangFiskal, error) {
	var rows []domain.RuangFiskal
	err := r.db.WithContext(ctx).Raw(`
		SELECT * FROM ruang_fiskal
		 WHERE (tahun, per_tanggal) = (SELECT tahun, per_tanggal FROM ruang_fiskal ORDER BY tahun DESC, per_tanggal DESC LIMIT 1)
		 ORDER BY pagu DESC, instrumen`).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("ruang fiskal: %w", err)
	}
	return rows, nil
}

func (r *Repository) HitungEWS(ctx context.Context) (map[string]int, error) {
	var rows []struct {
		Severity string `gorm:"column:severity"`
		Jumlah   int    `gorm:"column:jumlah"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT severity::text AS severity, COUNT(*)::int AS jumlah
		  FROM ews_alert WHERE status <> 'SELESAI' GROUP BY severity`).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("hitung ews: %w", err)
	}
	hasil := make(map[string]int, len(rows))
	for _, row := range rows {
		hasil[row.Severity] = row.Jumlah
	}
	return hasil, nil
}

func (r *Repository) HitungIntervensi(ctx context.Context) ([]domain.HitungIntervensi, error) {
	var rows []domain.HitungIntervensi
	err := r.db.WithContext(ctx).Raw(`
		SELECT lower(siklus::text) AS siklus,
		       (COUNT(*) FILTER (WHERE tahap NOT LIKE '%-selesai'))::int AS aktif,
		       (COUNT(*) FILTER (WHERE tahap LIKE '%-selesai'))::int AS selesai
		  FROM intervensi GROUP BY siklus`).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("hitung intervensi: %w", err)
	}
	return rows, nil
}
