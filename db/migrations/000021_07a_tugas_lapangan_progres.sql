-- Slice 07-A (docs/todos/07-lapangan-pwa.md): tugas_lapangan mencatat pembuat,
-- waktu dibuat dan selesai, serta progres pengisian; target dan progres dijaga
-- tetap wajar di basis data.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE tugas_lapangan
    ADD COLUMN IF NOT EXISTS dibuat_pada    TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS dibuat_oleh_id VARCHAR(255) REFERENCES pengguna(id),
    ADD COLUMN IF NOT EXISTS selesai_pada   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS jumlah_selesai INT NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_tugas_lapangan_target') THEN
        ALTER TABLE tugas_lapangan ADD CONSTRAINT chk_tugas_lapangan_target CHECK (target > 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_tugas_lapangan_jumlah_selesai') THEN
        ALTER TABLE tugas_lapangan ADD CONSTRAINT chk_tugas_lapangan_jumlah_selesai CHECK (jumlah_selesai >= 0);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_tugas_lapangan_pasar_tenggat ON tugas_lapangan(pasar_id, tenggat);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tugas_lapangan_pasar_tenggat;
ALTER TABLE tugas_lapangan DROP CONSTRAINT IF EXISTS chk_tugas_lapangan_jumlah_selesai;
ALTER TABLE tugas_lapangan DROP CONSTRAINT IF EXISTS chk_tugas_lapangan_target;
ALTER TABLE tugas_lapangan
    DROP COLUMN IF EXISTS jumlah_selesai,
    DROP COLUMN IF EXISTS selesai_pada,
    DROP COLUMN IF EXISTS dibuat_oleh_id,
    DROP COLUMN IF EXISTS dibuat_pada;
-- +goose StatementEnd
