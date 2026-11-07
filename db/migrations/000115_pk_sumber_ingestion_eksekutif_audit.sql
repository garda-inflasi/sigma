-- Standardisasi kunci primer (docs/SKEMA-DATABASE.md), langkah 6: sumber data, ingestion, eksekutif, siklus,
-- KPI, dashboard, dan audit.
--
-- Ringkasan per tabel (kategori PK mengikuti kategoriPK di internal/platform/schematest/kunci_test.go):
--
--   sumber_data       VARCHAR 'SRC-...'  -> UUID   id = garda_uuid_stabil('sumber_data', id_lama); kode = id_lama (NOT NULL,
--                                                  UNIQUE): kode tampilan, alias rute /v1/sumber-data/:id, dan kunci
--                                                  konfigurasi konektor/CLI ingestion.
--   eksekusi_sumber   BIGSERIAL tetap            sumber_id uuid CASCADE (riwayat run milik sumber).
--   data_mentah       BIGSERIAL tetap            sumber_id uuid CASCADE, eksekusi_id CASCADE; BRIN diambil_pada.
--   pemetaan_sumber   KOMPOSIT tetap             sumber_id uuid CASCADE (konfigurasi milik sumber).
--   harga_eksternal   PK komposit alami -> BIGINT id + UNIQUE alami (target upsert ingestion); sumber_id uuid RESTRICT
--                                                  (bukti asal data: sumber yang punya harga dinonaktifkan, bukan dihapus),
--                                                  komoditas/kabupaten RESTRICT, data_mentah_id/eksekusi_id SET NULL.
--   cuaca_harian      VARCHAR 'CUA-...'  -> BIGINT (urut tanggal, id); kode = id lama; UNIQUE alami
--                                                  (kabupaten_kode, tanggal, sumber) sudah ada; kabupaten RESTRICT,
--                                                  eksekusi_id/data_mentah_id SET NULL.
--   andil_inflasi, inflasi_bulanan, neraca_pangan, target_kpi
--                     PK komposit alami -> BIGINT id + UNIQUE alami; FK komoditas RESTRICT.
--   ruang_fiskal      VARCHAR -> BIGINT (urut diimpor_pada, id); kode = id lama; UNIQUE (tahun, instrumen, per_tanggal).
--   siklus_ekonomi    VARCHAR -> BIGINT (urut dibuat_pada, id); kode = id lama; wilayah_kode -> provinsi RESTRICT.
--   kpi_nilai, kpi_target
--                     BIGSERIAL tetap            wilayah_kode -> kabupaten RESTRICT + indeks; kpi_kode CASCADE tetap.
--   audit_log         VARCHAR (UUID teks) -> BIGINT identity (urut waktu, id); kode = id lama (alias rute /v1/audit/:id
--                                                  dan kunci idempoten seeder demo); BRIN waktu.
--
-- Cara konversi VARCHAR -> BIGINT: kolom id_baru diisi row_number() menurut urutan di atas, lalu
-- ALTER COLUMN id TYPE bigint USING id_baru (USING boleh membaca kolom lain pada baris yang sama). Cara ini menjaga
-- constraint PK, posisi kolom, dan indeks yang memuat id (mis. idx_audit_log_waktu_id), yang dibangun ulang otomatis.
-- Identity lalu dipasang dengan START WITH max(id)+1. Down memakai jalan sebaliknya: DROP IDENTITY, lalu
-- ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text), sehingga baris yang dibuat sesudah Up memakai
-- id::text.
--
-- audit_log: append-only dengan dua pemicu (trg_audit_log_append_only dan trg_audit_log_no_truncate). Keduanya
-- dimatikan hanya selama UPDATE konversi di transaksi yang sama (kolom kode, id_baru, dan rujukan lunak), lalu
-- dinyalakan lagi sebelum transaksi selesai (Up maupun Down). Tidak ada rantai hash atau kolom turunan dari id:
-- perlindungan tamper-evidence tabel ini hanya kedua pemicu itu (diperiksa: tidak ada kolom hash, dan tidak ada
-- fungsi atau indeks yang menghitung nilai dari id), jadi mengganti id tidak merusak verifikasi apa pun.
--
-- Rujukan lunak audit_log.entitas_id (teks, tanpa FK) yang dipetakan ke ID baru bila nilainya ID lama baris yang ada:
--   * entitas 'sumber_data'            : entitas_id = sumber_data.kode  -> sumber_data.id::text (UUID baru).
--   * entitas 'langganan_push' dan 'preferensi_notifikasi': slug pengguna lama di entitas_id (belum dipetakan 000110)
--     -> pengguna.id::text lewat pengguna_id_lama.
--   Down memetakan balik. Entitas 'kpi_nilai' tidak dipetakan: aplikasi menulis rentang periode ('2026-07..2026-09')
--   atau 'laporan' di entitas_id, bukan ID baris. Snapshot JSON (sebelum/sesudah/metadata, mis. eksekusi_id) dibiarkan
--   sebagai sejarah.
--
-- Indeks audit_log: idx_audit_log_entitas (entitas, entitas_id) dan idx_audit_log_waktu (waktu) menjadi redundan
-- (awalan idx_audit_log_entitas_waktu dan idx_audit_log_waktu_id) dan dibuang di Up, dibuat lagi di Down. BRIN
-- idx_audit_log_waktu_brin melayani rentang waktu pada log yang bertambah menurut waktu. idx_audit_log_aktor_waktu
-- (aktor_id) dipertahankan.
--
-- View: gold_harga_harian (materialized) membaca harga_eksternal(komoditas_kode, kabupaten_kode, tanggal, tipe, harga).
-- Tidak satu pun dari kolom itu berubah tipe, dan PK/sumber_id tidak dipakainya, jadi view dibiarkan.
--
-- ALTER COLUMN TYPE menulis ulang tabel dengan kunci ACCESS EXCLUSIVE (data_mentah menyimpan payload JSONB besar,
-- audit_log bertambah terus); jalankan di jendela pemeliharaan untuk basis data besar.

