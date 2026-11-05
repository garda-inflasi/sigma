-- Slice 10-B (docs/todos/10-ews.md): rule engine EWS.
-- - ews_alert: dedup_key (satu alert aktif per kondisi, UNIQUE parsial), jejak eskalasi/selesai, id berurut;
-- - ews_riwayat: setiap perubahan status alert (oleh NULL = sistem/rule engine);
-- - aturan_ews: cakupan kabupaten, peredaman (cooldown), versi untuk optimistic locking, pengubah terakhir;
-- - aturan_ews_riwayat: foto setiap versi ambang yang disimpan;
-- - indeks daftar alert dan pencarian per komoditas/kabupaten.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE ews_alert
    ADD COLUMN IF NOT EXISTS dedup_key VARCHAR(255),
    ADD COLUMN IF NOT EXISTS dieskalasi_pada TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS dieskalasi_oleh_id VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS selesai_pada TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ews_alert_dedup_aktif
    ON ews_alert (dedup_key)
    WHERE dedup_key IS NOT NULL AND status <> 'SELESAI';
CREATE INDEX IF NOT EXISTS idx_ews_alert_status_severity_waktu
    ON ews_alert (status, severity, dibuat_pada DESC);
CREATE INDEX IF NOT EXISTS idx_ews_alert_komoditas_kabupaten_waktu
    ON ews_alert (komoditas_kode, kabupaten_kode, dibuat_pada DESC);

-- Nomor id alert buatan sistem (EWS-2000, ...), di atas id seed demo (EWS-0901..EWS-1013).
CREATE SEQUENCE IF NOT EXISTS ews_alert_id_seq START 2000;

CREATE SEQUENCE IF NOT EXISTS ews_riwayat_id_seq;
CREATE TABLE IF NOT EXISTS ews_riwayat (
    id VARCHAR(255) PRIMARY KEY DEFAULT ('RW-' || nextval('ews_riwayat_id_seq')::text),
    alert_id VARCHAR(255) NOT NULL REFERENCES ews_alert(id) ON DELETE CASCADE,
    dari status_ews,
    ke status_ews NOT NULL,
    oleh VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    waktu TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    catatan TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_ews_riwayat_alert ON ews_riwayat (alert_id, waktu);

ALTER TABLE aturan_ews
    ADD COLUMN IF NOT EXISTS kabupaten_kode VARCHAR(4) REFERENCES kabupaten(kode),
    ADD COLUMN IF NOT EXISTS cooldown_jam INT NOT NULL DEFAULT 24,
    ADD COLUMN IF NOT EXISTS versi INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS diubah_oleh_id VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS diubah_pada TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_aturan_ews_jendela_cooldown') THEN
        ALTER TABLE aturan_ews ADD CONSTRAINT chk_aturan_ews_jendela_cooldown
            CHECK (jendela_hari > 0 AND cooldown_jam >= 0 AND versi > 0);
    END IF;
END $$;

CREATE SEQUENCE IF NOT EXISTS aturan_ews_riwayat_id_seq;
CREATE TABLE IF NOT EXISTS aturan_ews_riwayat (
    id VARCHAR(255) PRIMARY KEY DEFAULT ('AR-' || nextval('aturan_ews_riwayat_id_seq')::text),
    aturan_id VARCHAR(255) NOT NULL REFERENCES aturan_ews(id) ON DELETE CASCADE,
    versi INT NOT NULL,
    ambang_waspada FLOAT NOT NULL,
    ambang_siaga FLOAT NOT NULL,
    ambang_kritis FLOAT NOT NULL,
    jendela_hari INT NOT NULL,
    cooldown_jam INT NOT NULL,
    kanal TEXT[] NOT NULL,
    aktif BOOLEAN NOT NULL,
    keterangan TEXT NOT NULL DEFAULT '',
    diubah_oleh_id VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    waktu TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (aturan_id, versi)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aturan_ews_riwayat;
DROP SEQUENCE IF EXISTS aturan_ews_riwayat_id_seq;

ALTER TABLE aturan_ews DROP CONSTRAINT IF EXISTS chk_aturan_ews_jendela_cooldown;
ALTER TABLE aturan_ews
    DROP COLUMN IF EXISTS diubah_pada,
    DROP COLUMN IF EXISTS diubah_oleh_id,
    DROP COLUMN IF EXISTS versi,
    DROP COLUMN IF EXISTS cooldown_jam,
    DROP COLUMN IF EXISTS kabupaten_kode;

DROP TABLE IF EXISTS ews_riwayat;
DROP SEQUENCE IF EXISTS ews_riwayat_id_seq;
DROP SEQUENCE IF EXISTS ews_alert_id_seq;

DROP INDEX IF EXISTS idx_ews_alert_komoditas_kabupaten_waktu;
DROP INDEX IF EXISTS idx_ews_alert_status_severity_waktu;
DROP INDEX IF EXISTS uq_ews_alert_dedup_aktif;
ALTER TABLE ews_alert
    DROP COLUMN IF EXISTS selesai_pada,
    DROP COLUMN IF EXISTS dieskalasi_oleh_id,
    DROP COLUMN IF EXISTS dieskalasi_pada,
    DROP COLUMN IF EXISTS dedup_key;
-- +goose StatementEnd
