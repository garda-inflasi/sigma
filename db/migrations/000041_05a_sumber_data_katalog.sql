-- Slice 05-A (docs/todos/05-sumber-data.md): katalog sumber data dari database.
-- Entity Go mengikuti skema ini (keputusan D1), jadi hanya yang benar-benar
-- dibutuhkan katalog yang ditambahkan.
--
-- * url_dasar: alamat dasar layanan sumber (bahan konfigurasi klien di 05-B).
-- * status baru berawal BELUM_AKTIF (nilai enum dari 000040): belum pernah dijalankan.
-- * keberhasilan_7h NULL = belum ada eksekusi untuk dihitung; bawaan 100 menyesatkan.
-- * jadwal_cron dibaca dalam zona waktu WIB (Asia/Jakarta).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE sumber_data ADD COLUMN IF NOT EXISTS url_dasar TEXT;
ALTER TABLE sumber_data ALTER COLUMN status SET DEFAULT 'BELUM_AKTIF';
ALTER TABLE sumber_data ALTER COLUMN keberhasilan_7h DROP NOT NULL;
ALTER TABLE sumber_data ALTER COLUMN keberhasilan_7h DROP DEFAULT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_sumber_data_keberhasilan') THEN
        ALTER TABLE sumber_data
            ADD CONSTRAINT chk_sumber_data_keberhasilan CHECK (keberhasilan_7h BETWEEN 0 AND 100);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_sumber_data_latensi') THEN
        ALTER TABLE sumber_data
            ADD CONSTRAINT chk_sumber_data_latensi CHECK (latensi_ms >= 0);
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sumber_data DROP CONSTRAINT IF EXISTS chk_sumber_data_latensi;
ALTER TABLE sumber_data DROP CONSTRAINT IF EXISTS chk_sumber_data_keberhasilan;
UPDATE sumber_data SET keberhasilan_7h = 100 WHERE keberhasilan_7h IS NULL;
ALTER TABLE sumber_data ALTER COLUMN keberhasilan_7h SET DEFAULT 100;
ALTER TABLE sumber_data ALTER COLUMN keberhasilan_7h SET NOT NULL;
ALTER TABLE sumber_data ALTER COLUMN status SET DEFAULT 'SEHAT';
ALTER TABLE sumber_data DROP COLUMN IF EXISTS url_dasar;
-- +goose StatementEnd
