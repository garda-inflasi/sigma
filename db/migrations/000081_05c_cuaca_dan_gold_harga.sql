-- Slice 05-C (docs/todos/05-sumber-data.md): silver cuaca dan lapisan gold harga.
--
-- * cuaca_harian (sudah ada, belum berpemakai) mendapat lineage ke run dan payload bronze
--   agar tiap baris silver bisa ditelusuri; UNIQUE (kabupaten_kode, tanggal, sumber) yang
--   sudah ada menjamin upsert idempoten.
-- * gold_harga_harian: harga konsumen harian per komoditas dan kabupaten/kota dari dua
--   sisi, lapangan (agregat_harga_harian: entri tervalidasi) dan eksternal (harga_eksternal:
--   PIHPS dst.), beserta selisih persen untuk validasi silang. Disegarkan oleh
--   dashboard/refresher bersama view agregat lain (REFRESH ... CONCURRENTLY).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE cuaca_harian ADD COLUMN IF NOT EXISTS eksekusi_id BIGINT REFERENCES eksekusi_sumber(id);
ALTER TABLE cuaca_harian ADD COLUMN IF NOT EXISTS data_mentah_id BIGINT REFERENCES data_mentah(id);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_cuaca_harian_nilai') THEN
        ALTER TABLE cuaca_harian
            ADD CONSTRAINT chk_cuaca_harian_nilai CHECK (curah_hujan_mm >= 0 AND kelembapan BETWEEN 0 AND 100);
    END IF;
END $$;

CREATE MATERIALIZED VIEW IF NOT EXISTS gold_harga_harian AS
WITH lapangan AS (
    SELECT komoditas_kode, kabupaten_kode, tanggal,
           (SUM(rata_rata * jumlah) / SUM(jumlah))::numeric(18, 2) AS harga,
           SUM(jumlah)::int AS jumlah
      FROM agregat_harga_harian
     WHERE tipe = 'KONSUMEN'
     GROUP BY komoditas_kode, kabupaten_kode, tanggal
),
eksternal AS (
    SELECT komoditas_kode, kabupaten_kode, tanggal,
           AVG(harga)::numeric(18, 2) AS harga,
           COUNT(*)::int AS jumlah
      FROM harga_eksternal
     WHERE tipe = 'KONSUMEN'
     GROUP BY komoditas_kode, kabupaten_kode, tanggal
)
SELECT komoditas_kode, kabupaten_kode, tanggal,
       l.harga AS harga_lapangan, l.jumlah AS n_lapangan,
       e.harga AS harga_eksternal, e.jumlah AS n_eksternal,
       CASE WHEN l.harga IS NOT NULL AND e.harga IS NOT NULL
            THEN ROUND((l.harga - e.harga) / e.harga * 100, 1) END AS selisih_pct
  FROM lapangan l
  FULL JOIN eksternal e USING (komoditas_kode, kabupaten_kode, tanggal)
WITH DATA;

CREATE UNIQUE INDEX IF NOT EXISTS uq_gold_harga_harian ON gold_harga_harian(komoditas_kode, kabupaten_kode, tanggal);
CREATE INDEX IF NOT EXISTS idx_gold_harga_harian_tanggal ON gold_harga_harian(tanggal);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP MATERIALIZED VIEW IF EXISTS gold_harga_harian;
ALTER TABLE cuaca_harian DROP CONSTRAINT IF EXISTS chk_cuaca_harian_nilai;
ALTER TABLE cuaca_harian DROP COLUMN IF EXISTS data_mentah_id;
ALTER TABLE cuaca_harian DROP COLUMN IF EXISTS eksekusi_id;
-- +goose StatementEnd
