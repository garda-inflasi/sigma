-- Slice 04-D (docs/todos/04-master-data.md): Standardisasi kunci primer master data (BIGINT identity)
-- dan penyusunan urutan kolom tabel (id di paling kiri, atribut utama di tengah, opsional di kanan).
--
-- Tabel master data yang dikonversi:
--   provinsi   -> id BIGINT identity PRIMARY KEY, kode varchar(2) UNIQUE NOT NULL, nama varchar(255) NOT NULL
--   satuan     -> id BIGINT identity PRIMARY KEY, kode varchar(50) UNIQUE NOT NULL, nama varchar(100) NOT NULL
--   kabupaten  -> id BIGINT identity PRIMARY KEY, kode varchar(4) UNIQUE NOT NULL, nama, provinsi_kode, jenis, ibukota, lat, lng
--   kecamatan  -> id BIGINT identity PRIMARY KEY, kode varchar(7) UNIQUE NOT NULL, nama, kabupaten_kode, kode_dagri
--   komoditas  -> id BIGINT identity PRIMARY KEY, kode varchar(255) UNIQUE NOT NULL, nama, satuan, kategori, harga_acuan, status, versi, pilot, shelf_life_hari
--   pasar      -> id BIGINT identity PRIMARY KEY, kode varchar(64) UNIQUE NOT NULL (ID lama), nama, kabupaten_kode, kecamatan_kode, lat, lng, radius_meter, aktif, versi
--
-- Seluruh foreign key yang merujuk master data dikaitkan ke kode unik masing-masing dengan ON DELETE RESTRICT
-- (atau CASCADE untuk komoditas_versi, wilayah_geometri, dan jarak_antar_wilayah).
-- Materialized view agregat_harga_harian, mv_heatmap_harga, dan mv_kpi_operasional dibangun ulang.

-- +goose Up
-- +goose StatementBegin
DO $mig$
DECLARE
    r record;
