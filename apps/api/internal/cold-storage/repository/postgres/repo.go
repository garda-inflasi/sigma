package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/garda-inflasi/api/internal/cold-storage/domain"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// sortExpr memetakan kolom urut yang diizinkan ke ekspresi SQL. Whitelist ini
// satu-satunya jalur input pengguna menuju klausa ORDER BY.
var sortExpr = map[domain.StokSort]string{
	// Urut menurut batas kedaluwarsa setara urut menurut sisa masa simpan.
	domain.SortSisaMasaSimpan: "s.kedaluwarsa_pada",
	domain.SortKomoditas:      "LOWER(k.nama)",
	domain.SortFasilitas:      "LOWER(c.nama)",
	domain.SortVolume:         "s.volume_ton",
	domain.SortMasuk:          "s.masuk_pada",
	domain.SortRisiko:         "s.risiko_food_loss",
	domain.SortPemilik:        "LOWER(s.pemilik)",
}

func (r *Repository) ListFasilitas(ctx context.Context) ([]domain.FasilitasView, error) {
	var fasilitas []domain.FasilitasView
	err := r.db.WithContext(ctx).Raw(`
		SELECT c.id, c.nama, c.kabupaten_kode, kb.nama AS kabupaten_nama, c.lat, c.lng,
		       c.kapasitas_ton, COALESCE(SUM(s.volume_ton), 0) AS terisi_ton,
		       c.suhu_target_c, c.suhu_aktual_c, c.status
		  FROM cold_storage c
		  JOIN kabupaten kb ON kb.kode = c.kabupaten_kode
		  LEFT JOIN stok_cold_storage s ON s.cold_storage_id = c.id
		 GROUP BY c.id, kb.nama
		 ORDER BY CASE c.status WHEN 'OPERASIONAL' THEN 0 WHEN 'PERAWATAN' THEN 1 ELSE 2 END, c.id
	`).Scan(&fasilitas).Error
	if err != nil {
		return nil, err
	}
	return fasilitas, nil
}

func (r *Repository) ListStok(ctx context.Context, q domain.StokQuery) (domain.StokPage, error) {
	// Setiap query memerlukan builder baru: *gorm.DB dari Where/Joins tidak
	// aman dipakai ulang untuk dua statement berbeda.
	filtered := func() *gorm.DB {
		tx := r.db.WithContext(ctx).Table("stok_cold_storage AS s").
			Joins("JOIN cold_storage c ON c.id = s.cold_storage_id").
			Joins("JOIN komoditas k ON k.kode = s.komoditas_kode")
		if q.ColdStorageID != "" {
			tx = tx.Where("s.cold_storage_id = ?", q.ColdStorageID)
		}
		if q.Search != "" {
			like := "%" + escapeLike(q.Search) + "%"
			tx = tx.Where(`(k.nama ILIKE ? ESCAPE '\' OR s.pemilik ILIKE ? ESCAPE '\')`, like, like)
		}
		return tx
	}

	var agg struct {
		Total       int64
		TotalVolume float64
	}
	if err := filtered().
		Select("COUNT(*) AS total, COALESCE(SUM(s.volume_ton), 0) AS total_volume").
		Scan(&agg).Error; err != nil {
		return domain.StokPage{}, err
	}

	items := []domain.StokView{}
	if agg.Total > 0 {
		if err := filtered().
			Select(`s.id, s.cold_storage_id, c.nama AS cold_storage_nama, s.komoditas_kode,
				k.nama AS komoditas_nama, s.volume_ton, s.masuk_pada, s.kedaluwarsa_pada,
				s.risiko_food_loss, s.pemilik`).
			Order(orderClause(q)).
			Limit(q.Limit).
			Offset(q.Offset()).
			Scan(&items).Error; err != nil {
			return domain.StokPage{}, err
		}
	}

	return domain.StokPage{Items: items, Total: agg.Total, TotalVolumeTon: agg.TotalVolume}, nil
}

func (r *Repository) RingkasanStok(ctx context.Context, ambang float64) (domain.RingkasanStok, error) {
	var out domain.RingkasanStok
	err := r.db.WithContext(ctx).Table("stok_cold_storage").
		Select(`COUNT(*) AS total_lot,
			COALESCE(SUM(volume_ton), 0) AS total_volume_ton,
			COUNT(*) FILTER (WHERE risiko_food_loss > ?) AS risiko_tinggi_lot,
			COALESCE(SUM(volume_ton) FILTER (WHERE risiko_food_loss > ?), 0) AS risiko_tinggi_ton`,
			ambang, ambang).
		Scan(&out).Error
	return out, err
}

func (r *Repository) BulkAction(ctx context.Context, ids []string, action domain.BulkAction) (domain.BulkResult, error) {
	// Cold storage bulk actions are advisory workflow operations (distribution
	// planning / processing recommendations). They do not mutate stored state;
	// report how many of the requested lots exist and their combined volume.
	switch action {
	case domain.BulkRencanakanDistribusi, domain.BulkRekomendasiOlahan:
		var res struct {
			Berhasil  int64
			VolumeTon float64
		}
		if err := r.db.WithContext(ctx).Model(&domain.StokColdStorage{}).
			Select("COUNT(*) AS berhasil, COALESCE(SUM(volume_ton), 0) AS volume_ton").
			Where("id IN ?", ids).
			Scan(&res).Error; err != nil {
			return domain.BulkResult{}, fmt.Errorf("bulk action %s: %w", action, err)
		}
		return domain.BulkResult{Berhasil: res.Berhasil, VolumeTon: res.VolumeTon}, nil
	default:
		return domain.BulkResult{}, fmt.Errorf("unknown bulk action: %s", action)
	}
}

func (r *Repository) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// orderClause membangun ORDER BY dari kriteria yang sudah divalidasi. `s.id`
// menjadi pemutus seri agar urutan stabil antar-halaman.
func orderClause(q domain.StokQuery) string {
	expr, ok := sortExpr[q.Sort]
	if !ok {
		expr = sortExpr[domain.SortSisaMasaSimpan]
	}
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}
	return fmt.Sprintf("%s %s, s.id ASC", expr, dir)
}

// escapeLike menetralkan karakter wildcard LIKE agar pencarian bersifat literal.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
