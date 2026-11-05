-- Slice 12-A (docs/todos/12-peta-spasial.md): peta surplus-defisit dari basis data.
-- - view wilayah: kabupaten/kota sebagai titik peta (tanpa PostGIS; geometri menyusul di 12-B);
-- - neraca_wilayah: surplus/defisit dan deviasi harga per komoditas, wilayah, dan periode;
-- - rute_distribusi: usulan rute dari sentra surplus ke wilayah tertekan;
-- - kandidat_cold_storage: lokasi cold storage yang diusulkan beserta komponen skornya.

-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE VIEW wilayah AS
    SELECT kode, nama, lat, lng, LOWER(jenis) AS level
      FROM kabupaten;

CREATE TABLE IF NOT EXISTS neraca_wilayah (
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    periode DATE NOT NULL,
    surplus_defisit_ton NUMERIC(12, 1) NOT NULL,
    delta_harga_pct NUMERIC(6, 2) NOT NULL,
    sumber VARCHAR(20) NOT NULL DEFAULT 'DEMO',
    PRIMARY KEY (komoditas_kode, kabupaten_kode, periode)
);
CREATE INDEX IF NOT EXISTS idx_neraca_wilayah_periode ON neraca_wilayah (periode DESC, kabupaten_kode);

CREATE TABLE IF NOT EXISTS rute_distribusi (
    id VARCHAR(255) PRIMARY KEY,
    asal_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    tujuan_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    volume_ton NUMERIC(10, 1) NOT NULL,
    jarak_km INT NOT NULL,
    jam_tempuh NUMERIC(5, 1) NOT NULL,
    biaya BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'USULAN',
    sumber VARCHAR(20) NOT NULL DEFAULT 'DEMO',
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_rute_distribusi_beda CHECK (asal_kode <> tujuan_kode),
    CONSTRAINT chk_rute_distribusi_nilai CHECK (volume_ton > 0 AND jarak_km >= 0 AND jam_tempuh >= 0 AND biaya >= 0)
);
CREATE INDEX IF NOT EXISTS idx_rute_distribusi_tujuan ON rute_distribusi (tujuan_kode);

CREATE TABLE IF NOT EXISTS kandidat_cold_storage (
    id VARCHAR(255) PRIMARY KEY,
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    lokasi VARCHAR(255) NOT NULL,
    skor INT NOT NULL,
    komponen_skor JSONB NOT NULL,
    alasan TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'USULAN_BARU',
    sumber VARCHAR(20) NOT NULL DEFAULT 'DEMO',
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_kandidat_cold_storage_skor CHECK (skor BETWEEN 0 AND 100)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS kandidat_cold_storage;
DROP TABLE IF EXISTS rute_distribusi;
DROP TABLE IF EXISTS neraca_wilayah;
DROP VIEW IF EXISTS wilayah;
-- +goose StatementEnd
