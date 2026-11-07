-- Slice 03-C (docs/todos/03-audit.md): hash berantai pada audit_log (tamper-evident).
--
-- Setiap baris menyimpan hash = SHA-256 atas hash baris sebelumnya ditambah isi baris itu sendiri, berurutan menurut
-- id (BIGINT identity). Mengubah, menyisipkan, atau menghapus satu baris di tengah memutus rantai sesudahnya, dan
-- job verifikasi (worker audit_verifikasi) menghitung ulang seluruh rantai di Go untuk mendeteksinya.
--
-- Format masukan hash (versi garda-audit-v1; padanan Go: internal/audit/domain/rantai.go, keduanya wajib sama):
--   sha256( 'garda-audit-v1' || B(hash_sebelumnya) || B(id) || B(waktu_us) || B(kode) || B(aktor_nama)
--           || B(aktor_email) || B(aktor_peran) || B(aksi) || B(entitas) || B(entitas_id) || B(hasil) || B(ip)
--           || B(user_agent) || B(correlation_id) || B(sebelum) || B(sesudah) || B(metadata) )
--   B(NULL) = 0x00; B(x) = 0x01 || panjang x dalam bita (4 bita big-endian) || x. Teks dalam UTF-8; id dan waktu_us
--   (epoch dalam mikrodetik) sebagai angka desimal; kolom JSONB dalam bentuk teks keluaran Postgres (jsonb::text).
--   aktor_id sengaja tidak ikut: FK ON DELETE SET NULL boleh mengosongkannya saat akun dihapus (satu-satunya perubahan
--   sah, lihat 000011). Identitas pelaku tetap terlindungi lewat snapshot aktor_nama/aktor_email/aktor_peran.
--
-- Urutan rantai = urutan id = urutan commit. Pemicu BEFORE INSERT FOR EACH STATEMENT mengambil advisory lock
-- transaksi SEBELUM nilai identity dibagikan (bawaan kolom dievaluasi sesudah pemicu tingkat statement), jadi penulis
-- audit berjalan satu per satu dan id yang lebih besar selalu dirantai ke baris yang sudah di-commit. Pemicu
-- FOR EACH ROW lalu membaca hash baris dengan id terbesar (baris yang dilewati ON CONFLICT tidak pernah masuk, jadi
-- tidak memutus rantai). Penulis audit aplikasi memakai autocommit, sehingga kunci hanya ditahan sesaat. Penulisan di
-- dalam transaksi REPEATABLE READ/SERIALIZABLE ditolak: snapshot-nya bisa tidak melihat baris terakhir.
--
-- Baris lama diberi hash berurutan menurut id di migrasi ini (pemicu append-only dimatikan hanya selama pengisian,
-- di transaksi yang sama). Migrasi berikutnya TIDAK BOLEH mengubah isi baris audit_log: rantai sesudahnya akan putus
-- dan terlaporkan sebagai perusakan.
--
-- Pengisian berjalan baris per baris (O(n) UPDATE); jalankan di jendela pemeliharaan untuk audit_log besar.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE audit_log
    ADD COLUMN IF NOT EXISTS hash_sebelumnya bytea,
    ADD COLUMN IF NOT EXISTS hash bytea;

-- B(x): satu bidang masukan hash yang tidak ambigu (penanda NULL + panjang + isi).
CREATE OR REPLACE FUNCTION audit_log_bidang(p bytea) RETURNS bytea
    LANGUAGE sql IMMUTABLE PARALLEL SAFE AS
$$
    SELECT CASE WHEN p IS NULL THEN '\x00'::bytea ELSE '\x01'::bytea || int4send(length(p)) || p END
$$;

CREATE OR REPLACE FUNCTION audit_log_hitung_hash(
    p_hash_sebelumnya bytea, p_id bigint, p_waktu timestamptz, p_kode text,
    p_aktor_nama text, p_aktor_email text, p_aktor_peran text,
    p_aksi text, p_entitas text, p_entitas_id text, p_hasil text,
    p_ip text, p_user_agent text, p_correlation_id text,
    p_sebelum jsonb, p_sesudah jsonb, p_metadata jsonb
) RETURNS bytea
    LANGUAGE sql STABLE PARALLEL SAFE AS
$$
    SELECT sha256(
        convert_to('garda-audit-v1', 'UTF8')
        || audit_log_bidang(p_hash_sebelumnya)
        || audit_log_bidang(convert_to(p_id::text, 'UTF8'))
        || audit_log_bidang(convert_to(((extract(epoch FROM p_waktu) * 1000000)::bigint)::text, 'UTF8'))
        || audit_log_bidang(convert_to(p_kode, 'UTF8'))
        || audit_log_bidang(convert_to(p_aktor_nama, 'UTF8'))
        || audit_log_bidang(convert_to(p_aktor_email, 'UTF8'))
        || audit_log_bidang(convert_to(p_aktor_peran, 'UTF8'))
        || audit_log_bidang(convert_to(p_aksi, 'UTF8'))
        || audit_log_bidang(convert_to(p_entitas, 'UTF8'))
        || audit_log_bidang(convert_to(p_entitas_id, 'UTF8'))
        || audit_log_bidang(convert_to(p_hasil, 'UTF8'))
        || audit_log_bidang(convert_to(p_ip, 'UTF8'))
        || audit_log_bidang(convert_to(p_user_agent, 'UTF8'))
        || audit_log_bidang(convert_to(p_correlation_id, 'UTF8'))
        || audit_log_bidang(convert_to(p_sebelum::text, 'UTF8'))
        || audit_log_bidang(convert_to(p_sesudah::text, 'UTF8'))
        || audit_log_bidang(convert_to(p_metadata::text, 'UTF8'))
    )
