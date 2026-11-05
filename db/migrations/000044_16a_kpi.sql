-- Slice 16-A (docs/todos/16-kpi.md): definisi, target, dan nilai indikator kinerja.
-- Tidak ada data di sini: definisi dan target ditanam seeder kpi.reference, nilai
-- dihitung job dari data yang ada (kpi.demo untuk demo).
--
-- * kpi_definisi.sumber_query menamai penghitung otomatis di kode; NULL = belum
--   bisa dihitung (butuh model atau data yang belum ada), tampil "menunggu data".
-- * kpi_target.wilayah_kode NULL = berlaku untuk semua; baseline sengaja kosong
--   sampai ditetapkan pemangku kepentingan (keputusan bisnis, bukan teknis).
-- * kpi_nilai.wilayah_kode NULL = seluruh provinsi. Revisi nilai = baris dengan
--   versi baru, bukan menimpa; versi terbesar adalah yang berlaku.

-- +goose Up
-- +goose StatementBegin
DO $$ BEGIN
    CREATE TYPE kelompok_kpi AS ENUM ('DATA_SISTEM', 'LOGISTIK', 'DAMPAK_EKONOMI', 'TINDAK_LANJUT');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS kpi_definisi (
    kode VARCHAR(40) PRIMARY KEY,
    kelompok kelompok_kpi NOT NULL,
    nama VARCHAR(255) NOT NULL,
    deskripsi TEXT NOT NULL,
    rumus TEXT NOT NULL,
    satuan VARCHAR(10) NOT NULL DEFAULT '',
    arah_baik VARCHAR(5) NOT NULL,
    frekuensi VARCHAR(10) NOT NULL DEFAULT 'BULANAN',
    sumber_query VARCHAR(40),
    urutan INT NOT NULL DEFAULT 0,
    CONSTRAINT chk_kpi_definisi_arah CHECK (arah_baik IN ('NAIK', 'TURUN')),
    CONSTRAINT chk_kpi_definisi_frekuensi CHECK (frekuensi IN ('HARIAN', 'MINGGUAN', 'BULANAN', 'TRIWULAN'))
);

CREATE TABLE IF NOT EXISTS kpi_target (
    id BIGSERIAL PRIMARY KEY,
    kpi_kode VARCHAR(40) NOT NULL REFERENCES kpi_definisi(kode) ON DELETE CASCADE,
    wilayah_kode VARCHAR(4) REFERENCES kabupaten(kode),
    periode_mulai DATE NOT NULL,
    target NUMERIC(14, 4) NOT NULL,
    ambang_waspada NUMERIC(14, 4),
    baseline NUMERIC(14, 4),
    CONSTRAINT chk_kpi_target_periode CHECK (EXTRACT(DAY FROM periode_mulai) = 1)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_kpi_target ON kpi_target (kpi_kode, (COALESCE(wilayah_kode, '')), periode_mulai);

CREATE TABLE IF NOT EXISTS kpi_nilai (
    id BIGSERIAL PRIMARY KEY,
    kpi_kode VARCHAR(40) NOT NULL REFERENCES kpi_definisi(kode) ON DELETE CASCADE,
    wilayah_kode VARCHAR(4) REFERENCES kabupaten(kode),
    periode DATE NOT NULL,
    versi INT NOT NULL DEFAULT 1,
    nilai NUMERIC(14, 4) NOT NULL,
    dihitung_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_kpi_nilai_periode CHECK (EXTRACT(DAY FROM periode) = 1),
    CONSTRAINT chk_kpi_nilai_versi CHECK (versi >= 1)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_kpi_nilai ON kpi_nilai (kpi_kode, (COALESCE(wilayah_kode, '')), periode, versi);
CREATE INDEX IF NOT EXISTS idx_kpi_nilai_kpi_periode ON kpi_nilai (kpi_kode, periode DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS kpi_nilai;
DROP TABLE IF EXISTS kpi_target;
DROP TABLE IF EXISTS kpi_definisi;
DROP TYPE IF EXISTS kelompok_kpi;
-- +goose StatementEnd
