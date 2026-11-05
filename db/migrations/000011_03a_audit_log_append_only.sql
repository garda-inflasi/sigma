-- Slice 03-A (docs/todos/03-audit.md): jejak audit yang tak bisa diubah.
-- - kolom konteks: snapshot aktor, user agent, correlation id, sebelum/sesudah;
-- - FK aktor menjadi ON DELETE SET NULL (snapshot nama/email tetap menjelaskan pelakunya);
-- - trigger append-only: UPDATE/DELETE/TRUNCATE ditolak, kecuali pelepasan aktor_id
--   oleh aksi FK itu sendiri (pg_trigger_depth() > 1).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE audit_log
    ADD COLUMN IF NOT EXISTS aktor_nama VARCHAR(255),
    ADD COLUMN IF NOT EXISTS aktor_email VARCHAR(255),
    ADD COLUMN IF NOT EXISTS aktor_peran VARCHAR(50),
    ADD COLUMN IF NOT EXISTS user_agent TEXT,
    ADD COLUMN IF NOT EXISTS correlation_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS sebelum JSONB,
    ADD COLUMN IF NOT EXISTS sesudah JSONB;

ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_aktor_id_fkey;
ALTER TABLE audit_log
    ADD CONSTRAINT audit_log_aktor_id_fkey
    FOREIGN KEY (aktor_id) REFERENCES pengguna(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_audit_log_aktor_waktu ON audit_log (aktor_id, waktu DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_entitas_waktu ON audit_log (entitas, entitas_id, waktu DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_aksi_waktu ON audit_log (aksi, waktu DESC);
-- Mendukung paginasi cursor (waktu, id) tanpa urut ulang.
CREATE INDEX IF NOT EXISTS idx_audit_log_waktu_id ON audit_log (waktu DESC, id DESC);

CREATE OR REPLACE FUNCTION audit_log_tolak_perubahan() RETURNS trigger AS $$
BEGIN
    -- Satu-satunya perubahan yang sah: FK ON DELETE SET NULL saat pengguna dihapus.
    IF TG_OP = 'UPDATE' AND pg_trigger_depth() > 1 AND NEW.aktor_id IS NULL AND OLD.aktor_id IS NOT NULL
       AND (to_jsonb(NEW) - 'aktor_id') = (to_jsonb(OLD) - 'aktor_id') THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'audit_log bersifat append-only: % ditolak', TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_audit_log_append_only ON audit_log;
CREATE TRIGGER trg_audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_tolak_perubahan();

DROP TRIGGER IF EXISTS trg_audit_log_no_truncate ON audit_log;
CREATE TRIGGER trg_audit_log_no_truncate
    BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_tolak_perubahan();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_audit_log_no_truncate ON audit_log;
DROP TRIGGER IF EXISTS trg_audit_log_append_only ON audit_log;
DROP FUNCTION IF EXISTS audit_log_tolak_perubahan();

DROP INDEX IF EXISTS idx_audit_log_waktu_id;
DROP INDEX IF EXISTS idx_audit_log_aksi_waktu;
DROP INDEX IF EXISTS idx_audit_log_entitas_waktu;
DROP INDEX IF EXISTS idx_audit_log_aktor_waktu;

ALTER TABLE audit_log DROP CONSTRAINT IF EXISTS audit_log_aktor_id_fkey;
ALTER TABLE audit_log
    ADD CONSTRAINT audit_log_aktor_id_fkey FOREIGN KEY (aktor_id) REFERENCES pengguna(id);

ALTER TABLE audit_log
    DROP COLUMN IF EXISTS sesudah,
    DROP COLUMN IF EXISTS sebelum,
    DROP COLUMN IF EXISTS correlation_id,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS aktor_peran,
    DROP COLUMN IF EXISTS aktor_email,
    DROP COLUMN IF EXISTS aktor_nama;
-- +goose StatementEnd
