-- Slice 01-G (docs/todos/01-auth.md, "SSO Google"): masuk lewat OpenID Connect dipetakan ke akun
-- yang SUDAH ada (tanpa pendaftaran mandiri).
--
-- pengguna.sso_provider + sso_subject mengaitkan akun dengan satu identitas penyedia SSO. Pasangannya
-- unik (satu identitas Google hanya boleh menaut ke satu akun) dan keduanya terisi atau keduanya kosong.
-- password_hash sudah boleh NULL sejak 000001, jadi akun yang hanya masuk lewat SSO tidak perlu kata sandi.
--
-- sso_login_state menyimpan langkah awal alur Authorization Code + PKCE selama <= 10 menit: state dan
-- binding hanya disimpan sebagai hash SHA-256, nonce dan code_verifier tidak pernah dikirim ke browser.
-- Baris dipakai sekali (DELETE ... RETURNING) dan dibersihkan oleh API saat alur baru dimulai.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE pengguna
    ADD COLUMN IF NOT EXISTS sso_provider VARCHAR(32),
    ADD COLUMN IF NOT EXISTS sso_subject VARCHAR(255),
    ADD COLUMN IF NOT EXISTS sso_terhubung_pada TIMESTAMPTZ;

ALTER TABLE pengguna
    ADD CONSTRAINT pengguna_sso_pasangan_chk
    CHECK ((sso_provider IS NULL) = (sso_subject IS NULL));

CREATE UNIQUE INDEX IF NOT EXISTS ux_pengguna_sso
    ON pengguna (sso_provider, sso_subject);

CREATE TABLE IF NOT EXISTS sso_login_state (
    state_hash VARCHAR(64) PRIMARY KEY,
    binding_hash VARCHAR(64) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    nonce VARCHAR(128) NOT NULL,
    code_verifier VARCHAR(128) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sso_login_state_expires ON sso_login_state (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sso_login_state;
DROP INDEX IF EXISTS ux_pengguna_sso;
ALTER TABLE pengguna DROP CONSTRAINT IF EXISTS pengguna_sso_pasangan_chk;
ALTER TABLE pengguna
    DROP COLUMN IF EXISTS sso_terhubung_pada,
    DROP COLUMN IF EXISTS sso_subject,
    DROP COLUMN IF EXISTS sso_provider;
-- +goose StatementEnd
