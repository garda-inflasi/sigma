-- Slice 08-A (docs/todos/08-dashboard-operasional.md): dashboard operasional
-- membaca agregat atas entri_harga (materialized view), bukan tabel statis.
-- Hari dihitung menurut waktu Aceh (Asia/Jakarta). Hanya entri berstatus
-- DIVALIDASI atau DIPUBLIKASI yang masuk agregat harga.

-- +goose Up
-- +goose StatementBegin

-- ── Siklus ekonomi wilayah ─────────────────────────────────────────────
-- Satu siklus aktif (akhir_pada NULL) per wilayah pada satu waktu.
CREATE TABLE IF NOT EXISTS siklus_ekonomi (
    id VARCHAR(255) PRIMARY KEY,
    wilayah_kode VARCHAR(2) NOT NULL REFERENCES provinsi(kode),
    siklus siklus NOT NULL,
    mulai_pada TIMESTAMPTZ NOT NULL,
    akhir_pada TIMESTAMPTZ,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_siklus_ekonomi_rentang CHECK (akhir_pada IS NULL OR akhir_pada > mulai_pada)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_siklus_ekonomi_aktif ON siklus_ekonomi(wilayah_kode) WHERE akhir_pada IS NULL;
CREATE INDEX IF NOT EXISTS idx_siklus_ekonomi_wilayah ON siklus_ekonomi(wilayah_kode, mulai_pada DESC);

-- ── Target indikator operasional ───────────────────────────────────────
-- arah NAIK: makin tinggi makin baik; TURUN: makin rendah makin baik.
CREATE TABLE IF NOT EXISTS target_kpi (
    indikator VARCHAR(50) NOT NULL,
    periode_mulai DATE NOT NULL,
    target DOUBLE PRECISION NOT NULL,
    arah VARCHAR(5) NOT NULL,
    PRIMARY KEY (indikator, periode_mulai),
    CONSTRAINT chk_target_kpi_arah CHECK (arah IN ('NAIK', 'TURUN')),
    CONSTRAINT chk_target_kpi_target CHECK (target > 0)
);

-- ── Agregat harga harian ───────────────────────────────────────────────
CREATE MATERIALIZED VIEW IF NOT EXISTS agregat_harga_harian AS
SELECT
    e.komoditas_kode,
    p.kabupaten_kode,
    e.tipe,
    (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date AS tanggal,
    AVG(e.harga)::numeric(18, 2) AS rata_rata,
    MIN(e.harga) AS harga_min,
    MAX(e.harga) AS harga_maks,
    COUNT(*)::int AS jumlah
FROM entri_harga e
JOIN pasar p ON p.id = e.pasar_id
WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
GROUP BY e.komoditas_kode, p.kabupaten_kode, e.tipe, (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date
WITH DATA;

CREATE UNIQUE INDEX IF NOT EXISTS uq_agregat_harga_harian
    ON agregat_harga_harian(komoditas_kode, kabupaten_kode, tipe, tanggal);
CREATE INDEX IF NOT EXISTS idx_agregat_harga_harian_tanggal ON agregat_harga_harian(tanggal);

-- ── Peta panas harga konsumen ──────────────────────────────────────────
-- Harga terakhir (dalam tiga hari terakhir) terhadap rata-rata tertimbang
-- tujuh hari sebelum tanggal itu; delta_pct dalam persen.
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_heatmap_harga AS
WITH konsumen AS (
    SELECT komoditas_kode, kabupaten_kode, tanggal, rata_rata, jumlah
      FROM agregat_harga_harian
     WHERE tipe = 'KONSUMEN'
),
terakhir AS (
    SELECT DISTINCT ON (komoditas_kode, kabupaten_kode) komoditas_kode, kabupaten_kode, tanggal, rata_rata
      FROM konsumen
     WHERE tanggal >= (now() AT TIME ZONE 'Asia/Jakarta')::date - 3
     ORDER BY komoditas_kode, kabupaten_kode, tanggal DESC
),
basis AS (
    SELECT t.komoditas_kode, t.kabupaten_kode, SUM(k.rata_rata * k.jumlah) / SUM(k.jumlah) AS rata_rata_7h
      FROM terakhir t
      JOIN konsumen k
        ON k.komoditas_kode = t.komoditas_kode AND k.kabupaten_kode = t.kabupaten_kode
       AND k.tanggal >= t.tanggal - 7 AND k.tanggal < t.tanggal
     GROUP BY t.komoditas_kode, t.kabupaten_kode
)
SELECT t.komoditas_kode, t.kabupaten_kode,
       t.rata_rata AS harga,
       b.rata_rata_7h::numeric(18, 2) AS rata_rata_7h,
       ROUND((t.rata_rata - b.rata_rata_7h) / b.rata_rata_7h * 100, 1) AS delta_pct
  FROM terakhir t
  JOIN basis b USING (komoditas_kode, kabupaten_kode)
WITH DATA;

CREATE UNIQUE INDEX IF NOT EXISTS uq_mv_heatmap_harga ON mv_heatmap_harga(komoditas_kode, kabupaten_kode);

-- ── Deret KPI operasional (14 hari terakhir) ───────────────────────────
--   entri    : entri harga (selain DRAF) per hari
--   validasi : persen entri DIVALIDASI/DIPUBLIKASI dalam jendela 7 hari yang berakhir pada hari itu
--   waspada  : komoditas yang harga konsumennya di kabupaten mana pun naik lebih dari 7% terhadap rata-rata 7 hari sebelumnya
--   cs       : persen kapasitas fasilitas cold storage OPERASIONAL yang terisi stok pada hari itu
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_kpi_operasional AS
WITH hari AS (
    SELECT d::date AS tanggal
      FROM generate_series((now() AT TIME ZONE 'Asia/Jakarta')::date - 13, (now() AT TIME ZONE 'Asia/Jakarta')::date, interval '1 day') AS d
),
entri AS (
    SELECT (dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date AS tanggal, status
      FROM entri_harga
     WHERE status <> 'DRAF' AND dicatat_pada >= now() - interval '25 days'
),
harian AS (
    SELECT komoditas_kode, kabupaten_kode, tanggal, rata_rata,
           (SELECT SUM(b.rata_rata * b.jumlah) / SUM(b.jumlah)
              FROM agregat_harga_harian b
             WHERE b.tipe = 'KONSUMEN' AND b.komoditas_kode = a.komoditas_kode AND b.kabupaten_kode = a.kabupaten_kode
               AND b.tanggal BETWEEN a.tanggal - 7 AND a.tanggal - 1) AS basis
      FROM agregat_harga_harian a
     WHERE a.tipe = 'KONSUMEN' AND a.tanggal >= (now() AT TIME ZONE 'Asia/Jakarta')::date - 13
),
kapasitas AS (
    SELECT COALESCE(SUM(kapasitas_ton), 0) AS ton FROM cold_storage WHERE status = 'OPERASIONAL'
)
SELECT 'entri'::text AS indikator, h.tanggal,
       (SELECT COUNT(*) FROM entri e WHERE e.tanggal = h.tanggal)::double precision AS nilai
  FROM hari h
UNION ALL
SELECT 'validasi', h.tanggal,
       COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')) / NULLIF(COUNT(*), 0)
                   FROM entri e WHERE e.tanggal BETWEEN h.tanggal - 6 AND h.tanggal), 0)::double precision
  FROM hari h
UNION ALL
SELECT 'waspada', h.tanggal,
       (SELECT COUNT(DISTINCT x.komoditas_kode) FROM harian x
         WHERE x.tanggal = h.tanggal AND x.basis IS NOT NULL AND x.rata_rata > x.basis * 1.07)::double precision
  FROM hari h
UNION ALL
SELECT 'cs', h.tanggal,
       CASE WHEN k.ton > 0 THEN
           100.0 * COALESCE((SELECT SUM(s.volume_ton)
                               FROM stok_cold_storage s
                               JOIN cold_storage c ON c.id = s.cold_storage_id AND c.status = 'OPERASIONAL'
                              WHERE (s.masuk_pada AT TIME ZONE 'Asia/Jakarta')::date <= h.tanggal
                                AND (s.kedaluwarsa_pada AT TIME ZONE 'Asia/Jakarta')::date > h.tanggal), 0) / k.ton
       ELSE 0 END::double precision
  FROM hari h CROSS JOIN kapasitas k
WITH DATA;

CREATE UNIQUE INDEX IF NOT EXISTS uq_mv_kpi_operasional ON mv_kpi_operasional(indikator, tanggal);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP MATERIALIZED VIEW IF EXISTS mv_kpi_operasional;
DROP MATERIALIZED VIEW IF EXISTS mv_heatmap_harga;
DROP MATERIALIZED VIEW IF EXISTS agregat_harga_harian;
DROP TABLE IF EXISTS target_kpi;
DROP TABLE IF EXISTS siklus_ekonomi;
-- +goose StatementEnd
