-- Slice 14-B (docs/todos/14-intervensi.md): riwayat kartu, optimistic locking, eskalasi.
-- - intervensi.versi: naik tiap perubahan kartu; pemindahan dengan versi usang ditolak (409);
-- - intervensi.dieskalasi_pada/oleh: kartu yang dieskalasi ke Dewan Pengarah;
-- - intervensi_riwayat: setiap pembuatan, pemindahan tahap, penugasan, dan eskalasi
--   (oleh NULL = sistem).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE intervensi
    ADD COLUMN IF NOT EXISTS versi INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS dieskalasi_pada TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS dieskalasi_oleh_id VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_intervensi_versi') THEN
        ALTER TABLE intervensi ADD CONSTRAINT chk_intervensi_versi CHECK (versi > 0);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS intervensi_riwayat (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    intervensi_id VARCHAR(255) NOT NULL REFERENCES intervensi(id) ON DELETE CASCADE,
    aksi VARCHAR(30) NOT NULL,
    dari VARCHAR(100),
    ke VARCHAR(100),
    oleh VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    waktu TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alasan TEXT NOT NULL DEFAULT '',
    CONSTRAINT chk_intervensi_riwayat_aksi CHECK (aksi IN ('BUAT', 'PINDAH_TAHAP', 'TUGASKAN', 'ESKALASI'))
);
CREATE INDEX IF NOT EXISTS idx_intervensi_riwayat_kartu ON intervensi_riwayat (intervensi_id, waktu, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS intervensi_riwayat;
ALTER TABLE intervensi DROP CONSTRAINT IF EXISTS chk_intervensi_versi;
ALTER TABLE intervensi
    DROP COLUMN IF EXISTS dieskalasi_oleh_id,
    DROP COLUMN IF EXISTS dieskalasi_pada,
    DROP COLUMN IF EXISTS versi;
-- +goose StatementEnd
