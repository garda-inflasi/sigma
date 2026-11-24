-- Lokasi spasial sentra pertanian dan perkampungan untuk pemetaan distribusi, jarak ke pasar & cold storage,
-- serta input langsung oleh Superadmin (ADMIN_SISTEM).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS lokasi_spasial (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kode               VARCHAR(32) NOT NULL UNIQUE,
    jenis              VARCHAR(24) NOT NULL CHECK (jenis IN ('PERTANIAN', 'PERKAMPUNGAN')),
    nama               VARCHAR(160) NOT NULL,
    kabupaten_kode     VARCHAR(10) NOT NULL REFERENCES kabupaten(kode),
    kecamatan_kode     VARCHAR(16) NULL REFERENCES kecamatan(kode),
    komoditas_utama    VARCHAR(120) NOT NULL DEFAULT '',
    luas_ha            NUMERIC(10, 2) NULL,
    estimasi_panen_ton NUMERIC(10, 2) NULL,
    populasi           INTEGER NULL,
    lat                DOUBLE PRECISION NOT NULL,
    lng                DOUBLE PRECISION NOT NULL,
    aktif              BOOLEAN NOT NULL DEFAULT TRUE,
    versi              INTEGER NOT NULL DEFAULT 1,
    dibuat_pada        TIMESTAMPTZ NOT NULL DEFAULT now(),
    diperbarui_pada    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_lokasi_spasial_jenis_kab ON lokasi_spasial (jenis, kabupaten_kode);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS lokasi_spasial;
-- +goose StatementEnd
