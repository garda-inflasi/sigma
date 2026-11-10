package postgres

import (
	"context"
	"fmt"
	"strings"
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

// tanggalSQL memformat tanggal (DATE waktu Aceh) sebagai argumen yang di-cast ke date oleh SQL (?::date), supaya
// tidak bergantung pada zona waktu sesi maupun zona waktu nilai time.Time.
func tanggalSQL(t time.Time) string { return t.Format(domain.TanggalLayout) }

// kondisiKabupaten menerjemahkan Cakupan dan filter kabupaten permintaan menjadi kondisi SQL atas kolom kabupaten.
// Cakupan nol (tertutup) dan filter di luar cakupan menjadi FALSE: gagal-tutup, tidak ada baris yang lolos. Irisannya
// dihitung Cakupan.Irisi, yang juga dipakai usecase, jadi aturannya satu.
func kondisiKabupaten(kolom string, c domain.Cakupan, filter string) (string, []any) {
	kabupaten, ada := c.Irisi(filter)
	switch {
	case !ada:
		return "FALSE", nil
	case kabupaten == "":
		return "TRUE", nil
	}
	return kolom + " = ?", []any{kabupaten}
}

// kondisiKomoditas menyaring satu komoditas; kode kosong berarti semua.
func kondisiKomoditas(kolom, kode string) (string, []any) {
	if kode == "" {
		return "TRUE", nil
	}
	return kolom + " = ?", []any{kode}
}

// bangun menyusun SQL dan argumennya berurutan, supaya urutan penanda ? selalu sama dengan urutan argumen.
type bangun struct {
	sql  strings.Builder
	args []any
}

func (b *bangun) tulis(sql string, args ...any) *bangun {
	b.sql.WriteString(sql)
	b.args = append(b.args, args...)
	return b
}

// kondisi menulis kondisi cakupan+filter kabupaten dan filter komoditas, digabung AND.
func (b *bangun) kondisi(kolomKabupaten, kolomKomoditas string, c domain.Cakupan, kabupaten, komoditas string) *bangun {
	kab, a := kondisiKabupaten(kolomKabupaten, c, kabupaten)
	kom, k := kondisiKomoditas(kolomKomoditas, komoditas)
	return b.tulis(kab, a...).tulis(" AND ").tulis(kom, k...)
}

// ── Cakupan pengguna ──────────────────────────────────────────────────

// PenggunaCakupan membaca peran dan kabupaten tugas pengguna: satu pencarian kunci primer.
func (r *Repository) PenggunaCakupan(ctx context.Context, penggunaID string) (domain.PenggunaCakupan, error) {
	var rows []struct {
		Peran         string `gorm:"column:peran"`
		KabupatenKode string `gorm:"column:kabupaten_kode"`
	}
	err := r.db.WithContext(ctx).Raw(`SELECT peran::text AS peran, COALESCE(kabupaten_kode, '') AS kabupaten_kode
		FROM pengguna WHERE id = ?`, penggunaID).Scan(&rows).Error
	if err != nil {
		return domain.PenggunaCakupan{}, fmt.Errorf("baca peran pengguna: %w", err)
	}
	if len(rows) == 0 {
		return domain.PenggunaCakupan{}, domain.ErrPenggunaTidakAda
	}
	return domain.PenggunaCakupan{Peran: rows[0].Peran, KabupatenKode: rows[0].KabupatenKode}, nil
}

// ── KPI Operasional ───────────────────────────────────────────────────

// DeretKpi menjumlahkan fakta harian mv_kpi_operasional atas kabupaten dan komoditas yang lolos cakupan dan filter.
//
// Penyatuan per indikator dan hari:
//   - entri, validasi: jumlah pembilang dan penyebut.
//   - waspada: jumlah komoditas BERBEDA yang punya baris (komoditas yang melampaui ambang di dua kabupaten tetap satu).
//   - cs: pembilang dari baris volume (komoditas_kode terisi), penyebut dari baris kapasitas (komoditas_kode kosong).
//     Baris kapasitas lolos filter komoditas, sebab dengan filter komoditas pembilangnya hanya volume komoditas itu
//     sedangkan penyebutnya tetap kapasitas seluruh fasilitas di kabupaten yang dicakup.
func (r *Repository) DeretKpi(ctx context.Context, q domain.DeretKpiQuery) ([]domain.TitikKpi, error) {
	b := &bangun{}
	b.tulis(`
		SELECT indikator, tanggal,
		       CASE indikator
		            WHEN 'waspada' THEN COUNT(DISTINCT komoditas_kode) FILTER (WHERE pembilang > 0)::float8
		            WHEN 'cs' THEN COALESCE(SUM(pembilang) FILTER (WHERE komoditas_kode <> ''), 0)::float8
		            ELSE COALESCE(SUM(pembilang), 0)::float8
		       END AS pembilang,
		       CASE indikator
		            WHEN 'cs' THEN COALESCE(SUM(penyebut) FILTER (WHERE komoditas_kode = ''), 0)::float8
		            ELSE COALESCE(SUM(penyebut), 0)::float8
		       END AS penyebut
		  FROM mv_kpi_operasional
		 WHERE indikator IN ('entri', 'validasi', 'waspada', 'cs')
		   AND tanggal BETWEEN ?::date AND ?::date
		   AND `, tanggalSQL(q.Awal), tanggalSQL(q.Akhir))
	kab, a := kondisiKabupaten("kabupaten_kode", q.Cakupan, q.KabupatenKode)
	b.tulis(kab, a...)
	b.tulis(" AND (komoditas_kode = '' OR ")
	kom, k := kondisiKomoditas("komoditas_kode", q.KomoditasKode)
	b.tulis(kom, k...)
	b.tulis(`)
		 GROUP BY indikator, tanggal
		 ORDER BY indikator, tanggal`)

	var titik []domain.TitikKpi
	if err := r.db.WithContext(ctx).Raw(b.sql.String(), b.args...).Scan(&titik).Error; err != nil {
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
		 WHERE periode_mulai <= ?::date
		 ORDER BY indikator, periode_mulai DESC
	`, tanggalSQL(tanggal)).Scan(&targets).Error; err != nil {
		return nil, fmt.Errorf("get target kpi: %w", err)
	}
	return targets, nil
}

// ── Heatmap ───────────────────────────────────────────────────────────

// Jendela perhitungan sel heatmap (rumus sama dengan mv_heatmap_harga, migrasi 000122): harga terakhir dalam
// jendelaTerakhirHari sampai tanggal acuan, terhadap rata-rata tertimbang jendelaBasisHari sebelum tanggal harga itu.
const (
	jendelaTerakhirHari = 3
	jendelaBasisHari    = 7
	// jendelaBacaHari adalah seberapa jauh ke belakang agregat dibaca dari tanggal acuan: harga terakhir (3 hari)
	// ditambah basisnya (7 hari), dengan sisa satu hari.
	jendelaBacaHari = 11
)

// tulisLapanganMV menulis CTE sel lapangan dari mv_heatmap_harga (jalur bawaan: relatif terhadap hari ini).
func tulisLapanganMV(b *bangun, q domain.HeatmapQuery) {
	b.tulis(`
	lapangan AS (
	    SELECT komoditas_kode, kabupaten_kode, delta_pct::float8 AS delta, harga::float8 AS harga
	      FROM mv_heatmap_harga
	     WHERE `)
	b.kondisi("kabupaten_kode", "komoditas_kode", q.Cakupan, q.KabupatenKode, q.KomoditasKode)
	b.tulis(`
	)`)
}

// tulisLapanganLangsung menulis CTE sel lapangan yang dihitung dari agregat_harga_harian untuk tanggal acuan, dengan
// rumus yang sama dengan mv_heatmap_harga. Berguna untuk tanggal acuan selain hari ini, karena view itu hanya
// menyimpan keadaan terkini.
func tulisLapanganLangsung(b *bangun, q domain.HeatmapQuery, batasTerakhir time.Time) {
	b.tulis(`
	konsumen AS (
	    SELECT komoditas_kode, kabupaten_kode, tanggal, rata_rata, jumlah
	      FROM agregat_harga_harian
	     WHERE tipe = 'KONSUMEN' AND tanggal BETWEEN ?::date - `+fmt.Sprint(jendelaBacaHari)+` AND ?::date
	       AND `, tanggalSQL(q.Acuan), tanggalSQL(q.Acuan))
	b.kondisi("kabupaten_kode", "komoditas_kode", q.Cakupan, q.KabupatenKode, q.KomoditasKode)
	b.tulis(`
	),
	terakhir_lapangan AS (
	    SELECT DISTINCT ON (komoditas_kode, kabupaten_kode) komoditas_kode, kabupaten_kode, tanggal, rata_rata
	      FROM konsumen
	     WHERE tanggal >= ?::date
	     ORDER BY komoditas_kode, kabupaten_kode, tanggal DESC
	),
	basis_lapangan AS (
	    SELECT t.komoditas_kode, t.kabupaten_kode, SUM(k.rata_rata * k.jumlah) / SUM(k.jumlah) AS rata_rata_7h
	      FROM terakhir_lapangan t
	      JOIN konsumen k ON k.komoditas_kode = t.komoditas_kode AND k.kabupaten_kode = t.kabupaten_kode
	               AND k.tanggal >= t.tanggal - `+fmt.Sprint(jendelaBasisHari)+` AND k.tanggal < t.tanggal
	     GROUP BY t.komoditas_kode, t.kabupaten_kode
	),
	lapangan AS (
	    SELECT t.komoditas_kode, t.kabupaten_kode,
	           ROUND((t.rata_rata - b.rata_rata_7h) / b.rata_rata_7h * 100, 1)::float8 AS delta,
	           t.rata_rata::float8 AS harga
	      FROM terakhir_lapangan t
	      JOIN basis_lapangan b USING (komoditas_kode, kabupaten_kode)
	)`, tanggalSQL(batasTerakhir))
}

// tulisEksternal menulis CTE sel dari harga eksternal (gold_harga_harian) hanya untuk komoditas dan kabupaten/kota
// yang belum punya sel lapangan: harga terakhir (3 hari sampai tanggal acuan) terhadap rata-rata hari-hari sebelumnya
// dalam 7 hari, dengan rumus yang sama dengan sel lapangan. Mengikuti tanggal acuan, cakupan, dan filter yang sama.
func tulisEksternal(b *bangun, q domain.HeatmapQuery, batasTerakhir time.Time) {
	b.tulis(`,
	ek AS (
	    SELECT komoditas_kode, kabupaten_kode, tanggal, harga_eksternal AS harga
	      FROM gold_harga_harian
	     WHERE harga_eksternal IS NOT NULL AND tanggal BETWEEN ?::date - `+fmt.Sprint(jendelaBacaHari)+` AND ?::date
	       AND `, tanggalSQL(q.Acuan), tanggalSQL(q.Acuan))
	b.kondisi("kabupaten_kode", "komoditas_kode", q.Cakupan, q.KabupatenKode, q.KomoditasKode)
	b.tulis(`
	),
	terakhir AS (
	    SELECT DISTINCT ON (komoditas_kode, kabupaten_kode) komoditas_kode, kabupaten_kode, tanggal, harga
	      FROM ek
	     WHERE tanggal >= ?::date
	     ORDER BY komoditas_kode, kabupaten_kode, tanggal DESC
	),
	basis AS (
	    SELECT t.komoditas_kode, t.kabupaten_kode, AVG(k.harga) AS rata_rata
	      FROM terakhir t
	      JOIN ek k ON k.komoditas_kode = t.komoditas_kode AND k.kabupaten_kode = t.kabupaten_kode
	               AND k.tanggal >= t.tanggal - `+fmt.Sprint(jendelaBasisHari)+` AND k.tanggal < t.tanggal
	     GROUP BY t.komoditas_kode, t.kabupaten_kode
	),
	eksternal AS (
	    SELECT t.komoditas_kode, t.kabupaten_kode,
	           ROUND((t.harga - b.rata_rata) / b.rata_rata * 100, 1)::float8 AS delta,
	           t.harga::float8 AS harga, 'EKSTERNAL' AS sumber
	      FROM terakhir t
	      JOIN basis b USING (komoditas_kode, kabupaten_kode)
	     WHERE NOT EXISTS (SELECT 1 FROM lapangan l
	                        WHERE l.komoditas_kode = t.komoditas_kode AND l.kabupaten_kode = t.kabupaten_kode)
	)`, tanggalSQL(batasTerakhir))
}

// sqlHeatmap menyusun query sel heatmap: sel lapangan (dari mv_heatmap_harga atau dihitung langsung untuk tanggal
// acuan) dilengkapi sel eksternal untuk yang belum punya data lapangan. Cakupan dan filter berlaku pada keduanya.
func sqlHeatmap(q domain.HeatmapQuery) (string, []any) {
	// Harga terakhir dicari sejak 3 hari sebelum tanggal acuan, atau sejak Dari bila lebih belakangan.
	batasTerakhir := q.Acuan.AddDate(0, 0, -jendelaTerakhirHari)
	if q.Langsung && q.Dari.After(batasTerakhir) {
		batasTerakhir = q.Dari
	}

	b := &bangun{}
	b.tulis("WITH")
	if q.Langsung {
		tulisLapanganLangsung(b, q, batasTerakhir)
	} else {
		tulisLapanganMV(b, q)
	}
	tulisEksternal(b, q, batasTerakhir)
	b.tulis(`
	SELECT komoditas_kode, kabupaten_kode, delta, harga, 'LAPANGAN' AS sumber FROM lapangan
	UNION ALL
	SELECT komoditas_kode, kabupaten_kode, delta, harga, sumber FROM eksternal
	ORDER BY kabupaten_kode, komoditas_kode`)
	return b.sql.String(), b.args
}

// Heatmap membaca sel lapangan (mv_heatmap_harga, atau dihitung langsung untuk tanggal acuan), dilengkapi sel dari
// harga eksternal untuk yang belum punya data lapangan, beserta kabupaten dan komoditas yang punya sel.
func (r *Repository) Heatmap(ctx context.Context, q domain.HeatmapQuery) (*domain.HeatmapData, error) {
	tx := r.db.WithContext(ctx)
	data := &domain.HeatmapData{}

	sql, args := sqlHeatmap(q)
	if err := tx.Raw(sql, args...).Scan(&data.Sel).Error; err != nil {
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
// kabupaten/kota sejak tanggal itu, hanya di kabupaten dalam cakupan.
func (r *Repository) PembandingHarga(ctx context.Context, sejak time.Time, cakupan domain.Cakupan) ([]domain.PembandingHarga, error) {
	b := &bangun{}
	b.tulis(`
		SELECT DISTINCT ON (g.komoditas_kode, g.kabupaten_kode)
		       g.komoditas_kode, k.nama AS komoditas_nama, g.kabupaten_kode, kb.nama AS kabupaten_nama, g.tanggal,
		       g.harga_lapangan::float8 AS harga_lapangan, g.harga_eksternal::float8 AS harga_eksternal, g.selisih_pct::float8 AS selisih_pct,
		       EXISTS (SELECT 1 FROM harga_eksternal he
		                 JOIN eksekusi_sumber e ON e.id = he.eksekusi_id AND e.pemicu = 'DEMO'
		                WHERE he.komoditas_kode = g.komoditas_kode AND he.kabupaten_kode = g.kabupaten_kode AND he.tanggal = g.tanggal) AS demo
		  FROM gold_harga_harian g
		  JOIN komoditas k ON k.kode = g.komoditas_kode
		  JOIN kabupaten kb ON kb.kode = g.kabupaten_kode
		 WHERE g.selisih_pct IS NOT NULL AND g.tanggal >= ? AND `, sejak)
	kab, a := kondisiKabupaten("g.kabupaten_kode", cakupan, "")
	b.tulis(kab, a...)
	b.tulis(`
		 ORDER BY g.komoditas_kode, g.kabupaten_kode, g.tanggal DESC`)

	var items []domain.PembandingHarga
	if err := r.db.WithContext(ctx).Raw(b.sql.String(), b.args...).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get pembanding harga: %w", err)
	}
	return items, nil
}

// CuacaTerbaru mengambil cuaca harian terbaru tiap kabupaten/kota pada rentang tanggal, hanya di kabupaten dalam cakupan.
func (r *Repository) CuacaTerbaru(ctx context.Context, sejak, sampai time.Time, cakupan domain.Cakupan) ([]domain.CuacaKabupaten, error) {
	b := &bangun{}
	b.tulis(`
		SELECT DISTINCT ON (c.kabupaten_kode)
		       c.kabupaten_kode, kb.nama AS kabupaten_nama, c.tanggal, c.curah_hujan_mm, c.suhu_c, c.kelembapan, c.anomali, c.sumber,
		       EXISTS (SELECT 1 FROM eksekusi_sumber e WHERE e.id = c.eksekusi_id AND e.pemicu = 'DEMO') AS demo
		  FROM cuaca_harian c
		  JOIN kabupaten kb ON kb.kode = c.kabupaten_kode
		 WHERE c.tanggal BETWEEN ? AND ? AND `, sejak, sampai)
	kab, a := kondisiKabupaten("c.kabupaten_kode", cakupan, "")
	b.tulis(kab, a...)
	b.tulis(`
		 ORDER BY c.kabupaten_kode, c.tanggal DESC`)

	var items []domain.CuacaKabupaten
	if err := r.db.WithContext(ctx).Raw(b.sql.String(), b.args...).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get cuaca terbaru: %w", err)
	}
	return items, nil
}

// ── Aktivitas lapangan ────────────────────────────────────────────────

// AktivitasLapangan menghitung entri harga konsumen (selain DRAF) per kabupaten
// terhadap target harian: jumlah petugas aktif kali jumlah komoditas aktif. Hanya kabupaten dalam cakupan.
func (r *Repository) AktivitasLapangan(ctx context.Context, awal, akhir time.Time, cakupan domain.Cakupan) ([]domain.AktivitasKabupaten, error) {
	b := &bangun{}
	b.tulis(`
		WITH petugas AS (
			SELECT kabupaten_kode, COUNT(*) AS n
			  FROM pengguna
			 WHERE peran = 'OPS_MAHASISWA' AND status = 'AKTIF' AND kabupaten_kode IS NOT NULL AND `)
	kabPetugas, a := kondisiKabupaten("kabupaten_kode", cakupan, "")
	b.tulis(kabPetugas, a...)
	b.tulis(`
			 GROUP BY kabupaten_kode
		),
		entri AS (
			SELECT p.kabupaten_kode, COUNT(*) AS n
			  FROM entri_harga e JOIN pasar p ON p.kode = e.pasar_id
			 WHERE e.tipe = 'KONSUMEN' AND e.status <> 'DRAF' AND e.dicatat_pada >= ? AND e.dicatat_pada < ? AND `, awal, akhir)
	kabEntri, a := kondisiKabupaten("p.kabupaten_kode", cakupan, "")
	b.tulis(kabEntri, a...)
	b.tulis(`
			 GROUP BY p.kabupaten_kode
		),
		komoditas AS (SELECT COUNT(*) AS n FROM komoditas WHERE status = 'AKTIF')
		SELECT k.kode AS kabupaten_kode, k.nama AS kabupaten_nama, pt.n AS jumlah_petugas,
		       COALESCE(en.n, 0) AS entri, pt.n * km.n AS target
		  FROM petugas pt
		  JOIN kabupaten k ON k.kode = pt.kabupaten_kode
		  CROSS JOIN komoditas km
		  LEFT JOIN entri en ON en.kabupaten_kode = pt.kabupaten_kode
		 ORDER BY k.nama`)

	var items []domain.AktivitasKabupaten
	if err := r.db.WithContext(ctx).Raw(b.sql.String(), b.args...).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("get aktivitas lapangan: %w", err)
	}
	return items, nil
}
