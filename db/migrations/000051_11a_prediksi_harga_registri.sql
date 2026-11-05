-- Slice 11-A (docs/todos/11-prediksi-xai.md): prediksi_harga menjadi sumber
-- proyeksi (menggantikan proyeksi_xai yang tak pernah ada) dan terikat ke
-- model_registry.
--   * harga_dasar: harga konsumen saat proyeksi dibuat, pembagi untuk persen
--     perubahan dan rentang. Baris lama (bila ada) memakai nilai.
--   * sumber: 'MODEL' bagi keluaran pipeline ML, 'DEMO' bagi data contoh seeder.
--   * invarian: bawah <= nilai <= atas, horizon positif, satu proyeksi per
--     (komoditas, kabupaten, tanggal target, horizon, versi), versi harus ada
--     di registri, dan satu model PRODUKSI per komoditas.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE model_registry ADD COLUMN IF NOT EXISTS sumber VARCHAR(10) NOT NULL DEFAULT 'MODEL';

ALTER TABLE prediksi_harga ADD COLUMN IF NOT EXISTS harga_dasar INT;
UPDATE prediksi_harga SET harga_dasar = nilai WHERE harga_dasar IS NULL;
ALTER TABLE prediksi_harga ALTER COLUMN harga_dasar SET NOT NULL;
ALTER TABLE prediksi_harga ADD COLUMN IF NOT EXISTS sumber VARCHAR(10) NOT NULL DEFAULT 'MODEL';

DO $$
DECLARE
    aturan CONSTANT text[][] := ARRAY[
        ['model_registry',  'chk_model_registry_sumber',       'sumber IN (''MODEL'', ''DEMO'')'],
        ['model_registry',  'chk_model_registry_mape',         'mape >= 0'],
        ['prediksi_harga',  'chk_prediksi_harga_sumber',       'sumber IN (''MODEL'', ''DEMO'')'],
        ['prediksi_harga',  'chk_prediksi_harga_horizon',      'horizon_hari > 0'],
        ['prediksi_harga',  'chk_prediksi_harga_harga_dasar',  'harga_dasar > 0'],
        ['prediksi_harga',  'chk_prediksi_harga_interval',     'bawah <= nilai AND nilai <= atas']
    ];
    i int;
BEGIN
    FOR i IN 1 .. array_length(aturan, 1) LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = aturan[i][2]) THEN
            EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I CHECK (%s)', aturan[i][1], aturan[i][2], aturan[i][3]);
        END IF;
    END LOOP;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_prediksi_harga_proyeksi') THEN
        ALTER TABLE prediksi_harga ADD CONSTRAINT uq_prediksi_harga_proyeksi
            UNIQUE (komoditas_kode, kabupaten_kode, tanggal_target, horizon_hari, model_versi);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_prediksi_harga_model') THEN
        ALTER TABLE prediksi_harga ADD CONSTRAINT fk_prediksi_harga_model
            FOREIGN KEY (komoditas_kode, model_versi) REFERENCES model_registry (komoditas_kode, versi);
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_model_registry_produksi
    ON model_registry (komoditas_kode) WHERE status = 'PRODUKSI';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uq_model_registry_produksi;
ALTER TABLE prediksi_harga DROP CONSTRAINT IF EXISTS fk_prediksi_harga_model;
ALTER TABLE prediksi_harga DROP CONSTRAINT IF EXISTS uq_prediksi_harga_proyeksi;
ALTER TABLE prediksi_harga DROP CONSTRAINT IF EXISTS chk_prediksi_harga_interval;
ALTER TABLE prediksi_harga DROP CONSTRAINT IF EXISTS chk_prediksi_harga_harga_dasar;
ALTER TABLE prediksi_harga DROP CONSTRAINT IF EXISTS chk_prediksi_harga_horizon;
ALTER TABLE prediksi_harga DROP CONSTRAINT IF EXISTS chk_prediksi_harga_sumber;
ALTER TABLE prediksi_harga DROP COLUMN IF EXISTS sumber;
ALTER TABLE prediksi_harga DROP COLUMN IF EXISTS harga_dasar;
ALTER TABLE model_registry DROP CONSTRAINT IF EXISTS chk_model_registry_mape;
ALTER TABLE model_registry DROP CONSTRAINT IF EXISTS chk_model_registry_sumber;
ALTER TABLE model_registry DROP COLUMN IF EXISTS sumber;
-- +goose StatementEnd
