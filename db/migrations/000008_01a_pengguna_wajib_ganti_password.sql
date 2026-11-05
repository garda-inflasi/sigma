-- Slice 01-A (docs/todos/01-auth.md): akun yang kata sandinya ditetapkan pihak lain
-- wajib menggantinya saat masuk pertama, dan email unik tanpa peka huruf besar-kecil.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE pengguna
    ADD COLUMN IF NOT EXISTS wajib_ganti_password BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS password_diubah_pada TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS uq_pengguna_email_lower ON pengguna (lower(email));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uq_pengguna_email_lower;

ALTER TABLE pengguna
    DROP COLUMN IF EXISTS password_diubah_pada,
    DROP COLUMN IF EXISTS wajib_ganti_password;
-- +goose StatementEnd