-- +goose Up
-- +goose StatementBegin
DO $mig$
DECLARE
    r        record;
    jumlah   bigint;
    berikut  bigint;
BEGIN
    -- Rencana FK: tabel, kolom, nama constraint, induk, ON DELETE baru, ON DELETE semula, indeks baru (NULL bila
    -- sudah terlayani indeks lain yang berawalan kolom itu).
    CREATE TEMP TABLE g5_fk (
        tabel       text NOT NULL,
        kolom       text NOT NULL,
        nama        text NOT NULL,
        induk       text NOT NULL,
        kolom_induk text NOT NULL,
        hapus_baru  text NOT NULL,
        hapus_lama  text NOT NULL,
        indeks      text
    ) ON COMMIT DROP;

    INSERT INTO g5_fk (tabel, kolom, nama, induk, kolom_induk, hapus_baru, hapus_lama, indeks) VALUES
        ('eksekusi_sumber', 'sumber_id',      'eksekusi_sumber_sumber_id_fkey',     'sumber_data',     'id',   'CASCADE',  'NO ACTION', NULL),
        ('data_mentah',     'sumber_id',      'data_mentah_sumber_id_fkey',         'sumber_data',     'id',   'CASCADE',  'NO ACTION', NULL),
        ('data_mentah',     'eksekusi_id',    'data_mentah_eksekusi_id_fkey',       'eksekusi_sumber', 'id',   'CASCADE',  'NO ACTION', NULL),
        ('pemetaan_sumber', 'sumber_id',      'pemetaan_sumber_sumber_id_fkey',     'sumber_data',     'id',   'CASCADE',  'NO ACTION', NULL),
        ('harga_eksternal', 'sumber_id',      'harga_eksternal_sumber_id_fkey',     'sumber_data',     'id',   'RESTRICT', 'NO ACTION', NULL),
        ('harga_eksternal', 'komoditas_kode', 'harga_eksternal_komoditas_kode_fkey','komoditas',      'kode', 'RESTRICT', 'NO ACTION', NULL),
        ('harga_eksternal', 'kabupaten_kode', 'harga_eksternal_kabupaten_kode_fkey','kabupaten',      'kode', 'RESTRICT', 'NO ACTION', 'idx_harga_eksternal_kabupaten_kode'),
        ('harga_eksternal', 'data_mentah_id', 'harga_eksternal_data_mentah_id_fkey','data_mentah',    'id',   'SET NULL', 'NO ACTION', 'idx_harga_eksternal_data_mentah_id'),
        ('harga_eksternal', 'eksekusi_id',    'harga_eksternal_eksekusi_id_fkey',   'eksekusi_sumber', 'id',   'SET NULL', 'NO ACTION', 'idx_harga_eksternal_eksekusi_id'),
        ('cuaca_harian',    'kabupaten_kode', 'cuaca_harian_kabupaten_kode_fkey',   'kabupaten',       'kode', 'RESTRICT', 'NO ACTION', NULL),
        ('cuaca_harian',    'eksekusi_id',    'cuaca_harian_eksekusi_id_fkey',      'eksekusi_sumber', 'id',   'SET NULL', 'NO ACTION', 'idx_cuaca_harian_eksekusi_id'),
        ('cuaca_harian',    'data_mentah_id', 'cuaca_harian_data_mentah_id_fkey',   'data_mentah',     'id',   'SET NULL', 'NO ACTION', 'idx_cuaca_harian_data_mentah_id'),
        ('andil_inflasi',   'komoditas_kode', 'andil_inflasi_komoditas_kode_fkey',  'komoditas',       'kode', 'RESTRICT', 'NO ACTION', 'idx_andil_inflasi_komoditas_kode'),
        ('neraca_pangan',   'komoditas_kode', 'neraca_pangan_komoditas_kode_fkey',  'komoditas',       'kode', 'RESTRICT', 'NO ACTION', NULL),
        ('siklus_ekonomi',  'wilayah_kode',   'siklus_ekonomi_wilayah_kode_fkey',   'provinsi',        'kode', 'RESTRICT', 'NO ACTION', NULL),
        ('kpi_nilai',       'wilayah_kode',   'kpi_nilai_wilayah_kode_fkey',        'kabupaten',       'kode', 'RESTRICT', 'NO ACTION', 'idx_kpi_nilai_wilayah_kode'),
        ('kpi_target',      'wilayah_kode',   'kpi_target_wilayah_kode_fkey',       'kabupaten',       'kode', 'RESTRICT', 'NO ACTION', 'idx_kpi_target_wilayah_kode');

    -- Penjaga 1: setiap FK ke sumber_data harus ada di rencana; FK yang terlewat membuat konversi tipe gagal di
    -- tengah jalan, jadi lebih baik berhenti dengan pesan yang jelas.
    FOR r IN
        SELECT con.conrelid::regclass::text AS tabel, con.conname AS nama
          FROM pg_constraint con
         WHERE con.contype = 'f' AND con.confrelid = 'sumber_data'::regclass
           AND NOT EXISTS (SELECT 1 FROM g5_fk p WHERE p.tabel = con.conrelid::regclass::text AND p.nama = con.conname)
    LOOP
        RAISE EXCEPTION 'FK % pada % merujuk sumber_data tetapi belum ada di rencana migrasi 000115', r.nama, r.tabel;
    END LOOP;

    -- Penjaga 2: ID lama yang dijadikan kode harus muat di varchar(64).
    FOR r IN SELECT unnest(ARRAY['sumber_data', 'cuaca_harian', 'ruang_fiskal', 'siklus_ekonomi', 'audit_log']) AS tabel LOOP
        EXECUTE format('SELECT count(*) FROM %I WHERE length(id) > 64', r.tabel) INTO jumlah;
        IF jumlah > 0 THEN
            RAISE EXCEPTION '% baris % punya id lebih dari 64 karakter; kolom kode varchar(64) tidak muat', jumlah, r.tabel;
        END IF;
    END LOOP;

    -- 1. Lepas semua FK yang aturannya diganti atau tipenya berubah. FK dipasang lagi di langkah 8.
    FOR r IN SELECT * FROM g5_fk LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.nama);
    END LOOP;

    -- 2. PK komposit alami -> BIGINT id + UNIQUE alami. harga_eksternal: PK dilepas sebelum sumber_id berubah tipe supaya
    --    indeksnya tidak dibangun dua kali; UNIQUE dan PK baru dipasang sesudahnya (langkah 4).
    ALTER TABLE harga_eksternal ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;
    ALTER TABLE harga_eksternal DROP CONSTRAINT harga_eksternal_pkey;

    -- 3. sumber_data: VARCHAR -> UUID. kode = ID lama; anak (sumber_id) dikonversi dengan rumus yang sama.
    ALTER TABLE sumber_data ADD COLUMN kode varchar(64);
    UPDATE sumber_data SET kode = id;
    ALTER TABLE sumber_data ALTER COLUMN kode SET NOT NULL;
    ALTER TABLE sumber_data ADD CONSTRAINT uq_sumber_data_kode UNIQUE (kode);
    ALTER TABLE sumber_data ALTER COLUMN id TYPE uuid USING garda_uuid_stabil('sumber_data', id);
    ALTER TABLE sumber_data ALTER COLUMN id SET DEFAULT gen_random_uuid();
    ALTER TABLE eksekusi_sumber ALTER COLUMN sumber_id TYPE uuid USING garda_uuid_stabil('sumber_data', sumber_id);
    ALTER TABLE data_mentah     ALTER COLUMN sumber_id TYPE uuid USING garda_uuid_stabil('sumber_data', sumber_id);
    ALTER TABLE pemetaan_sumber ALTER COLUMN sumber_id TYPE uuid USING garda_uuid_stabil('sumber_data', sumber_id);
    ALTER TABLE harga_eksternal ALTER COLUMN sumber_id TYPE uuid USING garda_uuid_stabil('sumber_data', sumber_id);

    -- 4. Kunci komposit alami menjadi UNIQUE (target ON CONFLICT upsert) dan PK baru id.
    ALTER TABLE harga_eksternal
        ADD CONSTRAINT uq_harga_eksternal_alami UNIQUE (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe),
        ADD CONSTRAINT harga_eksternal_pkey PRIMARY KEY (id);

    ALTER TABLE andil_inflasi ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;
    ALTER TABLE andil_inflasi DROP CONSTRAINT andil_inflasi_pkey;
    ALTER TABLE andil_inflasi
        ADD CONSTRAINT uq_andil_inflasi_alami UNIQUE (periode, wilayah_kode, komoditas_kode),
        ADD CONSTRAINT andil_inflasi_pkey PRIMARY KEY (id);

    ALTER TABLE inflasi_bulanan ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;
    ALTER TABLE inflasi_bulanan DROP CONSTRAINT inflasi_bulanan_pkey;
    ALTER TABLE inflasi_bulanan
        ADD CONSTRAINT uq_inflasi_bulanan_alami UNIQUE (wilayah_kode, periode),
        ADD CONSTRAINT inflasi_bulanan_pkey PRIMARY KEY (id);

    ALTER TABLE neraca_pangan ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;
    ALTER TABLE neraca_pangan DROP CONSTRAINT neraca_pangan_pkey;
    ALTER TABLE neraca_pangan
        ADD CONSTRAINT uq_neraca_pangan_alami UNIQUE (komoditas_kode, wilayah_kode, periode),
        ADD CONSTRAINT neraca_pangan_pkey PRIMARY KEY (id);

    ALTER TABLE target_kpi ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;
    ALTER TABLE target_kpi DROP CONSTRAINT target_kpi_pkey;
    ALTER TABLE target_kpi
        ADD CONSTRAINT uq_target_kpi_alami UNIQUE (indikator, periode_mulai),
        ADD CONSTRAINT target_kpi_pkey PRIMARY KEY (id);

    -- 5. VARCHAR -> BIGINT identity. ID baru berurutan menurut kolom waktu tiap tabel, lalu ID lama sebagai pemutus
    --    seri (urutan byte, COLLATE "C", supaya hasilnya sama di semua klaster apa pun kolasi bawaannya).
    --    audit_log append-only: pemicunya dimatikan hanya selama UPDATE di bawah.
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_no_truncate;

    FOR r IN
        SELECT * FROM (VALUES
            ('cuaca_harian',  'tanggal'),
            ('ruang_fiskal',  'diimpor_pada'),
            ('siklus_ekonomi','dibuat_pada'),
            ('audit_log',     'waktu')
        ) AS t(tabel, urut)
    LOOP
        EXECUTE format('ALTER TABLE %I ADD COLUMN kode varchar(64)', r.tabel);
        EXECUTE format('UPDATE %I SET kode = id', r.tabel);
        EXECUTE format('ALTER TABLE %I ADD COLUMN id_baru bigint', r.tabel);
        EXECUTE format(
            'UPDATE %I t SET id_baru = n.nomor FROM (SELECT id, row_number() OVER (ORDER BY %I NULLS LAST, id COLLATE "C") AS nomor FROM %I) n WHERE t.id = n.id',
            r.tabel, r.urut, r.tabel);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id TYPE bigint USING id_baru', r.tabel);
        EXECUTE format('ALTER TABLE %I DROP COLUMN id_baru', r.tabel);
        EXECUTE format('SELECT COALESCE(max(id), 0) + 1 FROM %I', r.tabel) INTO berikut;
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', r.tabel, berikut);
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I UNIQUE (kode)', r.tabel, 'uq_' || r.tabel || '_kode');
    END LOOP;

    -- 6. Rujukan lunak audit_log.entitas_id. Nilai lain ('3 sumber', 'export', rentang periode KPI, ...) bukan ID.
    UPDATE audit_log a
       SET entitas_id = s.id::text
      FROM sumber_data s
     WHERE a.entitas = 'sumber_data' AND a.entitas_id = s.kode;
    UPDATE audit_log a
       SET entitas_id = l.id::text
      FROM pengguna_id_lama l
     WHERE a.entitas IN ('langganan_push', 'preferensi_notifikasi') AND a.entitas_id = l.id_lama;

    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_no_truncate;

    -- 7. Pasang FK dengan ON DELETE eksplisit, lalu indeks FK yang belum terlayani.
    FOR r IN SELECT * FROM g5_fk LOOP
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %I(%I) ON DELETE %s',
                       r.tabel, r.nama, r.kolom, r.induk, r.kolom_induk, r.hapus_baru);
        IF r.indeks IS NOT NULL THEN
            EXECUTE format('CREATE INDEX %I ON %I (%I)', r.indeks, r.tabel, r.kolom);
        END IF;
    END LOOP;

    -- 8. Indeks kueri dan time-series.
    -- audit_log: rentang waktu (BRIN, log bertambah menurut waktu); dua indeks lama menjadi redundan.
    CREATE INDEX idx_audit_log_waktu_brin ON audit_log USING brin (waktu) WITH (pages_per_range = 32);
    DROP INDEX idx_audit_log_entitas;
    DROP INDEX idx_audit_log_waktu;
    -- data_mentah(diambil_pada) dan harga_eksternal(tanggal): time-series bronze dan silver yang bertambah terus.
    CREATE INDEX idx_data_mentah_diambil_brin ON data_mentah USING brin (diambil_pada) WITH (pages_per_range = 32);
    CREATE INDEX idx_harga_eksternal_tanggal_brin ON harga_eksternal USING brin (tanggal) WITH (pages_per_range = 32);
    -- cuaca_harian: rentang tanggal lintas kabupaten (rule engine EWS, dashboard operasional).
    CREATE INDEX idx_cuaca_harian_tanggal ON cuaca_harian (tanggal);

    COMMENT ON COLUMN sumber_data.kode IS 'Kode tampilan sumber (ID lama, mis. SRC-BI): alias rute dan kunci konfigurasi konektor/CLI ingestion.';
    COMMENT ON COLUMN cuaca_harian.kode IS 'ID lama (CUA-<kabupaten>-<yyyymmdd>-<sumber>); NULL untuk baris yang dibuat sesudah migrasi 000115.';
    COMMENT ON COLUMN ruang_fiskal.kode IS 'ID lama (mis. DEMO-RF-202609-1); kunci idempoten seeder demo.';
    COMMENT ON COLUMN siklus_ekonomi.kode IS 'ID lama (mis. SE-11-DEMO); kunci idempoten seeder demo.';
    COMMENT ON COLUMN audit_log.kode IS 'ID lama (UUID teks atau demo-audit-NNNN); alias rute /v1/audit/:id dan kunci idempoten seeder demo. NULL untuk baris baru.';
