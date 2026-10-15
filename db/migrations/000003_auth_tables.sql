-- +goose Up
-- +goose StatementBegin

-- JWT Blacklist untuk logout dan revoke token
CREATE TABLE IF NOT EXISTS jwt_blacklist (
    jti VARCHAR(255) PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_jwt_blacklist_expires ON jwt_blacklist(expires_at);

-- Password reset tokens untuk forgot/reset password
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_prt_user ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_prt_token_hash ON password_reset_tokens(token_hash);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS jwt_blacklist;
-- +goose StatementEnd
