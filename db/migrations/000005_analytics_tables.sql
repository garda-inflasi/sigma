-- Migration 000005: Analytics — PostgreSQL enhancements for MongoDB-backed system
-- Core analytics (commodity_daily_buckets, field_entries, region_master, fiscal_space_buckets)
-- are stored in MongoDB. This migration only enhances PostgreSQL tables that overlap.
-- +goose Up
-- +goose StatementBegin

-- ── Enhance model_registry with additional columns ────────────────────
ALTER TABLE model_registry
    ADD COLUMN IF NOT EXISTS algorithm VARCHAR(255),
    ADD COLUMN IF NOT EXISTS metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS deployed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS feature_importance JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE model_registry
    DROP COLUMN IF EXISTS algorithm,
    DROP COLUMN IF EXISTS metrics,
    DROP COLUMN IF EXISTS deployed_at,
    DROP COLUMN IF EXISTS feature_importance;
-- +goose StatementEnd