END
$mig$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $mig$
DECLARE
    r record;
BEGIN
    CREATE TEMP TABLE g5_fk (
        tabel       text NOT NULL,
        kolom       text NOT NULL,
        nama        text NOT NULL,
        induk       text NOT NULL,
        kolom_induk text NOT NULL,
        hapus_lama  text NOT NULL,
        indeks      text
    ) ON COMMIT DROP;

    INSERT INTO g5_fk (tabel, kolom, nama, induk, kolom_induk, hapus_lama, indeks) VALUES
        ('eksekusi_sumber', 'sumber_id',      'eksekusi_sumber_sumber_id_fkey',     'sumber_data',     'id',   'NO ACTION', NULL),
        ('data_mentah',     'sumber_id',      'data_mentah_sumber_id_fkey',         'sumber_data',     'id',   'NO ACTION', NULL),
        ('data_mentah',     'eksekusi_id',    'data_mentah_eksekusi_id_fkey',       'eksekusi_sumber', 'id',   'NO ACTION', NULL),
        ('pemetaan_sumber', 'sumber_id',      'pemetaan_sumber_sumber_id_fkey',     'sumber_data',     'id',   'NO ACTION', NULL),
        ('harga_eksternal', 'sumber_id',      'harga_eksternal_sumber_id_fkey',     'sumber_data',     'id',   'NO ACTION', NULL),
        ('harga_eksternal', 'komoditas_kode', 'harga_eksternal_komoditas_kode_fkey','komoditas',      'kode', 'NO ACTION', NULL),
        ('harga_eksternal', 'kabupaten_kode', 'harga_eksternal_kabupaten_kode_fkey','kabupaten',      'kode', 'NO ACTION', 'idx_harga_eksternal_kabupaten_kode'),
        ('harga_eksternal', 'data_mentah_id', 'harga_eksternal_data_mentah_id_fkey','data_mentah',    'id',   'NO ACTION', 'idx_harga_eksternal_data_mentah_id'),
        ('harga_eksternal', 'eksekusi_id',    'harga_eksternal_eksekusi_id_fkey',   'eksekusi_sumber', 'id',   'NO ACTION', 'idx_harga_eksternal_eksekusi_id'),
        ('cuaca_harian',    'kabupaten_kode', 'cuaca_harian_kabupaten_kode_fkey',   'kabupaten',       'kode', 'NO ACTION', NULL),
        ('cuaca_harian',    'eksekusi_id',    'cuaca_harian_eksekusi_id_fkey',      'eksekusi_sumber', 'id',   'NO ACTION', 'idx_cuaca_harian_eksekusi_id'),
        ('cuaca_harian',    'data_mentah_id', 'cuaca_harian_data_mentah_id_fkey',   'data_mentah',     'id',   'NO ACTION', 'idx_cuaca_harian_data_mentah_id'),
        ('andil_inflasi',   'komoditas_kode', 'andil_inflasi_komoditas_kode_fkey',  'komoditas',       'kode', 'NO ACTION', 'idx_andil_inflasi_komoditas_kode'),
        ('neraca_pangan',   'komoditas_kode', 'neraca_pangan_komoditas_kode_fkey',  'komoditas',       'kode', 'NO ACTION', NULL),
        ('siklus_ekonomi',  'wilayah_kode',   'siklus_ekonomi_wilayah_kode_fkey',   'provinsi',        'kode', 'NO ACTION', NULL),
        ('kpi_nilai',       'wilayah_kode',   'kpi_nilai_wilayah_kode_fkey',        'kabupaten',       'kode', 'NO ACTION', 'idx_kpi_nilai_wilayah_kode'),
        ('kpi_target',      'wilayah_kode',   'kpi_target_wilayah_kode_fkey',       'kabupaten',       'kode', 'NO ACTION', 'idx_kpi_target_wilayah_kode');

    -- Pemulih kode sumber untuk kolom anak: USING pada ALTER COLUMN TYPE tidak boleh berisi subkueri, jadi lewat
    -- fungsi sementara. Dibuang di akhir blok.
    CREATE FUNCTION pg_temp.g5_kode_sumber(u uuid) RETURNS varchar(255)
    LANGUAGE sql STABLE
    AS $fn$
        SELECT COALESCE((SELECT kode FROM sumber_data WHERE id = u), u::text)::varchar(255)
    $fn$;

    -- 1. Rujukan lunak kembali ke ID lama (sebelum kode dan pemetaan pengguna dibuang).
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_no_truncate;
    UPDATE audit_log a
       SET entitas_id = s.kode
      FROM sumber_data s
     WHERE a.entitas = 'sumber_data' AND a.entitas_id = s.id::text;
    UPDATE audit_log a
       SET entitas_id = l.id_lama
      FROM pengguna_id_lama l
     WHERE a.entitas IN ('langganan_push', 'preferensi_notifikasi') AND a.entitas_id = l.id::text;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_no_truncate;

    -- 2. Lepas FK dan indeks baru.
    FOR r IN SELECT * FROM g5_fk LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.nama);
        IF r.indeks IS NOT NULL THEN
            EXECUTE format('DROP INDEX %I', r.indeks);
        END IF;
    END LOOP;
    DROP INDEX idx_cuaca_harian_tanggal;
    DROP INDEX idx_harga_eksternal_tanggal_brin;
    DROP INDEX idx_data_mentah_diambil_brin;
    DROP INDEX idx_audit_log_waktu_brin;
    CREATE INDEX idx_audit_log_waktu ON audit_log (waktu);
    CREATE INDEX idx_audit_log_entitas ON audit_log (entitas, entitas_id);

    -- 3. BIGINT identity -> VARCHAR. USING membaca kode (ID lama); baris tanpa kode memakai id::text.
    FOR r IN
        SELECT unnest(ARRAY['cuaca_harian', 'ruang_fiskal', 'siklus_ekonomi', 'audit_log']) AS tabel
    LOOP
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id DROP IDENTITY', r.tabel);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text)', r.tabel);
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, 'uq_' || r.tabel || '_kode');
        EXECUTE format('ALTER TABLE %I DROP COLUMN kode', r.tabel);
    END LOOP;

    -- 4. BIGINT id + UNIQUE alami -> PK komposit alami. PK baru dipasang sesudah sumber_id kembali ke varchar.
    ALTER TABLE harga_eksternal DROP CONSTRAINT harga_eksternal_pkey;
    ALTER TABLE harga_eksternal DROP CONSTRAINT uq_harga_eksternal_alami;
    ALTER TABLE harga_eksternal DROP COLUMN id;

    ALTER TABLE andil_inflasi DROP CONSTRAINT andil_inflasi_pkey;
    ALTER TABLE andil_inflasi DROP CONSTRAINT uq_andil_inflasi_alami;
    ALTER TABLE andil_inflasi DROP COLUMN id;
    ALTER TABLE andil_inflasi ADD CONSTRAINT andil_inflasi_pkey PRIMARY KEY (periode, wilayah_kode, komoditas_kode);

    ALTER TABLE inflasi_bulanan DROP CONSTRAINT inflasi_bulanan_pkey;
    ALTER TABLE inflasi_bulanan DROP CONSTRAINT uq_inflasi_bulanan_alami;
    ALTER TABLE inflasi_bulanan DROP COLUMN id;
    ALTER TABLE inflasi_bulanan ADD CONSTRAINT inflasi_bulanan_pkey PRIMARY KEY (wilayah_kode, periode);

    ALTER TABLE neraca_pangan DROP CONSTRAINT neraca_pangan_pkey;
    ALTER TABLE neraca_pangan DROP CONSTRAINT uq_neraca_pangan_alami;
    ALTER TABLE neraca_pangan DROP COLUMN id;
    ALTER TABLE neraca_pangan ADD CONSTRAINT neraca_pangan_pkey PRIMARY KEY (komoditas_kode, wilayah_kode, periode);

    ALTER TABLE target_kpi DROP CONSTRAINT target_kpi_pkey;
    ALTER TABLE target_kpi DROP CONSTRAINT uq_target_kpi_alami;
    ALTER TABLE target_kpi DROP COLUMN id;
    ALTER TABLE target_kpi ADD CONSTRAINT target_kpi_pkey PRIMARY KEY (indikator, periode_mulai);

    -- 5. sumber_data: UUID -> VARCHAR. Anak lebih dulu (fungsi membaca kode dari sumber_data), lalu induknya.
    ALTER TABLE eksekusi_sumber ALTER COLUMN sumber_id TYPE varchar(255) USING pg_temp.g5_kode_sumber(sumber_id);
    ALTER TABLE data_mentah     ALTER COLUMN sumber_id TYPE varchar(255) USING pg_temp.g5_kode_sumber(sumber_id);
    ALTER TABLE pemetaan_sumber ALTER COLUMN sumber_id TYPE varchar(255) USING pg_temp.g5_kode_sumber(sumber_id);
    ALTER TABLE harga_eksternal ALTER COLUMN sumber_id TYPE varchar(255) USING pg_temp.g5_kode_sumber(sumber_id);
    ALTER TABLE harga_eksternal
        ADD CONSTRAINT harga_eksternal_pkey PRIMARY KEY (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe);
    ALTER TABLE sumber_data ALTER COLUMN id DROP DEFAULT;
    ALTER TABLE sumber_data ALTER COLUMN id TYPE varchar(255) USING kode;
    ALTER TABLE sumber_data DROP CONSTRAINT uq_sumber_data_kode;
    ALTER TABLE sumber_data DROP COLUMN kode;

    -- 6. FK dengan aturan semula (NO ACTION = bawaan PostgreSQL).
    FOR r IN SELECT * FROM g5_fk LOOP
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %I(%I) ON DELETE %s',
                       r.tabel, r.nama, r.kolom, r.induk, r.kolom_induk, r.hapus_lama);
    END LOOP;

    DROP FUNCTION pg_temp.g5_kode_sumber(uuid);
END
$mig$;
-- +goose StatementEnd
