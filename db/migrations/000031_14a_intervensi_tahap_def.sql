-- Slice 14-A (docs/todos/14-intervensi.md): kanban intervensi dari basis data.
-- - tahap_intervensi_def: satu sumber definisi tahap per siklus (urutan, izin masuk, SLA);
-- - intervensi.tahap divalidasi terhadapnya lewat FK komposit (siklus, tahap);
-- - intervensi: deskripsi dan selesai_pada;
-- - intervensi_id_seq: nomor id kartu baru (INT-1000, INT-1001, ...), di atas id seed demo.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tahap_intervensi_def (
    siklus siklus NOT NULL,
    kode_tahap VARCHAR(100) NOT NULL,
    urutan INT NOT NULL,
    label VARCHAR(255) NOT NULL,
    petunjuk TEXT NOT NULL DEFAULT '',
    izin_dibutuhkan VARCHAR(100) NOT NULL DEFAULT 'intervensi.kelola',
    sla_hari INT,
    PRIMARY KEY (siklus, kode_tahap),
    UNIQUE (siklus, urutan),
    CONSTRAINT chk_tahap_intervensi_urutan CHECK (urutan > 0),
    CONSTRAINT chk_tahap_intervensi_sla CHECK (sla_hari IS NULL OR sla_hari > 0)
);

ALTER TABLE intervensi
    ADD COLUMN IF NOT EXISTS deskripsi TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS selesai_pada TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_intervensi_tahap') THEN
        ALTER TABLE intervensi ADD CONSTRAINT fk_intervensi_tahap
            FOREIGN KEY (siklus, tahap) REFERENCES tahap_intervensi_def (siklus, kode_tahap);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_intervensi_ews_alert ON intervensi (ews_alert_id);

CREATE SEQUENCE IF NOT EXISTS intervensi_id_seq START 1000;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SEQUENCE IF EXISTS intervensi_id_seq;
DROP INDEX IF EXISTS idx_intervensi_ews_alert;
ALTER TABLE intervensi DROP CONSTRAINT IF EXISTS fk_intervensi_tahap;
ALTER TABLE intervensi
    DROP COLUMN IF EXISTS selesai_pada,
    DROP COLUMN IF EXISTS deskripsi;
DROP TABLE IF EXISTS tahap_intervensi_def;
-- +goose StatementEnd