BEGIN
    -- 1. Matikan sementara view yang bergantung pada pasar dan agregat_harga_harian
    DROP MATERIALIZED VIEW IF EXISTS gold_harga_harian CASCADE;
    DROP MATERIALIZED VIEW IF EXISTS mv_kpi_operasional CASCADE;
    DROP MATERIALIZED VIEW IF EXISTS mv_heatmap_harga CASCADE;
    DROP MATERIALIZED VIEW IF EXISTS agregat_harga_harian CASCADE;
    DROP VIEW IF EXISTS wilayah CASCADE;

    -- 2. Lepas trigger pada komoditas
    DROP TRIGGER IF EXISTS trg_komoditas_versi ON komoditas;

    -- 3. Lepas seluruh foreign key yang mengarah ke 6 tabel master data
    FOR r IN
        SELECT con.conrelid::regclass::text AS anak, con.conname AS nama
          FROM pg_constraint con
         WHERE con.contype = 'f'
           AND con.confrelid IN ('provinsi'::regclass, 'kabupaten'::regclass, 'kecamatan'::regclass,
                                 'satuan'::regclass, 'komoditas'::regclass, 'pasar'::regclass)
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.anak, r.nama);
    END LOOP;

    -- 4. Reorganisasi tabel master data dengan id di sisi kiri dan urutan kolom terstruktur

    -- 4.1. provinsi
    CREATE TABLE provinsi_baru (
        id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        kode varchar(2) NOT NULL,
        nama varchar(255) NOT NULL,
        CONSTRAINT uq_provinsi_kode UNIQUE (kode)
    );
    INSERT INTO provinsi_baru (kode, nama)
    SELECT kode, nama FROM provinsi ORDER BY kode;

    DROP TABLE provinsi CASCADE;
    ALTER TABLE provinsi_baru RENAME TO provinsi;

    -- 4.2. satuan
    CREATE TABLE satuan_baru (
        id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        kode varchar(50) NOT NULL,
        nama varchar(100) NOT NULL,
        CONSTRAINT uq_satuan_kode UNIQUE (kode),
        CONSTRAINT chk_satuan_kode CHECK (length(btrim(kode)) > 0)
    );
    INSERT INTO satuan_baru (kode, nama)
    SELECT kode, nama FROM satuan ORDER BY kode;

    DROP TABLE satuan CASCADE;
    ALTER TABLE satuan_baru RENAME TO satuan;
    COMMENT ON TABLE satuan IS 'Katalog satuan komoditas (kg, liter, ...); diisi seeder master.reference.';

    -- 4.3. kabupaten
    CREATE TABLE kabupaten_baru (
        id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        kode          varchar(4) NOT NULL,
        nama          varchar(255) NOT NULL,
        provinsi_kode varchar(2) NOT NULL REFERENCES provinsi(kode) ON DELETE RESTRICT,
        jenis         varchar(20) NOT NULL,
        ibukota       varchar(255) NOT NULL,
        lat           double precision NOT NULL,
        lng           double precision NOT NULL,
        CONSTRAINT uq_kabupaten_kode UNIQUE (kode),
        CONSTRAINT chk_kabupaten_lat CHECK (lat BETWEEN -90 AND 90),
        CONSTRAINT chk_kabupaten_lng CHECK (lng BETWEEN -180 AND 180)
    );
    INSERT INTO kabupaten_baru (kode, nama, provinsi_kode, jenis, ibukota, lat, lng)
    SELECT kode, nama, provinsi_kode, jenis, ibukota, lat, lng FROM kabupaten ORDER BY kode;

    DROP TABLE kabupaten CASCADE;
    ALTER TABLE kabupaten_baru RENAME TO kabupaten;
    CREATE INDEX idx_kabupaten_provinsi ON kabupaten (provinsi_kode);

    -- 4.4. kecamatan
    CREATE TABLE kecamatan_baru (
        id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        kode           varchar(7) NOT NULL,
        nama           varchar(255) NOT NULL,
        kabupaten_kode varchar(4) NOT NULL REFERENCES kabupaten(kode) ON DELETE RESTRICT,
        kode_dagri     varchar(10),
        CONSTRAINT uq_kecamatan_kode UNIQUE (kode),
        CONSTRAINT chk_kecamatan_kode CHECK (kode ~ '^[0-9]{7}$'),
        CONSTRAINT chk_kecamatan_kabupaten CHECK (left(kode, 4) = kabupaten_kode)
    );

    INSERT INTO kecamatan_baru (kode, nama, kabupaten_kode, kode_dagri)
    SELECT kode, nama, kabupaten_kode, kode_dagri FROM kecamatan ORDER BY kode;

    DROP TABLE kecamatan CASCADE;
    ALTER TABLE kecamatan_baru RENAME TO kecamatan;
    CREATE INDEX idx_kecamatan_kabupaten_kode ON kecamatan (kabupaten_kode);
    COMMENT ON TABLE kecamatan IS 'Kecamatan (kode BPS 7 digit); diimpor dari BPS lewat seeder master.reference.';
    COMMENT ON COLUMN kecamatan.kode_dagri IS 'Kode Kemendagri padanan (dari API bridging BPS), untuk pencocokan sumber lain.';

    -- 4.5. komoditas
    CREATE TABLE komoditas_baru (
        id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        kode            varchar(255) NOT NULL,
        nama            varchar(255) NOT NULL,
        satuan          varchar(50) NOT NULL REFERENCES satuan(kode) ON DELETE RESTRICT,
        kategori        varchar(100) NOT NULL,
        harga_acuan     integer NOT NULL,
        status          status_master NOT NULL DEFAULT 'AKTIF',
        versi           integer NOT NULL DEFAULT 1,
        pilot           boolean NOT NULL DEFAULT false,
        shelf_life_hari integer NOT NULL,
        CONSTRAINT uq_komoditas_kode UNIQUE (kode),
        CONSTRAINT chk_komoditas_harga_acuan CHECK (harga_acuan > 0),
        CONSTRAINT chk_komoditas_shelf_life CHECK (shelf_life_hari > 0)
    );

    INSERT INTO komoditas_baru (kode, nama, satuan, kategori, harga_acuan, status, versi, pilot, shelf_life_hari)
    SELECT kode, nama, satuan, kategori, harga_acuan, status, versi, pilot, shelf_life_hari FROM komoditas ORDER BY kode;

    DROP TABLE komoditas CASCADE;
    ALTER TABLE komoditas_baru RENAME TO komoditas;
    CREATE INDEX idx_komoditas_satuan ON komoditas (satuan);

    CREATE TRIGGER trg_komoditas_versi
        AFTER INSERT OR UPDATE ON komoditas
        FOR EACH ROW EXECUTE FUNCTION komoditas_catat_versi();

    -- 4.6. pasar
    CREATE TABLE pasar_baru (
        id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
        kode           varchar(255) NOT NULL,
        nama           varchar(255) NOT NULL,
        kabupaten_kode varchar(4) NOT NULL REFERENCES kabupaten(kode) ON DELETE RESTRICT,
        kecamatan_kode varchar(7) REFERENCES kecamatan(kode) ON DELETE RESTRICT,
        lat            double precision NOT NULL,
        lng            double precision NOT NULL,
        radius_meter   integer NOT NULL DEFAULT 400,
        aktif          boolean NOT NULL DEFAULT true,
        versi          integer NOT NULL DEFAULT 1,
        CONSTRAINT uq_pasar_kode UNIQUE (kode),
        CONSTRAINT chk_pasar_lat CHECK (lat BETWEEN -90 AND 90),
        CONSTRAINT chk_pasar_lng CHECK (lng BETWEEN -180 AND 180),
        CONSTRAINT chk_pasar_radius_meter CHECK (radius_meter > 0),
        CONSTRAINT chk_pasar_versi CHECK (versi > 0),
        CONSTRAINT chk_pasar_kecamatan_kabupaten CHECK (kecamatan_kode IS NULL OR left(kecamatan_kode, 4) = kabupaten_kode)
    );

    INSERT INTO pasar_baru (kode, nama, kabupaten_kode, kecamatan_kode, lat, lng, radius_meter, aktif, versi)
    SELECT id, nama, kabupaten_kode, kecamatan_kode, lat, lng, radius_meter, aktif, versi FROM pasar ORDER BY id;

    DROP TABLE pasar CASCADE;
    ALTER TABLE pasar_baru RENAME TO pasar;
    CREATE INDEX idx_pasar_kabupaten ON pasar (kabupaten_kode);
    CREATE INDEX idx_pasar_kecamatan_kode ON pasar (kecamatan_kode);

    -- 5. Hubungkan kembali seluruh Foreign Key ke tabel master data
    ALTER TABLE cuaca_harian ADD CONSTRAINT cuaca_harian_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE pengguna ADD CONSTRAINT pengguna_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE batas_harga ADD CONSTRAINT batas_harga_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE batas_harga ADD CONSTRAINT batas_harga_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE aturan_ews ADD CONSTRAINT aturan_ews_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE aturan_ews ADD CONSTRAINT aturan_ews_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE wilayah_geometri ADD CONSTRAINT wilayah_geometri_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE CASCADE;
    ALTER TABLE cold_storage ADD CONSTRAINT cold_storage_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE kandidat_cold_storage ADD CONSTRAINT kandidat_cold_storage_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE stok_cold_storage ADD CONSTRAINT stok_cold_storage_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE optimasi_rute ADD CONSTRAINT optimasi_rute_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE rute_distribusi ADD CONSTRAINT rute_distribusi_asal_kode_fkey FOREIGN KEY (asal_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE rute_distribusi ADD CONSTRAINT rute_distribusi_tujuan_kode_fkey FOREIGN KEY (tujuan_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE rute_distribusi ADD CONSTRAINT rute_distribusi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE neraca_wilayah ADD CONSTRAINT neraca_wilayah_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE neraca_wilayah ADD CONSTRAINT neraca_wilayah_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE neraca_pangan ADD CONSTRAINT neraca_pangan_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE jarak_antar_wilayah ADD CONSTRAINT jarak_antar_wilayah_asal_kode_fkey FOREIGN KEY (asal_kode) REFERENCES kabupaten(kode) ON DELETE CASCADE;
    ALTER TABLE jarak_antar_wilayah ADD CONSTRAINT jarak_antar_wilayah_tujuan_kode_fkey FOREIGN KEY (tujuan_kode) REFERENCES kabupaten(kode) ON DELETE CASCADE;
    ALTER TABLE siklus_ekonomi ADD CONSTRAINT siklus_ekonomi_wilayah_kode_fkey FOREIGN KEY (wilayah_kode) REFERENCES provinsi(kode) ON DELETE RESTRICT;
    ALTER TABLE kpi_nilai ADD CONSTRAINT kpi_nilai_wilayah_kode_fkey FOREIGN KEY (wilayah_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE kpi_target ADD CONSTRAINT kpi_target_wilayah_kode_fkey FOREIGN KEY (wilayah_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE andil_inflasi ADD CONSTRAINT andil_inflasi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE model_registry ADD CONSTRAINT model_registry_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE komoditas_versi ADD CONSTRAINT komoditas_versi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE CASCADE;
    ALTER TABLE harga_eksternal ADD CONSTRAINT harga_eksternal_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE harga_eksternal ADD CONSTRAINT harga_eksternal_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE entri_harga ADD CONSTRAINT entri_harga_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE entri_harga ADD CONSTRAINT entri_harga_pasar_id_fkey FOREIGN KEY (pasar_id) REFERENCES pasar(kode) ON DELETE RESTRICT;
    ALTER TABLE tugas_lapangan ADD CONSTRAINT tugas_lapangan_pasar_id_fkey FOREIGN KEY (pasar_id) REFERENCES pasar(kode) ON DELETE RESTRICT;
    ALTER TABLE pedagang ADD CONSTRAINT pedagang_pasar_id_fkey FOREIGN KEY (pasar_id) REFERENCES pasar(kode) ON DELETE RESTRICT;

    -- 6. Bangun ulang Materialized Views
    CREATE MATERIALIZED VIEW agregat_harga_harian AS
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
    JOIN pasar p ON p.kode = e.pasar_id
    WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI')
    GROUP BY e.komoditas_kode, p.kabupaten_kode, e.tipe, (e.dicatat_pada AT TIME ZONE 'Asia/Jakarta')::date
    WITH DATA;

    CREATE UNIQUE INDEX uq_agregat_harga_harian
        ON agregat_harga_harian(komoditas_kode, kabupaten_kode, tipe, tanggal);
    CREATE INDEX idx_agregat_harga_harian_tanggal ON agregat_harga_harian(tanggal);

    CREATE MATERIALIZED VIEW mv_heatmap_harga AS
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

    CREATE UNIQUE INDEX uq_mv_heatmap_harga ON mv_heatmap_harga(komoditas_kode, kabupaten_kode);

    CREATE MATERIALIZED VIEW mv_kpi_operasional AS
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
           COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI')) / NULLIF(COUNT(*), 0)
                       FROM entri e
                      WHERE e.tanggal BETWEEN h.tanggal - 6 AND h.tanggal), 0)::double precision
      FROM hari h
    UNION ALL
    SELECT 'waspada', h.tanggal,
           (SELECT COUNT(DISTINCT komoditas_kode)
              FROM harian a
             WHERE a.tanggal = h.tanggal AND a.basis > 0 AND (a.rata_rata - a.basis) / a.basis > 0.07)::double precision
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

    CREATE UNIQUE INDEX uq_mv_kpi_operasional ON mv_kpi_operasional(indikator, tanggal);

    CREATE MATERIALIZED VIEW gold_harga_harian AS
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

    CREATE UNIQUE INDEX uq_gold_harga_harian ON gold_harga_harian(komoditas_kode, kabupaten_kode, tanggal);

    CREATE OR REPLACE VIEW wilayah AS
        SELECT kode, nama, lat, lng, LOWER(jenis) AS level
          FROM kabupaten;

    -- 7. Hak akses aplikasi
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        PERFORM garda_terapkan_hak_aplikasi();
    END IF;
END
$mig$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $mig$
DECLARE
    r record;
BEGIN
    DROP MATERIALIZED VIEW IF EXISTS gold_harga_harian CASCADE;
    DROP MATERIALIZED VIEW IF EXISTS mv_kpi_operasional CASCADE;
    DROP MATERIALIZED VIEW IF EXISTS mv_heatmap_harga CASCADE;
    DROP MATERIALIZED VIEW IF EXISTS agregat_harga_harian CASCADE;
    DROP VIEW IF EXISTS wilayah CASCADE;

    DROP TRIGGER IF EXISTS trg_komoditas_versi ON komoditas;

    FOR r IN
        SELECT con.conrelid::regclass::text AS anak, con.conname AS nama
          FROM pg_constraint con
         WHERE con.contype = 'f'
           AND con.confrelid IN ('provinsi'::regclass, 'kabupaten'::regclass, 'kecamatan'::regclass,
                                 'satuan'::regclass, 'komoditas'::regclass, 'pasar'::regclass)
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.anak, r.nama);
    END LOOP;

    -- Kembalikan tabel ke skema sebelum 000122
    CREATE TABLE provinsi_lama (
        kode varchar(2) PRIMARY KEY,
        nama varchar(255) NOT NULL
    );
    INSERT INTO provinsi_lama (kode, nama) SELECT kode, nama FROM provinsi ORDER BY kode;
    DROP TABLE provinsi CASCADE;
    ALTER TABLE provinsi_lama RENAME TO provinsi;

    CREATE TABLE satuan_lama (
        kode varchar(50) PRIMARY KEY,
        nama varchar(100) NOT NULL,
        CONSTRAINT chk_satuan_kode CHECK (length(btrim(kode)) > 0)
    );
    INSERT INTO satuan_lama (kode, nama) SELECT kode, nama FROM satuan ORDER BY kode;
    DROP TABLE satuan CASCADE;
    ALTER TABLE satuan_lama RENAME TO satuan;

    CREATE TABLE kabupaten_lama (
        kode varchar(4) PRIMARY KEY,
        nama varchar(255) NOT NULL,
        jenis varchar(20) NOT NULL,
        ibukota varchar(255) NOT NULL,
        lat double precision NOT NULL,
        lng double precision NOT NULL,
        provinsi_kode varchar(2) NOT NULL,
        CONSTRAINT kabupaten_provinsi_kode_fkey FOREIGN KEY (provinsi_kode) REFERENCES provinsi(kode),
        CONSTRAINT chk_kabupaten_lat CHECK (lat BETWEEN -90 AND 90),
        CONSTRAINT chk_kabupaten_lng CHECK (lng BETWEEN -180 AND 180)
    );
    INSERT INTO kabupaten_lama (kode, nama, jenis, ibukota, lat, lng, provinsi_kode)
    SELECT kode, nama, jenis, ibukota, lat, lng, provinsi_kode FROM kabupaten ORDER BY kode;
    DROP TABLE kabupaten CASCADE;
    ALTER TABLE kabupaten_lama RENAME TO kabupaten;
    CREATE INDEX idx_kabupaten_provinsi ON kabupaten (provinsi_kode);

    CREATE TABLE kecamatan_lama (
        kode varchar(7) PRIMARY KEY,
        nama varchar(255) NOT NULL,
        kabupaten_kode varchar(4) NOT NULL,
        kode_dagri varchar(10),
        CONSTRAINT kecamatan_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT,
        CONSTRAINT chk_kecamatan_kode CHECK (kode ~ '^[0-9]{7}$'),
        CONSTRAINT chk_kecamatan_kabupaten CHECK (left(kode, 4) = kabupaten_kode)
    );
    INSERT INTO kecamatan_lama (kode, nama, kabupaten_kode, kode_dagri)
    SELECT kode, nama, kabupaten_kode, kode_dagri FROM kecamatan ORDER BY kode;
    DROP TABLE kecamatan CASCADE;
    ALTER TABLE kecamatan_lama RENAME TO kecamatan;
    CREATE INDEX idx_kecamatan_kabupaten_kode ON kecamatan (kabupaten_kode);

    CREATE TABLE komoditas_lama (
        kode varchar(255) PRIMARY KEY,
        nama varchar(255) NOT NULL,
        satuan varchar(50) NOT NULL,
        kategori varchar(100) NOT NULL,
        harga_acuan integer NOT NULL,
        pilot boolean NOT NULL DEFAULT false,
        shelf_life_hari integer NOT NULL,
        versi integer NOT NULL DEFAULT 1,
        status status_master NOT NULL DEFAULT 'AKTIF',
        CONSTRAINT chk_komoditas_harga_acuan CHECK (harga_acuan > 0),
        CONSTRAINT chk_komoditas_shelf_life CHECK (shelf_life_hari > 0),
        CONSTRAINT komoditas_satuan_fkey FOREIGN KEY (satuan) REFERENCES satuan(kode) ON DELETE RESTRICT
    );
    INSERT INTO komoditas_lama (kode, nama, satuan, kategori, harga_acuan, pilot, shelf_life_hari, versi, status)
    SELECT kode, nama, satuan, kategori, harga_acuan, pilot, shelf_life_hari, versi, status FROM komoditas ORDER BY kode;
    DROP TABLE komoditas CASCADE;
    ALTER TABLE komoditas_lama RENAME TO komoditas;
    CREATE INDEX idx_komoditas_satuan ON komoditas (satuan);

    CREATE TRIGGER trg_komoditas_versi
        AFTER INSERT OR UPDATE ON komoditas
        FOR EACH ROW EXECUTE FUNCTION komoditas_catat_versi();

    CREATE TABLE pasar_lama (
        id varchar(255) PRIMARY KEY,
        nama varchar(255) NOT NULL,
        kabupaten_kode varchar(4) NOT NULL,
        lat double precision NOT NULL,
        lng double precision NOT NULL,
        radius_meter integer NOT NULL DEFAULT 400,
        aktif boolean NOT NULL DEFAULT true,
        kecamatan_kode varchar(7),
        versi integer NOT NULL DEFAULT 1,
        CONSTRAINT pasar_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT,
        CONSTRAINT pasar_kecamatan_kode_fkey FOREIGN KEY (kecamatan_kode) REFERENCES kecamatan(kode) ON DELETE RESTRICT,
        CONSTRAINT chk_pasar_lat CHECK (lat BETWEEN -90 AND 90),
        CONSTRAINT chk_pasar_lng CHECK (lng BETWEEN -180 AND 180),
        CONSTRAINT chk_pasar_radius_meter CHECK (radius_meter > 0),
        CONSTRAINT chk_pasar_versi CHECK (versi > 0),
        CONSTRAINT chk_pasar_kecamatan_kabupaten CHECK (kecamatan_kode IS NULL OR left(kecamatan_kode, 4) = kabupaten_kode)
    );
    INSERT INTO pasar_lama (id, nama, kabupaten_kode, lat, lng, radius_meter, aktif, kecamatan_kode, versi)
    SELECT kode, nama, kabupaten_kode, lat, lng, radius_meter, aktif, kecamatan_kode, versi FROM pasar ORDER BY kode;
    DROP TABLE pasar CASCADE;
    ALTER TABLE pasar_lama RENAME TO pasar;
    CREATE INDEX idx_pasar_kabupaten ON pasar (kabupaten_kode);
    CREATE INDEX idx_pasar_kecamatan_kode ON pasar (kecamatan_kode);

    -- Pasang kembali Foreign Key lama
    ALTER TABLE cuaca_harian ADD CONSTRAINT cuaca_harian_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE pengguna ADD CONSTRAINT pengguna_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE batas_harga ADD CONSTRAINT batas_harga_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE batas_harga ADD CONSTRAINT batas_harga_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE aturan_ews ADD CONSTRAINT aturan_ews_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE aturan_ews ADD CONSTRAINT aturan_ews_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE wilayah_geometri ADD CONSTRAINT wilayah_geometri_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE cold_storage ADD CONSTRAINT cold_storage_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE kandidat_cold_storage ADD CONSTRAINT kandidat_cold_storage_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE stok_cold_storage ADD CONSTRAINT stok_cold_storage_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE optimasi_rute ADD CONSTRAINT optimasi_rute_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE rute_distribusi ADD CONSTRAINT rute_distribusi_asal_kode_fkey FOREIGN KEY (asal_kode) REFERENCES kabupaten(kode);
    ALTER TABLE rute_distribusi ADD CONSTRAINT rute_distribusi_tujuan_kode_fkey FOREIGN KEY (tujuan_kode) REFERENCES kabupaten(kode);
    ALTER TABLE rute_distribusi ADD CONSTRAINT rute_distribusi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE neraca_wilayah ADD CONSTRAINT neraca_wilayah_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE neraca_wilayah ADD CONSTRAINT neraca_wilayah_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE neraca_pangan ADD CONSTRAINT neraca_pangan_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE jarak_antar_wilayah ADD CONSTRAINT jarak_antar_wilayah_asal_kode_fkey FOREIGN KEY (asal_kode) REFERENCES kabupaten(kode);
    ALTER TABLE jarak_antar_wilayah ADD CONSTRAINT jarak_antar_wilayah_tujuan_kode_fkey FOREIGN KEY (tujuan_kode) REFERENCES kabupaten(kode);
    ALTER TABLE siklus_ekonomi ADD CONSTRAINT siklus_ekonomi_wilayah_kode_fkey FOREIGN KEY (wilayah_kode) REFERENCES provinsi(kode);
    ALTER TABLE kpi_nilai ADD CONSTRAINT kpi_nilai_wilayah_kode_fkey FOREIGN KEY (wilayah_kode) REFERENCES kabupaten(kode);
    ALTER TABLE kpi_target ADD CONSTRAINT kpi_target_wilayah_kode_fkey FOREIGN KEY (wilayah_kode) REFERENCES kabupaten(kode);
    ALTER TABLE andil_inflasi ADD CONSTRAINT andil_inflasi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE model_registry ADD CONSTRAINT model_registry_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE komoditas_versi ADD CONSTRAINT komoditas_versi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE harga_eksternal ADD CONSTRAINT harga_eksternal_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE harga_eksternal ADD CONSTRAINT harga_eksternal_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE entri_harga ADD CONSTRAINT entri_harga_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE entri_harga ADD CONSTRAINT entri_harga_pasar_id_fkey FOREIGN KEY (pasar_id) REFERENCES pasar(id);
    ALTER TABLE tugas_lapangan ADD CONSTRAINT tugas_lapangan_pasar_id_fkey FOREIGN KEY (pasar_id) REFERENCES pasar(id);
    ALTER TABLE pedagang ADD CONSTRAINT pedagang_pasar_id_fkey FOREIGN KEY (pasar_id) REFERENCES pasar(id);

    CREATE MATERIALIZED VIEW agregat_harga_harian AS
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

    CREATE UNIQUE INDEX uq_agregat_harga_harian
        ON agregat_harga_harian(komoditas_kode, kabupaten_kode, tipe, tanggal);
    CREATE INDEX idx_agregat_harga_harian_tanggal ON agregat_harga_harian(tanggal);

    CREATE MATERIALIZED VIEW mv_heatmap_harga AS
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

    CREATE UNIQUE INDEX uq_mv_heatmap_harga ON mv_heatmap_harga(komoditas_kode, kabupaten_kode);

    CREATE MATERIALIZED VIEW mv_kpi_operasional AS
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
           COALESCE((SELECT 100.0 * COUNT(*) FILTER (WHERE status IN ('DIVALIDASI', 'DIPUBLIKASI')) / NULLIF(COUNT(*), 0)
                       FROM entri e
                      WHERE e.tanggal BETWEEN h.tanggal - 6 AND h.tanggal), 0)::double precision
      FROM hari h
    UNION ALL
    SELECT 'waspada', h.tanggal,
           (SELECT COUNT(DISTINCT komoditas_kode)
              FROM harian a
             WHERE a.tanggal = h.tanggal AND a.basis > 0 AND (a.rata_rata - a.basis) / a.basis > 0.07)::double precision
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

    CREATE UNIQUE INDEX uq_mv_kpi_operasional ON mv_kpi_operasional(indikator, tanggal);

    CREATE MATERIALIZED VIEW gold_harga_harian AS
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

    CREATE UNIQUE INDEX uq_gold_harga_harian ON gold_harga_harian(komoditas_kode, kabupaten_kode, tanggal);

    CREATE OR REPLACE VIEW wilayah AS
        SELECT kode, nama, lat, lng, LOWER(jenis) AS level
          FROM kabupaten;

    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        PERFORM garda_terapkan_hak_aplikasi();
    END IF;
END
$mig$;
-- +goose StatementEnd
