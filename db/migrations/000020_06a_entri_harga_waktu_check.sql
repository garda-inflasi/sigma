-- Slice 06-A (docs/todos/06-harga-validasi.md): entri_harga mendapat jejak waktu
-- baris, harga wajib positif, dan indeks untuk antrean validasi serta urutan terbaru.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE entri_harga
    ADD COLUMN IF NOT EXISTS dibuat_pada   TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS diupdate_pada TIMESTAMPTZ NOT NULL DEFAULT now();

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_entri_harga_harga') THEN
        ALTER TABLE entri_harga ADD CONSTRAINT chk_entri_harga_harga CHECK (harga > 0);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_entri_harga_antrean ON entri_harga(dicatat_pada DESC) WHERE status = 'DIAJUKAN';
CREATE INDEX IF NOT EXISTS idx_entri_harga_dicatat ON entri_harga(dicatat_pada DESC, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_entri_harga_dicatat;
DROP INDEX IF EXISTS idx_entri_harga_antrean;
ALTER TABLE entri_harga DROP CONSTRAINT IF EXISTS chk_entri_harga_harga;
ALTER TABLE entri_harga
    DROP COLUMN IF EXISTS diupdate_pada,
    DROP COLUMN IF EXISTS dibuat_pada;
-- +goose StatementEnd
