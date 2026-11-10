-- Slice 08-B (docs/todos/08-dashboard-operasional.md): penentuan siklus ekonomi otomatis.
--
-- Siklus aktif (siklus_ekonomi, akhir_pada NULL) semula hanya dibaca. Mesin penentuan siklus (internal/siklus/engine)
-- sekarang menghitungnya dari data nyata dan menulisnya, dengan dua tambahan skema:
--
--   * aturan_siklus: aturan penentuan sebagai DATA (bukan kode). Tiap aturan menyebut siklus yang dipicunya
--     (TEKANAN atau SURPLUS; NORMAL adalah keadaan dasar tanpa pemicu), indikatornya, ambang, prioritas, dan masa
--     pulih. Indikator yang dikenal:
--       ALERT_EWS       jumlah alert EWS yang belum SELESAI di wilayah itu menurut jenis_ews dan severity_min;
--                       memicu bila jumlahnya >= jumlah_min (pemetaan jenis -> siklus sama dengan kartu otomatis 14-C).
--       UTILISASI_STOK  terisi (lot cold storage layak simpan, belum HABIS) / kapasitas fasilitas OPERASIONAL
--                       di wilayah itu, dalam persen; memicu bila >= ambang (definisi yang sama dengan KPI dasbor).
--     Prioritas (angka kecil lebih diutamakan) memutus pemicu TEKANAN dan SURPLUS yang muncul bersamaan; bila sama,
--     TEKANAN menang. pulih_jam adalah lama tidak ada satu pun aturan siklus itu memicu sebelum siklus dilepas
--     (histeresis agar siklus tidak berkedip); bila aturan satu siklus berbeda nilai, yang terbesar berlaku.
--     Isi awal ada di seed reference (db/seeds/reference/14_siklus_aturan.sql), bukan di migrasi.
--
--   * siklus_ekonomi: jejak keputusan. otomatis = baris dibuka mesin; alasan = foto keputusan (JSON: ringkasan, aturan
--     yang memicu, dan aturan lain yang kalah prioritas); pulih_sejak = sejak kapan tidak ada aturan siklus ini
--     yang memicu (penanda masa pulih; hanya ada di baris aktif). Baris lama (seed/manual) tetap otomatis = false.
--
-- Hak garda_aplikasi (000119) mengikuti hak bawaan tabel baru (SELECT, INSERT, UPDATE, DELETE), jadi tidak ada GRANT
-- tambahan. Down membuang tabel aturan dan ketiga kolom; baris siklus_ekonomi tidak disentuh.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS aturan_siklus (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kode         varchar(64) NOT NULL,
    siklus       siklus NOT NULL,
    indikator    varchar(32) NOT NULL,
    jenis_ews    jenis_ews,
    severity_min severity,
    jumlah_min   integer,
    ambang       double precision,
    prioritas    integer NOT NULL,
    pulih_jam    integer NOT NULL DEFAULT 48,
    aktif        boolean NOT NULL DEFAULT true,
    keterangan   text NOT NULL DEFAULT '',
    dibuat_pada  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_aturan_siklus_kode UNIQUE (kode),
    CONSTRAINT chk_aturan_siklus_siklus CHECK (siklus <> 'NORMAL'),
    CONSTRAINT chk_aturan_siklus_indikator CHECK (indikator IN ('ALERT_EWS', 'UTILISASI_STOK')),
    CONSTRAINT chk_aturan_siklus_parameter CHECK (
        (indikator = 'ALERT_EWS'
            AND jenis_ews IS NOT NULL AND severity_min IS NOT NULL AND jumlah_min IS NOT NULL AND jumlah_min >= 1
            AND ambang IS NULL)
        OR
        (indikator = 'UTILISASI_STOK'
            AND jenis_ews IS NULL AND severity_min IS NULL AND jumlah_min IS NULL
            AND ambang IS NOT NULL AND ambang > 0 AND ambang <= 100)
    ),
    CONSTRAINT chk_aturan_siklus_prioritas CHECK (prioritas >= 1),
    CONSTRAINT chk_aturan_siklus_pulih CHECK (pulih_jam BETWEEN 0 AND 8760)
);

