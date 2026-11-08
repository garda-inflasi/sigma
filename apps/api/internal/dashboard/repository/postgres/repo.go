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

// cteEksternal menghitung sel heatmap dari harga eksternal (gold_harga_harian) hanya untuk
// komoditas dan kabupaten/kota yang belum punya sel lapangan: harga terakhir (3 hari terakhir)
// terhadap rata-rata hari-hari sebelumnya dalam 7 hari, dengan rumus yang sama dengan mv_heatmap_harga.
const cteEksternal = `
	ek AS (
	    SELECT komoditas_kode, kabupaten_kode, tanggal, harga_eksternal AS harga
	      FROM gold_harga_harian
	     WHERE harga_eksternal IS NOT NULL
	       AND tanggal >= (now() AT TIME ZONE 'Asia/Jakarta')::date - 11
	),
	terakhir AS (
	    SELECT DISTINCT ON (komoditas_kode, kabupaten_kode) komoditas_kode, kabupaten_kode, tanggal, harga
	      FROM ek
	     WHERE tanggal >= (now() AT TIME ZONE 'Asia/Jakarta')::date - 3
	     ORDER BY komoditas_kode, kabupaten_kode, tanggal DESC
	),
	basis AS (
	    SELECT t.komoditas_kode, t.kabupaten_kode, AVG(k.harga) AS rata_rata
	      FROM terakhir t
	      JOIN ek k ON k.komoditas_kode = t.komoditas_kode AND k.kabupaten_kode = t.kabupaten_kode
	               AND k.tanggal >= t.tanggal - 7 AND k.tanggal < t.tanggal
	     GROUP BY t.komoditas_kode, t.kabupaten_kode
	),
	eksternal AS (
	    SELECT t.komoditas_kode, t.kabupaten_kode,
	           ROUND((t.harga - b.rata_rata) / b.rata_rata * 100, 1)::float8 AS delta,
	           t.harga::float8 AS harga, 'EKSTERNAL' AS sumber
	      FROM terakhir t
	      JOIN basis b USING (komoditas_kode, kabupaten_kode)
	     WHERE NOT EXISTS (SELECT 1 FROM mv_heatmap_harga m
	                        WHERE m.komoditas_kode = t.komoditas_kode AND m.kabupaten_kode = t.kabupaten_kode)
	)`

// Heatmap membaca sel dari mv_heatmap_harga (lapangan), dilengkapi sel dari harga eksternal untuk
// yang belum punya data lapangan, beserta kabupaten dan komoditas yang punya sel.
func (r *Repository) Heatmap(ctx context.Context) (*domain.HeatmapData, error) {
	tx := r.db.WithContext(ctx)
	data := &domain.HeatmapData{}

	if err := tx.Raw(`WITH` + cteEksternal + `
		SELECT komoditas_kode, kabupaten_kode, delta_pct::float8 AS delta, harga::float8 AS harga, 'LAPANGAN' AS sumber
		  FROM mv_heatmap_harga
		UNION ALL
		SELECT komoditas_kode, kabupaten_kode, delta, harga, sumber FROM eksternal
		ORDER BY kabupaten_kode, komoditas_kode
	`).Scan(&data.Sel).Error; err != nil {
		return nil, fmt.Errorf("get sel heatmap: %w", err)
	}

	kabupaten, komoditas := map[string]bool{}, map[string]bool{}
	for _, s := range data.Sel {
		kabupaten[s.KabupatenKode], komoditas[s.KomoditasKode] = true, true
	}
	if len(data.Sel) == 0 {
		return data, nil
	}
	if err := tx.Raw(`SELECT kode, nama FROM kabupaten WHERE kode IN ? ORDER BY kode`, keys(kabupaten)).Scan(&data.Kabupaten).Error; err != nil {
		return nil, fmt.Errorf("get kabupaten heatmap: %w", err)
	}
	if err := tx.Raw(`SELECT kode, nama, pilot FROM komoditas WHERE kode IN ? ORDER BY pilot DESC, nama`, keys(komoditas)).Scan(&data.Komoditas).Error; err != nil {
		return nil, fmt.Errorf("get komoditas heatmap: %w", err)
	}
	return data, nil
}

func keys(m map[string]bool) []string {
	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	return res
}

// ── Validasi silang harga dan cuaca (hasil ingestion) ─────────────────

// PembandingHarga mengambil pasangan harga lapangan vs eksternal terbaru per komoditas dan
// kabupaten/kota sejak tanggal itu.
func (r *Repository) PembandingHarga(ctx context.Context, sejak time.Time) ([]domain.PembandingHarga, error) {
	var items []domain.PembandingHarga
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (g.komoditas_kode, g.kabupaten_kode)
		       g.komoditas_kode, k.nama AS komoditas_nama, g.kabupaten_kode, kb.nama AS kabupaten_nama, g.tanggal,
		       g.harga_lapangan::float8 AS harga_lapangan, g.harga_eksternal::float8 AS harga_eksternal, g.selisih_pct::float8 AS selisih_pct,
		       EXISTS (SELECT 1 FROM harga_eksternal he
		                 JOIN eksekusi_sumber e ON e.id = he.eksekusi_id AND e.pemicu = 'DEMO'
		                WHERE he.komoditas_kode = g.komoditas_kode AND he.kabupaten_kode = g.kabupaten_kode AND he.tanggal = g.tanggal) AS demo
		  FROM gold_harga_harian g
		  JOIN komoditas k ON k.kode = g.komoditas_kode
		  JOIN kabupaten kb ON kb.kode = g.kabupaten_kode
		 WHERE g.selisih_pct IS NOT NULL AND g.tanggal >= ?
		 ORDER BY g.komoditas_kode, g.kabupaten_kode, g.tanggal DESC`, sejak).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("get pembanding harga: %w", err)
	}
	return items, nil
}

// CuacaTerbaru mengambil cuaca harian terbaru tiap kabupaten/kota pada rentang tanggal.
func (r *Repository) CuacaTerbaru(ctx context.Context, sejak, sampai time.Time) ([]domain.CuacaKabupaten, error) {
	var items []domain.CuacaKabupaten
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (c.kabupaten_kode)
		       c.kabupaten_kode, kb.nama AS kabupaten_nama, c.tanggal, c.curah_hujan_mm, c.suhu_c, c.kelembapan, c.anomali, c.sumber,
		       EXISTS (SELECT 1 FROM eksekusi_sumber e WHERE e.id = c.eksekusi_id AND e.pemicu = 'DEMO') AS demo
		  FROM cuaca_harian c
		  JOIN kabupaten kb ON kb.kode = c.kabupaten_kode
		 WHERE c.tanggal BETWEEN ? AND ?
		 ORDER BY c.kabupaten_kode, c.tanggal DESC`, sejak, sampai).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("get cuaca terbaru: %w", err)
	}
	return items, nil
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
			  FROM entri_harga e JOIN pasar p ON (p.kode = e.pasar_id OR p.id::text = e.pasar_id)
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
