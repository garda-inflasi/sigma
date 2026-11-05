-- Slice 09-A (docs/todos/09-dashboard-eksekutif.md): tabel nyata untuk dashboard eksekutif.
-- Menggantikan tabel yang dirujuk kode lama tetapi tidak pernah ada
-- (ringkasan_eksekutif, tren_inflasi, komoditas_sumbangan, daerah_inflasi,
-- neraca/target tanpa tabel). Tidak ada data di sini: angka resmi hanya masuk
-- lewat impor (09-B); data contoh ber-sumber 'DEMO' ditanam seeder eksekutif.demo.
--
-- wilayah_kode berisi kode provinsi (2 digit) atau kabupaten/kota (4 digit) BPS,
-- sehingga tidak dapat memakai satu kunci asing; panjangnya dijaga CHECK.
-- sumber menandai asal angka agar data demo dan resmi tidak tercampur.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS inflasi_bulanan (
    wilayah_kode VARCHAR(4) NOT NULL,
    periode DATE NOT NULL,
    ihk NUMERIC(8, 3) NOT NULL,
    inflasi_mtm NUMERIC(6, 2) NOT NULL,
    inflasi_yoy NUMERIC(6, 2) NOT NULL,
    inflasi_ytd NUMERIC(6, 2) NOT NULL,
    sumber VARCHAR(10) NOT NULL,
    diimpor_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (wilayah_kode, periode),
    CONSTRAINT chk_inflasi_bulanan_wilayah CHECK (length(wilayah_kode) IN (2, 4)),
    CONSTRAINT chk_inflasi_bulanan_periode CHECK (EXTRACT(DAY FROM periode) = 1),
    CONSTRAINT chk_inflasi_bulanan_sumber CHECK (sumber IN ('BPS', 'BI', 'DJPB', 'BAPANAS', 'DEMO'))
);

CREATE TABLE IF NOT EXISTS andil_inflasi (
    periode DATE NOT NULL,
    wilayah_kode VARCHAR(4) NOT NULL,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    andil NUMERIC(6, 3) NOT NULL,
    inflasi_pct NUMERIC(6, 2) NOT NULL,
    sumber VARCHAR(10) NOT NULL,
    diimpor_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (periode, wilayah_kode, komoditas_kode),
    CONSTRAINT chk_andil_inflasi_wilayah CHECK (length(wilayah_kode) IN (2, 4)),
    CONSTRAINT chk_andil_inflasi_periode CHECK (EXTRACT(DAY FROM periode) = 1),
    CONSTRAINT chk_andil_inflasi_sumber CHECK (sumber IN ('BPS', 'BI', 'DJPB', 'BAPANAS', 'DEMO'))
);

CREATE TABLE IF NOT EXISTS neraca_pangan (
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    wilayah_kode VARCHAR(4) NOT NULL,
    periode DATE NOT NULL,
    produksi_ton NUMERIC(12, 1) NOT NULL,
    konsumsi_ton NUMERIC(12, 1) NOT NULL,
    stok_ton NUMERIC(12, 1),
    -- Turunan: positif = surplus, negatif = defisit.
    surplus_ton NUMERIC(12, 1) GENERATED ALWAYS AS (produksi_ton - konsumsi_ton) STORED,
    sumber VARCHAR(10) NOT NULL,
    diimpor_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (komoditas_kode, wilayah_kode, periode),
    CONSTRAINT chk_neraca_pangan_wilayah CHECK (length(wilayah_kode) IN (2, 4)),
    CONSTRAINT chk_neraca_pangan_periode CHECK (EXTRACT(DAY FROM periode) = 1),
    CONSTRAINT chk_neraca_pangan_volume CHECK (produksi_ton >= 0 AND konsumsi_ton > 0 AND (stok_ton IS NULL OR stok_ton >= 0)),
    CONSTRAINT chk_neraca_pangan_sumber CHECK (sumber IN ('BPS', 'BI', 'DJPB', 'BAPANAS', 'DEMO'))
);
CREATE INDEX IF NOT EXISTS idx_neraca_pangan_periode ON neraca_pangan(wilayah_kode, periode DESC);

CREATE TABLE IF NOT EXISTS target_inflasi (
    tahun INT PRIMARY KEY,
    batas_bawah NUMERIC(4, 2) NOT NULL,
    batas_atas NUMERIC(4, 2) NOT NULL,
    CONSTRAINT chk_target_inflasi_batas CHECK (batas_bawah < batas_atas)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS target_inflasi;
DROP TABLE IF EXISTS neraca_pangan;
DROP TABLE IF EXISTS andil_inflasi;
DROP TABLE IF EXISTS inflasi_bulanan;
-- +goose StatementEnd
