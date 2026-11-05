-- Slice 11-C (docs/todos/11-prediksi-xai.md): MLOps dan simulasi.
--   * model_registry.dipromosikan_oleh/dipromosikan_pada: siapa dan kapan sebuah
--     model terakhir dipromosikan ke PRODUKSI (persetujuan manusia, izin model.kelola).
--   * model_promosi: riwayat tiap promosi dan pengarsipan, termasuk model yang
--     digantikan, sehingga keputusan dapat ditelusuri dan dikembalikan.
--   * skenario_simulasi: simulasi what-if yang dihitung server beserta parameter,
--     hasil (termasuk koefisien yang dipakai dan sumbernya), dan pelakunya.
-- Tidak ada data di sini.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE model_registry ADD COLUMN IF NOT EXISTS dipromosikan_oleh VARCHAR(255) REFERENCES pengguna(id);
ALTER TABLE model_registry ADD COLUMN IF NOT EXISTS dipromosikan_pada TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS model_promosi (
    id VARCHAR(255) PRIMARY KEY,
    model_id VARCHAR(255) NOT NULL REFERENCES model_registry(id),
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    aksi VARCHAR(10) NOT NULL,
    status_lama VARCHAR(10) NOT NULL,
    status_baru VARCHAR(10) NOT NULL,
    -- Model PRODUKSI yang diarsipkan oleh promosi ini (bila ada).
    model_digantikan_id VARCHAR(255) REFERENCES model_registry(id),
    oleh VARCHAR(255) NOT NULL REFERENCES pengguna(id),
    waktu TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_model_promosi_aksi CHECK (aksi IN ('PROMOSI', 'ARSIP')),
    CONSTRAINT chk_model_promosi_status CHECK (
        status_lama IN ('PRODUKSI', 'KANDIDAT', 'ARSIP') AND status_baru IN ('PRODUKSI', 'KANDIDAT', 'ARSIP')
    )
);
CREATE INDEX IF NOT EXISTS idx_model_promosi_model ON model_promosi (model_id, waktu DESC);
CREATE INDEX IF NOT EXISTS idx_model_promosi_komoditas ON model_promosi (komoditas_kode, waktu DESC);

CREATE TABLE IF NOT EXISTS skenario_simulasi (
    id VARCHAR(255) PRIMARY KEY,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    horizon_hari INT NOT NULL,
    parameter JSONB NOT NULL,
    hasil JSONB NOT NULL,
    dibuat_oleh VARCHAR(255) NOT NULL REFERENCES pengguna(id),
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_skenario_simulasi_horizon CHECK (horizon_hari > 0)
);
CREATE INDEX IF NOT EXISTS idx_skenario_simulasi_pembuat ON skenario_simulasi (dibuat_oleh, dibuat_pada DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS skenario_simulasi;
DROP TABLE IF EXISTS model_promosi;
ALTER TABLE model_registry DROP COLUMN IF EXISTS dipromosikan_pada;
ALTER TABLE model_registry DROP COLUMN IF EXISTS dipromosikan_oleh;
-- +goose StatementEnd
