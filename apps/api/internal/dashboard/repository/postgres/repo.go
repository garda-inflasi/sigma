package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/garda-inflasi/api/internal/dashboard/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ── KPI Operasional ───────────────────────────────────────────────────

// DeretKpi membaca deret harian indikator dari mv_kpi_operasional.
func (r *Repository) DeretKpi(ctx context.Context) ([]domain.TitikKpi, error) {
	var titik []domain.TitikKpi
	if err := r.db.WithContext(ctx).Raw(`
		SELECT indikator, tanggal, nilai FROM mv_kpi_operasional ORDER BY indikator, tanggal
	`).Scan(&titik).Error; err != nil {
		return nil, fmt.Errorf("get deret kpi: %w", err)
	}
	return titik, nil
}

// TargetKpi mengambil target terbaru yang sudah berlaku pada tanggal itu untuk tiap indikator.
func (r *Repository) TargetKpi(ctx context.Context, tanggal time.Time) ([]domain.TargetKpi, error) {
	var targets []domain.TargetKpi
	if err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (indikator) indikator, target, arah
		  FROM target_kpi
		 WHERE periode_mulai <= ?
		 ORDER BY indikator, periode_mulai DESC
	`, tanggal).Scan(&targets).Error; err != nil {
		return nil, fmt.Errorf("get target kpi: %w", err)
	}
	return targets, nil
}

// ── Heatmap ───────────────────────────────────────────────────────────

// Heatmap membaca sel dari mv_heatmap_harga beserta kabupaten dan komoditas yang punya data.
func (r *Repository) Heatmap(ctx context.Context) (*domain.HeatmapData, error) {
	tx := r.db.WithContext(ctx)
	data := &domain.HeatmapData{}

	if err := tx.Raw(`
		SELECT kode, nama FROM kabupaten
		 WHERE kode IN (SELECT kabupaten_kode FROM mv_heatmap_harga) ORDER BY kode
	`).Scan(&data.Kabupaten).Error; err != nil {
		return nil, fmt.Errorf("get kabupaten heatmap: %w", err)
	}
	if err := tx.Raw(`
		SELECT kode, nama, pilot FROM komoditas
		 WHERE kode IN (SELECT komoditas_kode FROM mv_heatmap_harga) ORDER BY pilot DESC, nama
	`).Scan(&data.Komoditas).Error; err != nil {
		return nil, fmt.Errorf("get komoditas heatmap: %w", err)
	}
	if err := tx.Raw(`
		SELECT komoditas_kode, kabupaten_kode, delta_pct AS delta, harga
		  FROM mv_heatmap_harga ORDER BY kabupaten_kode, komoditas_kode
	`).Scan(&data.Sel).Error; err != nil {
		return nil, fmt.Errorf("get sel heatmap: %w", err)
	}
	return data, nil
}

// ── Aktivitas lapangan ────────────────────────────────────────────────

// AktivitasLapangan menghitung entri harga konsumen (selain DRAF) per kabupaten
// terhadap target harian: jumlah petugas aktif kali jumlah komoditas aktif.
func (r *Repository) AktivitasLapangan(ctx context.Context, awal, akhir time.Time) ([]domain.AktivitasKabupaten, error) {
	var items []domain.AktivitasKabupaten
	err := r.db.WithContext(ctx).Raw(`
		WITH petugas AS (
			SELECT kabupaten_kode, COUNT(*) AS n
			  FROM pengguna
			 WHERE peran = 'OPS_MAHASISWA' AND status = 'AKTIF' AND kabupaten_kode IS NOT NULL
			 GROUP BY kabupaten_kode
		),
		entri AS (
			SELECT p.kabupaten_kode, COUNT(*) AS n
			  FROM entri_harga e JOIN pasar p ON p.id = e.pasar_id
			 WHERE e.tipe = 'KONSUMEN' AND e.status <> 'DRAF' AND e.dicatat_pada >= ? AND e.dicatat_pada < ?
			 GROUP BY p.kabupaten_kode
		),
		komoditas AS (SELECT COUNT(*) AS n FROM komoditas WHERE status = 'AKTIF')
		SELECT k.kode AS kabupaten_kode, k.nama AS kabupaten_nama, pt.n AS jumlah_petugas,
		       COALESCE(en.n, 0) AS entri, pt.n * km.n AS target
		  FROM petugas pt
		  JOIN kabupaten k ON k.kode = pt.kabupaten_kode
		  CROSS JOIN komoditas km
		  LEFT JOIN entri en ON en.kabupaten_kode = pt.kabupaten_kode
		 ORDER BY k.nama`, awal, akhir).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("get aktivitas lapangan: %w", err)
	}
	return items, nil
}

// ── Ringkasan Eksekutif ───────────────────────────────────────────────

func (r *Repository) GetRingkasanEksekutif(ctx context.Context, periode string) (*domain.RingkasanEksekutif, error) {
	var result domain.RingkasanEksekutif

	query := `
		SELECT
			inflasi_tahunan,
			inflasi_bulanan,
			ihk,
			ntp,
			periode
		FROM ringkasan_eksekutif
		WHERE ($1 = '' OR periode = $1)
		ORDER BY periode DESC
		LIMIT 1
	`

	if err := r.db.WithContext(ctx).Raw(query, periode).Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("get ringkasan eksekutif: %w", err)
	}

	var komoditas []domain.KomoditasSumbangan
	if err := r.db.WithContext(ctx).Raw(`
		SELECT kode, nama, andil, inflasi_pct
		FROM komoditas_sumbangan
		WHERE ($1 = '' OR periode = $1)
		ORDER BY andil DESC
		LIMIT 10
	`, periode).Scan(&komoditas).Error; err != nil {
		return nil, fmt.Errorf("get komoditas sumbangan: %w", err)
	}
	result.KomoditasPenyumbang = komoditas

	var tertinggi []domain.DaerahInflasi
	if err := r.db.WithContext(ctx).Raw(`
		SELECT kabupaten_kode, kabupaten_nama, inflasi_pct
		FROM daerah_inflasi
		WHERE ($1 = '' OR periode = $1)
		ORDER BY inflasi_pct DESC
		LIMIT 5
	`, periode).Scan(&tertinggi).Error; err != nil {
		return nil, fmt.Errorf("get daerah tertinggi: %w", err)
	}
	result.DaerahTertinggi = tertinggi

	var terendah []domain.DaerahInflasi
	if err := r.db.WithContext(ctx).Raw(`
		SELECT kabupaten_kode, kabupaten_nama, inflasi_pct
		FROM daerah_inflasi
		WHERE ($1 = '' OR periode = $1)
		ORDER BY inflasi_pct ASC
		LIMIT 5
	`, periode).Scan(&terendah).Error; err != nil {
		return nil, fmt.Errorf("get daerah terendah: %w", err)
	}
	result.DaerahTerendah = terendah

	var tren []domain.TrenBulanan
	if err := r.db.WithContext(ctx).Raw(`
		SELECT bulan, inflasi_pct, ihk
		FROM tren_inflasi
		ORDER BY bulan ASC
		LIMIT 12
	`).Scan(&tren).Error; err != nil {
		return nil, fmt.Errorf("get tren: %w", err)
	}
	result.Tren = tren

	return &result, nil
}
