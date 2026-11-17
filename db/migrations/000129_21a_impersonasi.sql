-- 21-A: impersonasi peran dan akun (alih akun sementara untuk pengecekan). Lihat docs/impersonasi.md.
--
-- Migrasi hanya membuat skema (tes TestMigrationsLeaveEveryTableEmpty): tanpa baris di impersonasi_kebijakan,
-- kebijakan bawaan berlaku di aplikasi (aktif, wajib alasan, 60 menit); seeder reference juga menyemainya.
--
--   impersonasi_kebijakan  satu baris (id = 1): saklar global, kewajiban alasan, batas durasi, versi (kunci optimistis).
--   impersonasi_pelaku     allowlist pihak SELAIN superadmin (ADMIN_SISTEM selalu berhak) yang boleh memulai impersonasi:
--                          satu peran atau satu akun. ADMIN_SISTEM tidak boleh masuk daftar (redundan).
--   impersonasi_target     sasaran yang diizinkan per pelaku: satu peran atau satu akun. ADMIN_SISTEM tidak pernah boleh
--                          menjadi sasaran, oleh siapa pun (dijaga CHECK di sini dan di usecase).
--   impersonasi_sesi       satu baris per impersonasi (riwayat sekaligus sumber kebenaran sisi server): pelaku, sasaran,
--                          alasan, sesi (family_id) dan kapan berakhir. Nama, email, dan peran disalin (snapshot) agar
--                          riwayat tetap terbaca walau akunnya kemudian dihapus (FK SET NULL).
--   refresh_tokens.impersonasi_id  menandai family refresh token milik sesi impersonasi: tidak tampil di daftar sesi milik
--                          sasaran, dan rotasi refresh menerbitkan token ber-klaim impersonasi.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS impersonasi_kebijakan (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    aktif BOOLEAN NOT NULL DEFAULT TRUE,
    wajib_alasan BOOLEAN NOT NULL DEFAULT TRUE,
    durasi_maks_menit INTEGER CHECK (durasi_maks_menit IS NULL OR durasi_maks_menit BETWEEN 5 AND 1440),
    versi BIGINT NOT NULL DEFAULT 1 CHECK (versi > 0),
    diperbarui_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    diperbarui_oleh UUID REFERENCES pengguna(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_impersonasi_kebijakan_diperbarui_oleh ON impersonasi_kebijakan (diperbarui_oleh);

CREATE TABLE IF NOT EXISTS impersonasi_pelaku (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tipe VARCHAR(16) NOT NULL CHECK (tipe IN ('PERAN', 'PENGGUNA')),
    peran peran,
    pengguna_id UUID REFERENCES pengguna(id) ON DELETE CASCADE,
    aktif BOOLEAN NOT NULL DEFAULT TRUE,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_impersonasi_pelaku_tipe CHECK (
        (tipe = 'PERAN' AND peran IS NOT NULL AND pengguna_id IS NULL)
        OR (tipe = 'PENGGUNA' AND pengguna_id IS NOT NULL AND peran IS NULL)
    ),
    CONSTRAINT ck_impersonasi_pelaku_bukan_admin CHECK (peran IS DISTINCT FROM 'ADMIN_SISTEM')
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_impersonasi_pelaku_peran ON impersonasi_pelaku (peran) WHERE tipe = 'PERAN';
CREATE UNIQUE INDEX IF NOT EXISTS uq_impersonasi_pelaku_pengguna ON impersonasi_pelaku (pengguna_id) WHERE tipe = 'PENGGUNA';

CREATE TABLE IF NOT EXISTS impersonasi_target (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pelaku_id UUID NOT NULL REFERENCES impersonasi_pelaku(id) ON DELETE CASCADE,
    tipe VARCHAR(16) NOT NULL CHECK (tipe IN ('PERAN', 'PENGGUNA')),
    peran peran,
    pengguna_id UUID REFERENCES pengguna(id) ON DELETE CASCADE,
    CONSTRAINT ck_impersonasi_target_tipe CHECK (
        (tipe = 'PERAN' AND peran IS NOT NULL AND pengguna_id IS NULL)
        OR (tipe = 'PENGGUNA' AND pengguna_id IS NOT NULL AND peran IS NULL)
    ),
    CONSTRAINT ck_impersonasi_target_bukan_admin CHECK (peran IS DISTINCT FROM 'ADMIN_SISTEM')
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_impersonasi_target_peran ON impersonasi_target (pelaku_id, peran) WHERE tipe = 'PERAN';
CREATE UNIQUE INDEX IF NOT EXISTS uq_impersonasi_target_pengguna ON impersonasi_target (pelaku_id, pengguna_id) WHERE tipe = 'PENGGUNA';
CREATE INDEX IF NOT EXISTS idx_impersonasi_target_pengguna ON impersonasi_target (pengguna_id) WHERE pengguna_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS impersonasi_sesi (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pelaku_id UUID REFERENCES pengguna(id) ON DELETE SET NULL,
    pelaku_nama VARCHAR(255) NOT NULL,
    pelaku_email VARCHAR(255) NOT NULL,
    pelaku_peran peran NOT NULL,
    sasaran_id UUID REFERENCES pengguna(id) ON DELETE SET NULL,
    sasaran_nama VARCHAR(255) NOT NULL,
    sasaran_email VARCHAR(255) NOT NULL,
    sasaran_peran peran NOT NULL,
    alasan TEXT CHECK (alasan IS NULL OR (btrim(alasan) <> '' AND char_length(alasan) <= 500)),
    family_id UUID NOT NULL,
    sesi_asal_mulai TIMESTAMPTZ NOT NULL,
    mulai_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    kedaluwarsa_pada TIMESTAMPTZ,
    berakhir_pada TIMESTAMPTZ,
    alasan_berakhir VARCHAR(16) CHECK (alasan_berakhir IN ('STOP', 'BERGANTI', 'KEDALUWARSA', 'KEBIJAKAN', 'DIAKHIRI', 'LOGOUT', 'DICABUT')),
    ip VARCHAR(45),
    user_agent VARCHAR(512),
    CONSTRAINT ck_impersonasi_sesi_berakhir CHECK ((berakhir_pada IS NULL) = (alasan_berakhir IS NULL)),
    CONSTRAINT ck_impersonasi_sesi_bukan_admin CHECK (sasaran_peran <> 'ADMIN_SISTEM')
);
-- Satu sesi (family) hanya boleh punya satu catatan impersonasi yang hidup.
CREATE UNIQUE INDEX IF NOT EXISTS uq_impersonasi_sesi_family_aktif ON impersonasi_sesi (family_id) WHERE berakhir_pada IS NULL;
CREATE INDEX IF NOT EXISTS idx_impersonasi_sesi_family ON impersonasi_sesi (family_id);
CREATE INDEX IF NOT EXISTS idx_impersonasi_sesi_pelaku ON impersonasi_sesi (pelaku_id, mulai_pada DESC);
CREATE INDEX IF NOT EXISTS idx_impersonasi_sesi_sasaran ON impersonasi_sesi (sasaran_id, mulai_pada DESC);
CREATE INDEX IF NOT EXISTS idx_impersonasi_sesi_mulai ON impersonasi_sesi (mulai_pada DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_impersonasi_sesi_aktif ON impersonasi_sesi (mulai_pada DESC, id DESC) WHERE berakhir_pada IS NULL;

ALTER TABLE refresh_tokens
    ADD COLUMN IF NOT EXISTS impersonasi_id UUID REFERENCES impersonasi_sesi(id) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS idx_rt_impersonasi ON refresh_tokens (impersonasi_id) WHERE impersonasi_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_rt_impersonasi;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS impersonasi_id;
DROP TABLE IF EXISTS impersonasi_sesi;
DROP TABLE IF EXISTS impersonasi_target;
DROP TABLE IF EXISTS impersonasi_pelaku;
DROP TABLE IF EXISTS impersonasi_kebijakan;
-- +goose StatementEnd
