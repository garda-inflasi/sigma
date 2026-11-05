-- Slice 01-C (docs/todos/01-auth.md): kode cadangan MFA disimpan sebagai hash, bukan teks polos.
-- - mfa_kode_cadangan (user_id, kode_hash, dipakai_pada): satu baris per kode, hash SHA-256 dari
--   "<user_id>:<kode huruf besar tanpa spasi/tanda hubung>"; kode dipakai sekali (dipakai_pada);
-- - kode polos di pengguna.mfa_backup_codes dipindahkan sebagai hash lalu kolomnya dibuang;
-- - mfa_langkah_terakhir mencegah satu kode TOTP dipakai dua kali.
-- Enkripsi mfa_secret (butuh kunci AES) dijalankan sebagai migrasi Go versi 64 (internal/platform/migrate).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS mfa_kode_cadangan (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    kode_hash VARCHAR(64) NOT NULL,
    dipakai_pada TIMESTAMPTZ,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, kode_hash)
);

INSERT INTO mfa_kode_cadangan (user_id, kode_hash)
SELECT p.id, encode(sha256(convert_to(p.id || ':' || upper(regexp_replace(kode, '[\s-]', '', 'g')), 'UTF8')), 'hex')
  FROM pengguna p
 CROSS JOIN LATERAL jsonb_array_elements_text(p.mfa_backup_codes) AS kode
 WHERE jsonb_typeof(p.mfa_backup_codes) = 'array'
ON CONFLICT (user_id, kode_hash) DO NOTHING;

ALTER TABLE pengguna ADD COLUMN IF NOT EXISTS mfa_langkah_terakhir BIGINT NOT NULL DEFAULT 0;
ALTER TABLE pengguna DROP COLUMN IF EXISTS mfa_backup_codes;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Hash tidak dapat dikembalikan menjadi kode polos: setelah Down, pengguna MFA membuat kode cadangan baru.
ALTER TABLE pengguna ADD COLUMN IF NOT EXISTS mfa_backup_codes JSONB DEFAULT '[]'::jsonb;
ALTER TABLE pengguna DROP COLUMN IF EXISTS mfa_langkah_terakhir;
DROP TABLE IF EXISTS mfa_kode_cadangan;
-- +goose StatementEnd
