-- Slice 06-B (docs/todos/06-harga-validasi.md): state machine entri harga.
-- - alasan, pelaku, dan waktu penolakan disimpan di entri;
-- - riwayat_status_entri mencatat setiap perpindahan status;
-- - pemisahan tugas dan alasan penolakan wajib dijaga di basis data. Kedua CHECK
--   NOT VALID: baris lama tidak diperiksa ulang, tetapi setiap tulis baru wajib patuh.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE entri_harga
    ADD COLUMN IF NOT EXISTS alasan_penolakan TEXT,
    ADD COLUMN IF NOT EXISTS ditolak_oleh VARCHAR(255) REFERENCES pengguna(id),
    ADD COLUMN IF NOT EXISTS ditolak_pada TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_entri_harga_pemisahan_tugas') THEN
        ALTER TABLE entri_harga ADD CONSTRAINT chk_entri_harga_pemisahan_tugas
            CHECK (divalidasi_oleh IS NULL OR divalidasi_oleh IS DISTINCT FROM petugas_id) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_entri_harga_alasan_penolakan') THEN
        ALTER TABLE entri_harga ADD CONSTRAINT chk_entri_harga_alasan_penolakan
            CHECK (status <> 'DITOLAK' OR alasan_penolakan IS NOT NULL) NOT VALID;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS riwayat_status_entri (
    id BIGSERIAL PRIMARY KEY,
    entri_id VARCHAR(255) NOT NULL REFERENCES entri_harga(id) ON DELETE CASCADE,
    dari status_entri,
    ke status_entri NOT NULL,
    oleh VARCHAR(255) REFERENCES pengguna(id),
    waktu TIMESTAMPTZ NOT NULL DEFAULT now(),
    alasan TEXT
);
CREATE INDEX IF NOT EXISTS idx_riwayat_status_entri ON riwayat_status_entri(entri_id, waktu, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS riwayat_status_entri;
ALTER TABLE entri_harga DROP CONSTRAINT IF EXISTS chk_entri_harga_alasan_penolakan;
ALTER TABLE entri_harga DROP CONSTRAINT IF EXISTS chk_entri_harga_pemisahan_tugas;
ALTER TABLE entri_harga
    DROP COLUMN IF EXISTS ditolak_pada,
    DROP COLUMN IF EXISTS ditolak_oleh,
    DROP COLUMN IF EXISTS alasan_penolakan;
-- +goose StatementEnd
