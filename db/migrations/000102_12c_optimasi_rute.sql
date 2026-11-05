-- 12-C: optimasi rute distribusi.
-- - jarak_antar_wilayah: cache matriks jarak/waktu tempuh. 'ESTIMASI' = jarak garis lurus
--   x faktor jalan (tanpa layanan routing eksternal); 'OSRM' disediakan untuk impor jarak jalan
--   nyata di kemudian hari dan tidak ditimpa oleh estimasi.
-- - optimasi_rute: satu baris per rencana yang disimpan beserta parameter dan ringkasannya.
-- - rute_distribusi.optimasi_id: rute usulan hasil optimasi menunjuk ke rencananya.
-- Tanpa data demo.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS jarak_antar_wilayah (
    asal_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    tujuan_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    jarak_km NUMERIC(8, 1) NOT NULL,
    jam_tempuh NUMERIC(5, 1) NOT NULL,
    sumber VARCHAR(20) NOT NULL DEFAULT 'ESTIMASI',
    diperbarui_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asal_kode, tujuan_kode),
    CONSTRAINT chk_jarak_antar_wilayah_beda CHECK (asal_kode <> tujuan_kode),
    CONSTRAINT chk_jarak_antar_wilayah_nilai CHECK (jarak_km >= 0 AND jam_tempuh >= 0),
    CONSTRAINT chk_jarak_antar_wilayah_sumber CHECK (sumber IN ('ESTIMASI', 'OSRM'))
);

CREATE TABLE IF NOT EXISTS optimasi_rute (
    id VARCHAR(64) PRIMARY KEY,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    periode DATE NOT NULL,
    parameter JSONB NOT NULL,
    total_volume_ton NUMERIC(12, 1) NOT NULL,
    total_biaya BIGINT NOT NULL,
    belum_terpenuhi_ton NUMERIC(12, 1) NOT NULL,
    dibuat_oleh VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_optimasi_rute_nilai CHECK (total_volume_ton >= 0 AND total_biaya >= 0 AND belum_terpenuhi_ton >= 0)
);
CREATE INDEX IF NOT EXISTS idx_optimasi_rute_dibuat ON optimasi_rute (dibuat_pada DESC);

ALTER TABLE rute_distribusi
    ADD COLUMN IF NOT EXISTS optimasi_id VARCHAR(64) REFERENCES optimasi_rute(id);
CREATE INDEX IF NOT EXISTS idx_rute_distribusi_optimasi ON rute_distribusi (optimasi_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rute hasil optimasi tetap ada sebagai usulan (sumber OPTIMASI) tanpa tautan ke rencananya.
DROP INDEX IF EXISTS idx_rute_distribusi_optimasi;
ALTER TABLE rute_distribusi DROP COLUMN IF EXISTS optimasi_id;
DROP TABLE IF EXISTS optimasi_rute;
DROP TABLE IF EXISTS jarak_antar_wilayah;
-- +goose StatementEnd
