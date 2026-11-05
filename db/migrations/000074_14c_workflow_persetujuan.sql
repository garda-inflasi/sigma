-- Slice 14-C (docs/todos/14-intervensi.md): workflow engine persetujuan berjenjang.
-- - alur_persetujuan + alur_persetujuan_langkah: definisi alur sebagai data (dapat diubah tanpa deploy);
-- - intervensi_persetujuan + intervensi_persetujuan_langkah: permintaan persetujuan per kartu beserta
--   rantai langkahnya (disalin dari definisi saat diajukan) dan keputusan tiap langkah;
-- - intervensi.dibuat_oleh_id boleh NULL (kartu dibuat sistem dari alert), tahap_sejak untuk SLA tahap,
--   UNIQUE parsial agar satu alert hanya menghasilkan satu kartu otomatis;
-- - riwayat kartu mengenal aksi AJUKAN, SETUJU, TOLAK.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE intervensi
    ALTER COLUMN dibuat_oleh_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS tahap_sejak TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS uq_intervensi_otomatis_alert
    ON intervensi (ews_alert_id)
    WHERE dibuat_oleh_id IS NULL AND ews_alert_id IS NOT NULL;

ALTER TABLE intervensi_riwayat DROP CONSTRAINT IF EXISTS chk_intervensi_riwayat_aksi;
ALTER TABLE intervensi_riwayat ADD CONSTRAINT chk_intervensi_riwayat_aksi
    CHECK (aksi IN ('BUAT', 'PINDAH_TAHAP', 'TUGASKAN', 'ESKALASI', 'AJUKAN', 'SETUJU', 'TOLAK'));

CREATE TABLE IF NOT EXISTS alur_persetujuan (
    kode VARCHAR(50) PRIMARY KEY,
    nama VARCHAR(255) NOT NULL,
    jenis VARCHAR(20) NOT NULL,
    siklus siklus,
    tahap_tujuan VARCHAR(100),
    aktif BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT chk_alur_persetujuan_jenis CHECK (
        (jenis = 'PINDAH_TAHAP' AND siklus IS NOT NULL AND tahap_tujuan IS NOT NULL)
        OR (jenis = 'ESKALASI' AND siklus IS NULL AND tahap_tujuan IS NULL)
    ),
    CONSTRAINT fk_alur_persetujuan_tahap FOREIGN KEY (siklus, tahap_tujuan)
        REFERENCES tahap_intervensi_def (siklus, kode_tahap)
);
-- Satu alur aktif per sasaran: per (siklus, tahap tujuan) untuk pemindahan, satu untuk eskalasi.
CREATE UNIQUE INDEX IF NOT EXISTS uq_alur_persetujuan_pindah_aktif
    ON alur_persetujuan (siklus, tahap_tujuan) WHERE aktif AND jenis = 'PINDAH_TAHAP';
CREATE UNIQUE INDEX IF NOT EXISTS uq_alur_persetujuan_eskalasi_aktif
    ON alur_persetujuan (jenis) WHERE aktif AND jenis = 'ESKALASI';

CREATE TABLE IF NOT EXISTS alur_persetujuan_langkah (
    alur_kode VARCHAR(50) NOT NULL REFERENCES alur_persetujuan(kode) ON DELETE CASCADE,
    urutan INT NOT NULL,
    label VARCHAR(255) NOT NULL,
    izin_dibutuhkan VARCHAR(100) NOT NULL,
    PRIMARY KEY (alur_kode, urutan),
    CONSTRAINT chk_alur_langkah_urutan CHECK (urutan > 0)
);

CREATE TABLE IF NOT EXISTS intervensi_persetujuan (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    intervensi_id VARCHAR(255) NOT NULL REFERENCES intervensi(id) ON DELETE CASCADE,
    alur_kode VARCHAR(50) REFERENCES alur_persetujuan(kode) ON DELETE SET NULL,
    jenis VARCHAR(20) NOT NULL,
    tahap_tujuan VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'MENUNGGU',
    langkah_kini INT NOT NULL DEFAULT 1,
    diajukan_oleh VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    diajukan_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    selesai_pada TIMESTAMPTZ,
    catatan TEXT NOT NULL DEFAULT '',
    CONSTRAINT chk_intervensi_persetujuan_jenis CHECK (jenis IN ('PINDAH_TAHAP', 'ESKALASI')),
    CONSTRAINT chk_intervensi_persetujuan_status CHECK (status IN ('MENUNGGU', 'DISETUJUI', 'DITOLAK')),
    CONSTRAINT chk_intervensi_persetujuan_tujuan CHECK ((jenis = 'PINDAH_TAHAP') = (tahap_tujuan IS NOT NULL)),
    CONSTRAINT chk_intervensi_persetujuan_langkah CHECK (langkah_kini > 0)
);
-- Satu permintaan terbuka per kartu.
CREATE UNIQUE INDEX IF NOT EXISTS uq_intervensi_persetujuan_terbuka
    ON intervensi_persetujuan (intervensi_id) WHERE status = 'MENUNGGU';
CREATE INDEX IF NOT EXISTS idx_intervensi_persetujuan_status ON intervensi_persetujuan (status, diajukan_pada);

CREATE TABLE IF NOT EXISTS intervensi_persetujuan_langkah (
    persetujuan_id BIGINT NOT NULL REFERENCES intervensi_persetujuan(id) ON DELETE CASCADE,
    urutan INT NOT NULL,
    label VARCHAR(255) NOT NULL,
    izin_dibutuhkan VARCHAR(100) NOT NULL,
    keputusan VARCHAR(10),
    oleh VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    waktu TIMESTAMPTZ,
    catatan TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (persetujuan_id, urutan),
    CONSTRAINT chk_persetujuan_langkah_keputusan CHECK (keputusan IS NULL OR keputusan IN ('SETUJU', 'TOLAK'))
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS intervensi_persetujuan_langkah;
DROP TABLE IF EXISTS intervensi_persetujuan;
DROP TABLE IF EXISTS alur_persetujuan_langkah;
DROP TABLE IF EXISTS alur_persetujuan;

ALTER TABLE intervensi_riwayat DROP CONSTRAINT IF EXISTS chk_intervensi_riwayat_aksi;
ALTER TABLE intervensi_riwayat ADD CONSTRAINT chk_intervensi_riwayat_aksi
    CHECK (aksi IN ('BUAT', 'PINDAH_TAHAP', 'TUGASKAN', 'ESKALASI'));

DROP INDEX IF EXISTS uq_intervensi_otomatis_alert;
ALTER TABLE intervensi DROP COLUMN IF EXISTS tahap_sejak;
-- Kartu buatan sistem (dibuat_oleh_id NULL) tidak punya pembuat; isi sebelum NOT NULL dipulihkan.
UPDATE intervensi SET dibuat_oleh_id = COALESCE(
    (SELECT id FROM pengguna ORDER BY dibuat_pada, id LIMIT 1), dibuat_oleh_id)
 WHERE dibuat_oleh_id IS NULL;
ALTER TABLE intervensi ALTER COLUMN dibuat_oleh_id SET NOT NULL;
-- +goose StatementEnd
