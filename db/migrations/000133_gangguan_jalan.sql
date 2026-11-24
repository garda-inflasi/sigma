-- Tabel operasional gangguan jalan (bencana, banjir, longsor, jembatan putus, jalan rusak)
-- untuk pengalihan rute distribusi jalan raya otomatis di modul spasial.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS gangguan_jalan (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kode              VARCHAR(36) NOT NULL UNIQUE,
    judul             VARCHAR(180) NOT NULL,
    jenis             VARCHAR(32) NOT NULL CHECK (jenis IN ('LONGSOR', 'BANJIR', 'JEMBATAN_PUTUS', 'JALAN_RUSAK', 'PENUTUPAN')),
    tingkat           VARCHAR(24) NOT NULL DEFAULT 'TUTUP_TOTAL' CHECK (tingkat IN ('TUTUP_TOTAL', 'TERHAMBAT')),
    kabupaten_kode    VARCHAR(10) NULL REFERENCES kabupaten(kode),
    ruas_jalan        VARCHAR(160) NOT NULL DEFAULT '',
    lat               DOUBLE PRECISION NOT NULL CHECK (lat BETWEEN -90 AND 90),
    lng               DOUBLE PRECISION NOT NULL CHECK (lng BETWEEN -180 AND 180),
    radius_m          INTEGER NOT NULL DEFAULT 350 CHECK (radius_m BETWEEN 50 AND 10000),
    keterangan        TEXT NOT NULL DEFAULT '',
    aktif             BOOLEAN NOT NULL DEFAULT TRUE,
    mulai             TIMESTAMPTZ NOT NULL DEFAULT now(),
    perkiraan_selesai TIMESTAMPTZ NULL,
    dibuat_pada       TIMESTAMPTZ NOT NULL DEFAULT now(),
    diperbarui_pada   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_gangguan_jalan_aktif ON gangguan_jalan (aktif, tingkat, mulai DESC);

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON public.gangguan_jalan TO garda_aplikasi;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS gangguan_jalan;
-- +goose StatementEnd