$$;

-- Pemicu statement: serialisasi penulis audit sebelum id dibagikan.
CREATE OR REPLACE FUNCTION audit_log_kunci_rantai() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'audit_log hanya boleh ditulis dalam transaksi READ COMMITTED (sekarang %)',
            current_setting('transaction_isolation')
            USING ERRCODE = 'object_not_in_prerequisite_state';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('garda.audit_log.rantai', 0));
    RETURN NULL;
END;
$$;

-- Pemicu baris: sambungkan ke baris dengan id terbesar yang terlihat (sudah di-commit, atau ditulis lebih dulu oleh
-- transaksi/statement ini).
CREATE OR REPLACE FUNCTION audit_log_rantai_hash() RETURNS trigger
    LANGUAGE plpgsql AS
$$
DECLARE
    v_sebelumnya bytea;
BEGIN
    SELECT a.hash INTO v_sebelumnya FROM audit_log a ORDER BY a.id DESC LIMIT 1;
    NEW.hash_sebelumnya := v_sebelumnya;
    NEW.hash := audit_log_hitung_hash(
        v_sebelumnya, NEW.id, NEW.waktu, NEW.kode,
        NEW.aktor_nama, NEW.aktor_email, NEW.aktor_peran,
        NEW.aksi, NEW.entitas, NEW.entitas_id, NEW.hasil,
        NEW.ip, NEW.user_agent, NEW.correlation_id,
        NEW.sebelum, NEW.sesudah, NEW.metadata);
    RETURN NEW;
END;
$$;

-- Pengisian baris lama, berurutan menurut id.
ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
DO $isi$
DECLARE
    r audit_log%ROWTYPE;
    v_sebelumnya bytea;
    v_hash bytea;
BEGIN
    FOR r IN SELECT * FROM audit_log ORDER BY id LOOP
        v_hash := audit_log_hitung_hash(
            v_sebelumnya, r.id, r.waktu, r.kode,
            r.aktor_nama, r.aktor_email, r.aktor_peran,
            r.aksi, r.entitas, r.entitas_id, r.hasil,
            r.ip, r.user_agent, r.correlation_id,
            r.sebelum, r.sesudah, r.metadata);
        UPDATE audit_log SET hash_sebelumnya = v_sebelumnya, hash = v_hash WHERE id = r.id;
        v_sebelumnya := v_hash;
    END LOOP;
END;
$isi$;
ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

ALTER TABLE audit_log
    ALTER COLUMN hash SET NOT NULL,
    ADD CONSTRAINT chk_audit_log_hash CHECK (octet_length(hash) = 32),
    ADD CONSTRAINT chk_audit_log_hash_sebelumnya CHECK (hash_sebelumnya IS NULL OR octet_length(hash_sebelumnya) = 32);

DROP TRIGGER IF EXISTS trg_audit_log_rantai_kunci ON audit_log;
CREATE TRIGGER trg_audit_log_rantai_kunci
    BEFORE INSERT ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_kunci_rantai();

DROP TRIGGER IF EXISTS trg_audit_log_rantai ON audit_log;
CREATE TRIGGER trg_audit_log_rantai
    BEFORE INSERT ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_rantai_hash();

COMMENT ON COLUMN audit_log.hash IS 'SHA-256 garda-audit-v1 atas hash_sebelumnya + isi baris (lihat migrasi 000116); diisi pemicu.';
COMMENT ON COLUMN audit_log.hash_sebelumnya IS 'hash baris dengan id sebelumnya; NULL hanya untuk baris pertama rantai.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_audit_log_rantai ON audit_log;
DROP TRIGGER IF EXISTS trg_audit_log_rantai_kunci ON audit_log;
DROP FUNCTION IF EXISTS audit_log_rantai_hash();
DROP FUNCTION IF EXISTS audit_log_kunci_rantai();
DROP FUNCTION IF EXISTS audit_log_hitung_hash(bytea, bigint, timestamptz, text, text, text, text, text, text, text,
    text, text, text, text, jsonb, jsonb, jsonb);
DROP FUNCTION IF EXISTS audit_log_bidang(bytea);

ALTER TABLE audit_log
    DROP CONSTRAINT IF EXISTS chk_audit_log_hash_sebelumnya,
    DROP CONSTRAINT IF EXISTS chk_audit_log_hash,
    DROP COLUMN IF EXISTS hash,
    DROP COLUMN IF EXISTS hash_sebelumnya;
-- +goose StatementEnd
