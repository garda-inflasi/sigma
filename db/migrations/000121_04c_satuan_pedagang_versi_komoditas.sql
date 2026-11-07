-- Slice 04-C (docs/todos/04-master-data.md): pelengkap master data.
--
-- * satuan (PK KODE): katalog satuan komoditas. komoditas.satuan menjadi FK RESTRICT ke satuan(kode); satuan yang
--   sudah dipakai komoditas diisi dari data yang ada (pada basis data kosong tidak ada yang diisi), nama resminya
--   ditulis seeder master.reference. entri_harga.satuan tetap salinan saat entri dicatat (bukti historis, tanpa FK).
-- * pedagang (PK BIGINT): daftar pedagang per pasar, UNIQUE (pasar_id, kode). entri_harga.kode_pedagang tetap teks
--   bebas (entri lapangan luring tidak boleh gagal karena pedagang belum terdaftar); daftar ini dicocokkan menurut
--   (pasar_id, kode_pedagang). Pasangan yang sudah muncul di entri_harga diisi dari data yang ada.
-- * kualitas: diputuskan TETAP enum (PREMIUM, MEDIUM, ASALAN). Nilainya sedikit, stabil, dipakai validasi harga dan
--   model, dan tidak punya atribut lain; tabel baru diperlukan hanya bila kualitas berbeda per komoditas.
-- * komoditas_versi (PK BIGINT, append-only): salinan definisi komoditas setiap kali isinya berubah, dengan
--   berlaku_dari. Harga lama dibaca terhadap definisi yang berlaku saat itu: versi terakhir dengan
--   berlaku_dari <= waktu harga (atau versi paling awal bila harga lebih tua dari riwayat). Ditulis pemicu pada
--   komoditas (INSERT, atau UPDATE yang mengubah isi/versi; perubahan status saja tidak). perubahan_id mengacu usulan
--   perubahan_master yang menghasilkannya (GUC transaksi garda.perubahan_master_id). Peran garda_aplikasi hanya
--   SELECT/INSERT (03-C); daftar tabel append-only di garda_terapkan_hak_aplikasi ikut diperbarui.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS satuan (
    kode varchar(50) PRIMARY KEY,
    nama varchar(100) NOT NULL,
    CONSTRAINT chk_satuan_kode CHECK (length(btrim(kode)) > 0)
);
COMMENT ON TABLE satuan IS 'Katalog satuan komoditas (kg, liter, ...); diisi seeder master.reference.';
INSERT INTO satuan (kode, nama)
SELECT DISTINCT k.satuan, k.satuan FROM komoditas k
ON CONFLICT (kode) DO NOTHING;
ALTER TABLE komoditas
    ADD CONSTRAINT komoditas_satuan_fkey FOREIGN KEY (satuan) REFERENCES satuan(kode) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS idx_komoditas_satuan ON komoditas (satuan);

CREATE TABLE IF NOT EXISTS pedagang (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pasar_id    varchar(255) NOT NULL REFERENCES pasar(id) ON DELETE RESTRICT,
    kode        varchar(100) NOT NULL,
    nama        varchar(255),
    aktif       boolean NOT NULL DEFAULT true,
    dibuat_pada timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_pedagang_pasar_kode UNIQUE (pasar_id, kode),
    CONSTRAINT chk_pedagang_kode CHECK (length(btrim(kode)) > 0)
);
COMMENT ON TABLE pedagang IS 'Pedagang per pasar; entri_harga.kode_pedagang dicocokkan menurut (pasar_id, kode).';
INSERT INTO pedagang (pasar_id, kode)
SELECT DISTINCT e.pasar_id, e.kode_pedagang FROM entri_harga e
 WHERE e.kode_pedagang IS NOT NULL AND btrim(e.kode_pedagang) <> ''
ON CONFLICT (pasar_id, kode) DO NOTHING;

COMMENT ON TYPE kualitas IS 'Kualitas harga (PREMIUM, MEDIUM, ASALAN): tetap enum, lihat migrasi 000121.';

