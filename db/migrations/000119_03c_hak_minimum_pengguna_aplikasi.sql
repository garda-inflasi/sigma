-- Slice 03-C (docs/todos/03-audit.md) dan 19 §7 (docs/todos/19-migration-seeder.md): pengguna DB aplikasi berhak
-- minimum.
--
-- * Peran grup garda_aplikasi (NOLOGIN) memegang hak aplikasi: baca/tulis data (SELECT, INSERT, UPDATE, DELETE),
--   sequence, dan fungsi pemeliharaan tertentu. Tanpa DDL, tanpa TRUNCATE, dan bukan pemilik objek apa pun.
-- * Tabel jejak append-only hanya SELECT dan INSERT: audit_log, audit_log_verifikasi, audit_log_arsip,
--   audit_log_arsip_segmen, dan entri_harga_versi (lineage, 000118). REVOKE UPDATE, DELETE, TRUNCATE di sini menjadi
--   lapis kedua di samping pemicu append-only (pemicu tetap melindungi bila aplikasi masih memakai pemilik skema).
-- * Partisi audit_log tidak bisa diakses langsung (hanya lewat induk), dan goose_db_version tidak bisa disentuh.
-- * REFRESH MATERIALIZED VIEW mensyaratkan pemilik (PG16), jadi penyegaran dashboard lewat garda_segarkan_mv
--   (SECURITY DEFINER, hanya materialized view di skema public).
-- * Hak bawaan (ALTER DEFAULT PRIVILEGES) memberi garda_aplikasi DML pada tabel dan sequence yang dibuat migrasi
--   berikutnya. Tabel append-only baru wajib mencabut UPDATE/DELETE-nya sendiri di migrasinya.
--
-- Login aplikasi dibuat cmd/migrate (DB_APP_USER, DB_APP_PASSWORD) sebagai anggota garda_aplikasi; api dan worker
-- lalu terhubung dengannya, sedangkan migrate dan seed tetap memakai pemilik skema (DB_USER). Peran adalah objek
-- tingkat klaster: bila pengguna migrasi tidak boleh membuat peran (CREATEROLE), migrasi ini hanya membuat fungsi dan
-- mencatat NOTICE; buat garda_aplikasi secara manual lalu jalankan SELECT garda_terapkan_hak_aplikasi().
-- Down mencabut hak di basis data ini, tetapi tidak membuang peran (bisa dipakai basis data lain di klaster).

-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION garda_segarkan_mv(p_nama text, p_serentak boolean DEFAULT true) RETURNS void
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = pg_catalog, public
AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_matviews WHERE schemaname = 'public' AND matviewname = p_nama) THEN
        RAISE EXCEPTION 'materialized view % tidak dikenal', p_nama USING ERRCODE = 'invalid_parameter_value';
    END IF;
    IF p_serentak THEN
        EXECUTE format('REFRESH MATERIALIZED VIEW CONCURRENTLY public.%I', p_nama);
    ELSE
        EXECUTE format('REFRESH MATERIALIZED VIEW public.%I', p_nama);
    END IF;
END;
$$;
REVOKE ALL ON FUNCTION garda_segarkan_mv(text, boolean) FROM PUBLIC;

-- Menerapkan (ulang) hak garda_aplikasi di basis data ini. Idempoten; dijalankan pemilik skema.
CREATE OR REPLACE FUNCTION garda_terapkan_hak_aplikasi() RETURNS void
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
AS $$
DECLARE
    r record;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        RAISE EXCEPTION 'peran garda_aplikasi belum ada';
    END IF;

    EXECUTE format('GRANT CONNECT ON DATABASE %I TO garda_aplikasi', current_database());
    GRANT USAGE ON SCHEMA public TO garda_aplikasi;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO garda_aplikasi;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO garda_aplikasi;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO garda_aplikasi;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO garda_aplikasi;

    -- Jejak append-only: hanya baca dan tambah.
    FOR r IN
        SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p') AND NOT c.relispartition
           AND c.relname IN ('audit_log', 'audit_log_verifikasi', 'audit_log_arsip', 'audit_log_arsip_segmen',
                             'entri_harga_versi')
    LOOP
        EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON public.%I FROM garda_aplikasi', r.relname);
    END LOOP;

    -- Partisi audit_log hanya lewat induk; tabel migrasi tidak untuk aplikasi.
    FOR r IN SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
              WHERE i.inhparent = 'public.audit_log'::regclass LOOP
        EXECUTE format('REVOKE ALL ON public.%I FROM garda_aplikasi', r.relname);
    END LOOP;
    IF to_regclass('public.goose_db_version') IS NOT NULL THEN
        REVOKE ALL ON public.goose_db_version FROM garda_aplikasi;
        IF to_regclass('public.goose_db_version_id_seq') IS NOT NULL THEN
            REVOKE ALL ON SEQUENCE public.goose_db_version_id_seq FROM garda_aplikasi;
        END IF;
    END IF;

    GRANT EXECUTE ON FUNCTION garda_segarkan_mv(text, boolean) TO garda_aplikasi;
    GRANT EXECUTE ON FUNCTION audit_log_siapkan_partisi(timestamptz, integer) TO garda_aplikasi;
    GRANT EXECUTE ON FUNCTION audit_log_lepas_partisi(text, integer, bigint, bytea, text, text) TO garda_aplikasi;
END;
$$;
REVOKE ALL ON FUNCTION garda_terapkan_hak_aplikasi() FROM PUBLIC;

DO $peran$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        IF NOT (SELECT rolsuper OR rolcreaterole FROM pg_roles WHERE rolname = current_user) THEN
            RAISE NOTICE 'peran garda_aplikasi belum ada dan % tidak boleh membuat peran; buat secara manual lalu '
                'jalankan SELECT garda_terapkan_hak_aplikasi()', current_user;
            RETURN;
        END IF;
        BEGIN
            CREATE ROLE garda_aplikasi NOLOGIN;
        EXCEPTION WHEN duplicate_object OR unique_violation THEN
            NULL; -- dibuat bersamaan oleh migrasi basis data lain di klaster yang sama
        END;
    END IF;
    PERFORM garda_terapkan_hak_aplikasi();
END;
$peran$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $peran$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM garda_aplikasi;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM garda_aplikasi;
        REVOKE ALL ON ALL TABLES IN SCHEMA public FROM garda_aplikasi;
        REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM garda_aplikasi;
        REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM garda_aplikasi;
        REVOKE ALL ON SCHEMA public FROM garda_aplikasi;
        EXECUTE format('REVOKE ALL ON DATABASE %I FROM garda_aplikasi', current_database());
    END IF;
END;
$peran$;
DROP FUNCTION IF EXISTS garda_terapkan_hak_aplikasi();
DROP FUNCTION IF EXISTS garda_segarkan_mv(text, boolean);
-- +goose StatementEnd
