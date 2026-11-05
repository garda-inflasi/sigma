-- Slice 05-B (docs/todos/05-sumber-data.md): kerangka ingestion dan sumber pertama (PIHPS BI).
-- Tidak ada data di sini; pemetaan awal ditanam seeder ingestion.reference.
--
-- * sumber_data.konfigurasi: parameter permintaan per sumber (bukan rahasia). Kunci API
--   tidak disimpan di basis data; konektor membacanya dari variabel lingkungan.
-- * eksekusi_sumber: satu baris per run (jadwal, manual, CLI). Sumber paling banyak punya
--   satu run BERJALAN (indeks unik parsial) sehingga penjadwal dan tombol tidak saling tumpang.
--   keberhasilan_7h dan latensi_ms di sumber_data dihitung dari tabel ini.
-- * data_mentah (bronze): payload asli respons + checksum; UNIQUE (sumber_id, checksum)
--   membuat pengambilan ulang respons yang sama tidak menggandakan baris.
-- * pemetaan_sumber: alias komoditas/wilayah milik sumber -> kode master. Alias yang sengaja
--   diabaikan (mis. baris induk kategori) ditandai eksplisit, bukan dibuang diam-diam.
-- * harga_eksternal (silver): harga terstandar per komoditas, kabupaten/kota, tanggal.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE sumber_data ADD COLUMN IF NOT EXISTS konfigurasi JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS eksekusi_sumber (
    id BIGSERIAL PRIMARY KEY,
    sumber_id VARCHAR(255) NOT NULL REFERENCES sumber_data(id),
    pemicu VARCHAR(10) NOT NULL,
    aktor_id VARCHAR(255) REFERENCES pengguna(id),
    status VARCHAR(10) NOT NULL DEFAULT 'BERJALAN',
    mulai TIMESTAMPTZ NOT NULL DEFAULT now(),
    selesai TIMESTAMPTZ,
    durasi_ms INT,
    latensi_ms INT,
    dokumen INT NOT NULL DEFAULT 0,
    baris_dibaca INT NOT NULL DEFAULT 0,
    baris_diambil INT NOT NULL DEFAULT 0,
    baris_tak_terpetakan INT NOT NULL DEFAULT 0,
    baris_tidak_valid INT NOT NULL DEFAULT 0,
    baris_ditolak INT GENERATED ALWAYS AS (baris_tak_terpetakan + baris_tidak_valid) STORED,
    galat TEXT,
    ringkasan JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT chk_eksekusi_sumber_status CHECK (status IN ('BERJALAN', 'SUKSES', 'SEBAGIAN', 'GAGAL')),
    CONSTRAINT chk_eksekusi_sumber_pemicu CHECK (pemicu IN ('JADWAL', 'MANUAL', 'CLI', 'DEMO')),
    CONSTRAINT chk_eksekusi_sumber_selesai CHECK ((status = 'BERJALAN') = (selesai IS NULL)),
    CONSTRAINT chk_eksekusi_sumber_baris CHECK (
        baris_dibaca >= 0 AND baris_diambil >= 0 AND baris_tak_terpetakan >= 0 AND baris_tidak_valid >= 0)
);
CREATE INDEX IF NOT EXISTS idx_eksekusi_sumber_riwayat ON eksekusi_sumber(sumber_id, mulai DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_eksekusi_sumber_berjalan ON eksekusi_sumber(sumber_id) WHERE status = 'BERJALAN';

CREATE TABLE IF NOT EXISTS data_mentah (
    id BIGSERIAL PRIMARY KEY,
    sumber_id VARCHAR(255) NOT NULL REFERENCES sumber_data(id),
    eksekusi_id BIGINT NOT NULL REFERENCES eksekusi_sumber(id),
    kunci_permintaan VARCHAR(255) NOT NULL,
    tipe_konten VARCHAR(100) NOT NULL DEFAULT 'application/json',
    diambil_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload JSONB NOT NULL,
    checksum CHAR(64) NOT NULL,
    CONSTRAINT uq_data_mentah_checksum UNIQUE (sumber_id, checksum)
);
CREATE INDEX IF NOT EXISTS idx_data_mentah_eksekusi ON data_mentah(eksekusi_id);
CREATE INDEX IF NOT EXISTS idx_data_mentah_diambil ON data_mentah(sumber_id, diambil_pada DESC);

CREATE TABLE IF NOT EXISTS pemetaan_sumber (
    sumber_id VARCHAR(255) NOT NULL REFERENCES sumber_data(id),
    jenis VARCHAR(10) NOT NULL,
    -- Dinormalisasi: huruf kecil, spasi tunggal.
    alias VARCHAR(255) NOT NULL,
    kode_master VARCHAR(255),
    diabaikan BOOLEAN NOT NULL DEFAULT false,
    catatan TEXT,
    PRIMARY KEY (sumber_id, jenis, alias),
    CONSTRAINT chk_pemetaan_sumber_jenis CHECK (jenis IN ('KOMODITAS', 'WILAYAH')),
    CONSTRAINT chk_pemetaan_sumber_tujuan CHECK (diabaikan = (kode_master IS NULL))
);

CREATE TABLE IF NOT EXISTS harga_eksternal (
    sumber_id VARCHAR(255) NOT NULL REFERENCES sumber_data(id),
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    tanggal DATE NOT NULL,
    tipe tipe_harga NOT NULL DEFAULT 'KONSUMEN',
    harga NUMERIC(14, 2) NOT NULL,
    satuan VARCHAR(50) NOT NULL,
    data_mentah_id BIGINT REFERENCES data_mentah(id),
    eksekusi_id BIGINT REFERENCES eksekusi_sumber(id),
    diperbarui_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe),
    CONSTRAINT chk_harga_eksternal_harga CHECK (harga > 0)
);
CREATE INDEX IF NOT EXISTS idx_harga_eksternal_lookup ON harga_eksternal(komoditas_kode, kabupaten_kode, tanggal);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS harga_eksternal;
DROP TABLE IF EXISTS pemetaan_sumber;
DROP TABLE IF EXISTS data_mentah;
DROP TABLE IF EXISTS eksekusi_sumber;
ALTER TABLE sumber_data DROP COLUMN IF EXISTS konfigurasi;
-- +goose StatementEnd