COMMENT ON TABLE aturan_siklus IS 'Aturan penentuan siklus ekonomi otomatis (08-B). Data konfigurasi; seed reference tidak menimpa perubahan operator.';
COMMENT ON COLUMN aturan_siklus.kode IS 'Kode aturan (SKL-...); kunci idempoten seed reference (ON CONFLICT DO NOTHING).';
COMMENT ON COLUMN aturan_siklus.siklus IS 'Siklus yang dipicu aturan ini: TEKANAN atau SURPLUS (NORMAL = tidak ada pemicu).';
COMMENT ON COLUMN aturan_siklus.indikator IS 'ALERT_EWS: jumlah alert belum SELESAI menurut jenis_ews dan severity_min; UTILISASI_STOK: persen terisi cold storage OPERASIONAL.';
COMMENT ON COLUMN aturan_siklus.jumlah_min IS 'ALERT_EWS: aturan memicu bila jumlah alert yang cocok >= nilai ini.';
COMMENT ON COLUMN aturan_siklus.ambang IS 'UTILISASI_STOK: aturan memicu bila utilisasi (persen) >= nilai ini.';
COMMENT ON COLUMN aturan_siklus.prioritas IS 'Angka kecil lebih diutamakan saat aturan TEKANAN dan SURPLUS memicu bersamaan; bila sama, TEKANAN menang.';
COMMENT ON COLUMN aturan_siklus.pulih_jam IS 'Lama (jam) tidak ada aturan siklus ini yang memicu sebelum siklus dilepas (histeresis); terbesar di antara aturan aktif satu siklus yang berlaku.';

ALTER TABLE siklus_ekonomi
    ADD COLUMN IF NOT EXISTS otomatis boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS alasan jsonb,
    ADD COLUMN IF NOT EXISTS pulih_sejak timestamptz;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_siklus_ekonomi_otomatis') THEN
        ALTER TABLE siklus_ekonomi ADD CONSTRAINT chk_siklus_ekonomi_otomatis
            CHECK (NOT otomatis OR (alasan IS NOT NULL AND jsonb_typeof(alasan) = 'object'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_siklus_ekonomi_pulih') THEN
        ALTER TABLE siklus_ekonomi ADD CONSTRAINT chk_siklus_ekonomi_pulih
            CHECK (pulih_sejak IS NULL OR akhir_pada IS NULL);
    END IF;
END $$;

COMMENT ON COLUMN siklus_ekonomi.otomatis IS 'true bila baris dibuka mesin penentuan siklus (internal/siklus/engine); false untuk seed atau penetapan manual.';
COMMENT ON COLUMN siklus_ekonomi.alasan IS 'Foto keputusan mesin saat siklus dibuka: ringkasan, aturan yang memicu (pemicu), dan yang kalah prioritas (ditekan). NULL untuk baris tak otomatis.';
COMMENT ON COLUMN siklus_ekonomi.pulih_sejak IS 'Sejak kapan tidak ada aturan siklus ini yang memicu (penanda masa pulih). Hanya di baris aktif; NULL bila masih dipicu.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aturan_siklus;

ALTER TABLE siklus_ekonomi DROP CONSTRAINT IF EXISTS chk_siklus_ekonomi_pulih;
ALTER TABLE siklus_ekonomi DROP CONSTRAINT IF EXISTS chk_siklus_ekonomi_otomatis;
ALTER TABLE siklus_ekonomi
    DROP COLUMN IF EXISTS pulih_sejak,
    DROP COLUMN IF EXISTS alasan,
    DROP COLUMN IF EXISTS otomatis;
-- +goose StatementEnd
