-- Slice 01-A: tabel password_history tidak dirujuk kode mana pun. Aturan "tidak boleh
-- memakai ulang kata sandi" memeriksa hash saat ini, jadi tabel yang tak dipakai dibuang.

-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS password_history;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS password_history (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_password_history_user ON password_history(user_id);
-- +goose StatementEnd
