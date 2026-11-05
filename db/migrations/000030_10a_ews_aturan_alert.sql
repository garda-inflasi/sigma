-- Slice 10-A (docs/todos/10-ews.md): alert dan ambang EWS dari basis data.
-- - ews_alert.aturan_id menautkan alert ke aturan ambang yang memicunya;
-- - aturan_ews: invarian ambang waspada < siaga < kritis dan keterangan kalibrasi;
-- - indeks untuk daftar alert (status/severity, terbaru dahulu).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE ews_alert
    ADD COLUMN IF NOT EXISTS aturan_id VARCHAR(255) REFERENCES aturan_ews(id) ON DELETE SET NULL;

ALTER TABLE aturan_ews
    ADD COLUMN IF NOT EXISTS keterangan TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_aturan_ews_ambang') THEN
        ALTER TABLE aturan_ews ADD CONSTRAINT chk_aturan_ews_ambang
            CHECK (ambang_waspada < ambang_siaga AND ambang_siaga < ambang_kritis);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_ews_alert_dibuat_pada ON ews_alert (dibuat_pada DESC, id);
CREATE INDEX IF NOT EXISTS idx_ews_alert_aturan ON ews_alert (aturan_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_ews_alert_aturan;
DROP INDEX IF EXISTS idx_ews_alert_dibuat_pada;

ALTER TABLE aturan_ews DROP CONSTRAINT IF EXISTS chk_aturan_ews_ambang;
ALTER TABLE aturan_ews DROP COLUMN IF EXISTS keterangan;
ALTER TABLE ews_alert DROP COLUMN IF EXISTS aturan_id;
-- +goose StatementEnd
