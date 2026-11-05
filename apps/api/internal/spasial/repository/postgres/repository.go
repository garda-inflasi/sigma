// Package postgres adalah repository spasial di atas PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/garda-inflasi/api/internal/spasial/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ListMarker: untuk tiap wilayah diambil komoditas berdeviasi harga terbesar
// (mutlak) pada periode neraca terbaru.
func (r *Repository) ListMarker(ctx context.Context) ([]domain.Marker, error) {
	var items []domain.Marker
	err := r.db.WithContext(ctx).Raw(`
		SELECT w.kode, w.nama, w.lat, w.lng, w.level,
		       COALESCE(n.delta_harga_pct, 0)::float8 AS delta,
		       COALESCE(k.nama, '') AS komoditas,
		       EXISTS (SELECT 1 FROM pasar p WHERE p.kabupaten_kode = w.kode AND p.aktif) AS punya_pasar
		  FROM wilayah w
		  LEFT JOIN LATERAL (
		        SELECT komoditas_kode, delta_harga_pct
		          FROM neraca_wilayah
		         WHERE kabupaten_kode = w.kode
		           AND periode = (SELECT MAX(periode) FROM neraca_wilayah)
		         ORDER BY ABS(delta_harga_pct) DESC, komoditas_kode
		         LIMIT 1) n ON true
		  LEFT JOIN komoditas k ON k.kode = n.komoditas_kode
		 ORDER BY w.kode`).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("daftar marker wilayah: %w", err)
	}
	return items, nil
}

func (r *Repository) ListNeraca(ctx context.Context, q domain.NeracaQuery) ([]domain.Neraca, error) {
	query := r.db.WithContext(ctx).Table("neraca_wilayah AS n").
		Select("n.*, k.nama AS komoditas_nama, w.nama AS kabupaten_nama").
		Joins("JOIN komoditas k ON k.kode = n.komoditas_kode").
		Joins("JOIN kabupaten w ON w.kode = n.kabupaten_kode")
	if q.Periode.IsZero() {
		query = query.Where("n.periode = (SELECT MAX(periode) FROM neraca_wilayah)")
	} else {
		query = query.Where("n.periode = ?", q.Periode)
	}
	if q.KomoditasKode != "" {
		query = query.Where("n.komoditas_kode = ?", q.KomoditasKode)
	}

	var items []domain.Neraca
	if err := query.Order("n.kabupaten_kode, n.komoditas_kode").Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("daftar neraca wilayah: %w", err)
	}
	return items, nil
}

func (r *Repository) ListRute(ctx context.Context) ([]domain.Rute, error) {
	var items []domain.Rute
	err := r.db.WithContext(ctx).Table("rute_distribusi AS r").
		Select("r.*, a.nama AS asal_nama, t.nama AS tujuan_nama, k.nama AS komoditas_nama").
		Joins("JOIN kabupaten a ON a.kode = r.asal_kode").
		Joins("JOIN kabupaten t ON t.kode = r.tujuan_kode").
		Joins("JOIN komoditas k ON k.kode = r.komoditas_kode").
		Order("r.biaya DESC, r.id").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("daftar rute distribusi: %w", err)
	}
	return items, nil
}

func (r *Repository) ListKandidat(ctx context.Context) ([]domain.Kandidat, error) {
	var items []domain.Kandidat
	err := r.db.WithContext(ctx).Table("kandidat_cold_storage AS c").
		Select("c.*, w.nama AS kabupaten_nama").
		Joins("JOIN kabupaten w ON w.kode = c.kabupaten_kode").
		Order("c.skor DESC, c.id").
		Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("daftar kandidat cold storage: %w", err)
	}
	return items, nil
}

func (r *Repository) ListGeometri(ctx context.Context, bbox *domain.BBox) ([]domain.GeometriWilayah, error) {
	query := r.db.WithContext(ctx).Table("wilayah_geometri AS g").
		Select("g.*, w.nama, w.level").
		Joins("JOIN wilayah w ON w.kode = g.kabupaten_kode")
	if bbox != nil {
		query = query.Where("g.min_lng <= ? AND g.max_lng >= ? AND g.min_lat <= ? AND g.max_lat >= ?",
			bbox.MaxLng, bbox.MinLng, bbox.MaxLat, bbox.MinLat)
	}
	var items []domain.GeometriWilayah
	if err := query.Order("g.kabupaten_kode").Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("daftar geometri wilayah: %w", err)
	}
	return items, nil
}
