-- Migration 000004: Add MFA/TOTP fields to pengguna table
-- +goose Up
-- +goose StatementBegin

ALTER TABLE pengguna
  ADD COLUMN IF NOT EXISTS mfa_secret TEXT,
  ADD COLUMN IF NOT EXISTS mfa_backup_codes JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS mfa_wajib BOOLEAN NOT NULL DEFAULT false;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin

ALTER TABLE pengguna
  DROP COLUMN IF EXISTS mfa_secret,
  DROP COLUMN IF EXISTS mfa_backup_codes,
  DROP COLUMN IF EXISTS mfa_wajib;

-- +goose StatementEnd
