package postgres

import (
	"context"
	"fmt"

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

func (r *Repository) GetKpiOperasional(ctx context.Context, periode string) ([]domain.KpiOperasional, error) {
	var items []domain.KpiOperasional

	query := `
		SELECT
			k.id,
			k.indikator,
			k.nilai,
			k.target,
			CASE WHEN k.target > 0 THEN (k.nilai / k.target * 100) ELSE 0 END AS capaian_pct,
			k.status,
			k.periode,
			k.kabupaten_kode,
			COALESCE(kab.nama, '') AS kabupaten_nama
		FROM kpi_operasional k
		LEFT JOIN kabupaten kab ON kab.kode = k.kabupaten_kode
		WHERE ($1 = '' OR k.periode = $1)
		ORDER BY k.indikator
	`

	if err := r.db.WithContext(ctx).Raw(query, periode).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get kpi operasional: %w", err)
	}

	return items, nil
}

// ── Heatmap ───────────────────────────────────────────────────────────

func (r *Repository) GetHeatmapData(ctx context.Context, periode string) (*domain.HeatmapData, error) {
	var kabupaten []domain.Kabupaten
	if err := r.db.WithContext(ctx).Raw(`
		SELECT kode, nama FROM kabupaten ORDER BY nama
	`).Scan(&kabupaten).Error; err != nil {
		return nil, fmt.Errorf("get kabupaten: %w", err)
	}

	var komoditas []domain.Komoditas
	if err := r.db.WithContext(ctx).Raw(`
		SELECT kode, nama, COALESCE(pilot, false) AS pilot FROM komoditas ORDER BY nama
	`).Scan(&komoditas).Error; err != nil {
		return nil, fmt.Errorf("get komoditas: %w", err)
	}

	var sel []domain.SelHeatmap
	if err := r.db.WithContext(ctx).Raw(`
		SELECT
			kabupaten_kode,
			komoditas_kode,
			nilai,
			kategori
		FROM heatmap_sel
		WHERE ($1 = '' OR periode = $1)
		ORDER BY kabupaten_kode, komoditas_kode
	`, periode).Scan(&sel).Error; err != nil {
		return nil, fmt.Errorf("get heatmap sel: %w", err)
	}

	return &domain.HeatmapData{
		Kabupaten: kabupaten,
		Komoditas: komoditas,
		Sel:       sel,
	}, nil
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