CREATE TABLE IF NOT EXISTS komoditas_versi (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    komoditas_kode  varchar(255) NOT NULL REFERENCES komoditas(kode) ON DELETE CASCADE,
    versi           integer NOT NULL,
    nama            varchar(255) NOT NULL,
    kategori        varchar(100) NOT NULL,
    satuan          varchar(50) NOT NULL,
    harga_acuan     integer NOT NULL,
    shelf_life_hari integer NOT NULL,
    pilot           boolean NOT NULL,
    berlaku_dari    timestamptz NOT NULL DEFAULT now(),
    perubahan_id    bigint REFERENCES perubahan_master(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_komoditas_versi_kode_berlaku ON komoditas_versi (komoditas_kode, berlaku_dari DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_komoditas_versi_perubahan_id ON komoditas_versi (perubahan_id);
COMMENT ON TABLE komoditas_versi IS 'Riwayat definisi komoditas (append-only); versi yang berlaku pada waktu t = berlaku_dari terakhir <= t.';

-- Satu versi untuk setiap komoditas yang sudah ada (tidak ada apa pun pada basis data kosong).
INSERT INTO komoditas_versi (komoditas_kode, versi, nama, kategori, satuan, harga_acuan, shelf_life_hari, pilot)
SELECT k.kode, k.versi, k.nama, k.kategori, k.satuan, k.harga_acuan, k.shelf_life_hari, k.pilot
  FROM komoditas k ORDER BY k.kode;

CREATE OR REPLACE FUNCTION komoditas_catat_versi() RETURNS trigger AS $$
DECLARE
    v_perubahan text := nullif(current_setting('garda.perubahan_master_id', true), '');
    v_perubahan_id bigint;
BEGIN
    IF TG_OP = 'UPDATE'
       AND NEW.nama IS NOT DISTINCT FROM OLD.nama AND NEW.kategori IS NOT DISTINCT FROM OLD.kategori
       AND NEW.satuan IS NOT DISTINCT FROM OLD.satuan AND NEW.harga_acuan IS NOT DISTINCT FROM OLD.harga_acuan
       AND NEW.shelf_life_hari IS NOT DISTINCT FROM OLD.shelf_life_hari AND NEW.pilot IS NOT DISTINCT FROM OLD.pilot
       AND NEW.versi IS NOT DISTINCT FROM OLD.versi THEN
        RETURN NULL; -- perubahan status saja bukan versi definisi baru
    END IF;
    -- GUC yang rusak atau merujuk usulan yang tidak ada diabaikan, tidak membatalkan transaksi.
    IF v_perubahan ~ '^[0-9]{1,18}$' THEN
        SELECT p.id INTO v_perubahan_id FROM perubahan_master p WHERE p.id = v_perubahan::bigint;
    END IF;
    INSERT INTO komoditas_versi (komoditas_kode, versi, nama, kategori, satuan, harga_acuan, shelf_life_hari, pilot,
                                 perubahan_id)
    VALUES (NEW.kode, NEW.versi, NEW.nama, NEW.kategori, NEW.satuan, NEW.harga_acuan, NEW.shelf_life_hari, NEW.pilot,
            v_perubahan_id);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_komoditas_versi ON komoditas;
CREATE TRIGGER trg_komoditas_versi
    AFTER INSERT OR UPDATE ON komoditas
    FOR EACH ROW EXECUTE FUNCTION komoditas_catat_versi();

CREATE OR REPLACE FUNCTION komoditas_versi_tolak_perubahan() RETURNS trigger AS $$
BEGIN
    -- Satu-satunya penghapusan yang sah: ON DELETE CASCADE saat komoditasnya dihapus (dijalankan pemicu FK).
    IF TG_OP = 'DELETE' AND pg_trigger_depth() > 1 THEN
        RETURN OLD;
    END IF;
    -- Pelepasan perubahan_id oleh FK ON DELETE SET NULL juga sah.
    IF TG_OP = 'UPDATE' AND pg_trigger_depth() > 1 AND NEW.perubahan_id IS NULL AND OLD.perubahan_id IS NOT NULL
       AND (to_jsonb(NEW) - 'perubahan_id') = (to_jsonb(OLD) - 'perubahan_id') THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'komoditas_versi bersifat append-only: % ditolak', TG_OP USING ERRCODE = 'restrict_violation';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_komoditas_versi_append_only ON komoditas_versi;
CREATE TRIGGER trg_komoditas_versi_append_only
    BEFORE UPDATE OR DELETE ON komoditas_versi
    FOR EACH ROW EXECUTE FUNCTION komoditas_versi_tolak_perubahan();
DROP TRIGGER IF EXISTS trg_komoditas_versi_no_truncate ON komoditas_versi;
CREATE TRIGGER trg_komoditas_versi_no_truncate
    BEFORE TRUNCATE ON komoditas_versi
    FOR EACH STATEMENT EXECUTE FUNCTION komoditas_versi_tolak_perubahan();

-- Hak minimum (03-C): komoditas_versi masuk daftar tabel append-only.
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
                             'entri_harga_versi', 'komoditas_versi')
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

DO $hak$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        PERFORM garda_terapkan_hak_aplikasi();
    END IF;
END;
$hak$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
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

DO $hak$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        REVOKE ALL ON komoditas_versi, pedagang, satuan FROM garda_aplikasi;
    END IF;
END;
$hak$;
DROP TRIGGER IF EXISTS trg_komoditas_versi ON komoditas;
DROP FUNCTION IF EXISTS komoditas_catat_versi();
DROP TABLE IF EXISTS komoditas_versi;
DROP FUNCTION IF EXISTS komoditas_versi_tolak_perubahan();
COMMENT ON TYPE kualitas IS NULL;
DROP TABLE IF EXISTS pedagang;
DROP INDEX IF EXISTS idx_komoditas_satuan;
ALTER TABLE komoditas DROP CONSTRAINT IF EXISTS komoditas_satuan_fkey;
DROP TABLE IF EXISTS satuan;
-- +goose StatementEnd
