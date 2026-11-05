-- Slice 09-A (docs/todos/09-dashboard-eksekutif.md): ruang_fiskal menandai asal
-- angkanya. Baris yang sudah ada (dari impor sebelum kolom ini) dianggap DJPB;
-- sesudahnya sumber wajib dinyatakan eksplisit saat menulis.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE ruang_fiskal ADD COLUMN IF NOT EXISTS sumber VARCHAR(10) NOT NULL DEFAULT 'DJPB';
ALTER TABLE ruang_fiskal ALTER COLUMN sumber DROP DEFAULT;
ALTER TABLE ruang_fiskal ADD COLUMN IF NOT EXISTS diimpor_pada TIMESTAMPTZ NOT NULL DEFAULT NOW();

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_ruang_fiskal_sumber') THEN
        ALTER TABLE ruang_fiskal
            ADD CONSTRAINT chk_ruang_fiskal_sumber CHECK (sumber IN ('BPS', 'BI', 'DJPB', 'BAPANAS', 'DEMO'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_ruang_fiskal_angka') THEN
        ALTER TABLE ruang_fiskal
            ADD CONSTRAINT chk_ruang_fiskal_angka CHECK (pagu > 0 AND realisasi >= 0 AND realisasi <= pagu);
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ruang_fiskal DROP CONSTRAINT IF EXISTS chk_ruang_fiskal_angka;
ALTER TABLE ruang_fiskal DROP CONSTRAINT IF EXISTS chk_ruang_fiskal_sumber;
ALTER TABLE ruang_fiskal DROP COLUMN IF EXISTS diimpor_pada;
ALTER TABLE ruang_fiskal DROP COLUMN IF EXISTS sumber;
-- +goose StatementEnd
