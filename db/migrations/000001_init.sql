-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ── Enums ───────────────────────────────────────────────────────────
DO $$ BEGIN
  CREATE TYPE lapis AS ENUM ('STRATEGIS', 'MANAJERIAL', 'OPERASIONAL', 'SISTEM');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE peran AS ENUM (
    'STRATEGIS_FMIPA', 'STRATEGIS_BI', 'STRATEGIS_BPS', 'STRATEGIS_DJPB',
    'MANAJER_PROGRAM', 'MANAJER_DATA', 'MANAJER_KEBIJAKAN', 'MANAJER_PEMBIAYAAN',
    'MANAJER_RISIKO', 'MANAJER_MUTU',
    'OPS_KOORDINATOR', 'OPS_AGREGATOR', 'OPS_MAHASISWA',
    'ADMIN_SISTEM'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_akun AS ENUM ('AKTIF', 'NONAKTIF', 'MENUNGGU');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_master AS ENUM ('AKTIF', 'MENUNGGU_PERSETUJUAN', 'DIARSIPKAN');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE tipe_harga AS ENUM ('PRODUSEN', 'KONSUMEN');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE kualitas AS ENUM ('PREMIUM', 'MEDIUM', 'ASALAN');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_entri AS ENUM ('DRAF', 'DIAJUKAN', 'DIVALIDASI', 'DIPUBLIKASI', 'DITOLAK');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE sumber_harga AS ENUM ('LAPANGAN', 'PIHPS', 'BAPANAS', 'PORTAL_DAERAH');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE severity AS ENUM ('INFO', 'WASPADA', 'SIAGA', 'KRITIS');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE jenis_ews AS ENUM ('LONJAKAN_HARGA', 'KELANGKAAN_STOK', 'ANOMALI_CUACA', 'OVER_SUPPLY', 'ANOMALI_DATA');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_ews AS ENUM ('BARU', 'DIAKUI', 'DIESKALASI', 'SELESAI');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE siklus AS ENUM ('NORMAL', 'TEKANAN', 'SURPLUS');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE prioritas AS ENUM ('TINGGI', 'SEDANG', 'RENDAH');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_fasilitas AS ENUM ('OPERASIONAL', 'PERAWATAN', 'RENCANA');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_brief AS ENUM ('DRAF', 'TELAAH', 'DISETUJUI', 'DIDISTRIBUSIKAN');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_tugas AS ENUM ('BELUM', 'BERJALAN', 'SELESAI', 'TERLAMBAT');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE jenis_sumber AS ENUM ('API', 'SCRAPER');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_sumber AS ENUM ('SEHAT', 'TERTUNDA', 'GAGAL', 'DIJEDA');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_persetujuan AS ENUM ('MENUNGGU', 'DISETUJUI', 'DITOLAK');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE status_model AS ENUM ('PRODUKSI', 'KANDIDAT', 'ARSIP');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
-- +goose StatementEnd

-- ── Master data ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS provinsi (
    kode VARCHAR(2) PRIMARY KEY,
    nama VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS kabupaten (
    kode VARCHAR(4) PRIMARY KEY,
    nama VARCHAR(255) NOT NULL,
    jenis VARCHAR(20) NOT NULL,
    ibukota VARCHAR(255) NOT NULL,
    lat FLOAT NOT NULL,
    lng FLOAT NOT NULL,
    provinsi_kode VARCHAR(2) NOT NULL REFERENCES provinsi(kode)
);
CREATE INDEX IF NOT EXISTS idx_kabupaten_provinsi ON kabupaten(provinsi_kode);

CREATE TABLE IF NOT EXISTS pasar (
    id VARCHAR(255) PRIMARY KEY,
    nama VARCHAR(255) NOT NULL,
    kabupaten_kode VARCHAR(4) NOT NULL REFERENCES kabupaten(kode),
    lat FLOAT NOT NULL,
    lng FLOAT NOT NULL,
    radius_meter INT NOT NULL DEFAULT 400,
    aktif BOOLEAN NOT NULL DEFAULT true
);
CREATE INDEX IF NOT EXISTS idx_pasar_kabupaten ON pasar(kabupaten_kode);

CREATE TABLE IF NOT EXISTS komoditas (
    kode VARCHAR(255) PRIMARY KEY,
    nama VARCHAR(255) NOT NULL,
    satuan VARCHAR(50) NOT NULL,
    kategori VARCHAR(100) NOT NULL,
    harga_acuan INT NOT NULL,
    pilot BOOLEAN NOT NULL DEFAULT false,
    shelf_life_hari INT NOT NULL,
    versi INT NOT NULL DEFAULT 1,
    status status_master NOT NULL DEFAULT 'AKTIF'
);

-- ── Auth / RBAC ─────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    is_system BOOLEAN NOT NULL DEFAULT false,
    description TEXT
);

CREATE TABLE IF NOT EXISTS permissions (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    is_system BOOLEAN NOT NULL DEFAULT false,
    UNIQUE(resource, action)
);
CREATE INDEX IF NOT EXISTS idx_permissions_resource ON permissions(resource);

CREATE TABLE IF NOT EXISTS pengguna (
    id VARCHAR(255) PRIMARY KEY,
    nama VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255),
    peran peran NOT NULL,
    lapis lapis NOT NULL,
    lembaga VARCHAR(255) NOT NULL,
    kabupaten_kode VARCHAR(4) REFERENCES kabupaten(kode),
    mfa_aktif BOOLEAN NOT NULL DEFAULT false,
    status status_akun NOT NULL DEFAULT 'AKTIF',
    terakhir_masuk TIMESTAMPTZ,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    diupdate_pada TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_pengguna_peran ON pengguna(peran);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    family_id UUID NOT NULL,
    device VARCHAR(255),
    ip VARCHAR(45),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_rt_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_rt_family ON refresh_tokens(family_id);

CREATE TABLE IF NOT EXISTS login_attempts (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    ip VARCHAR(45),
    success BOOLEAN NOT NULL,
    failure_reason VARCHAR(100),
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_la_user_time ON login_attempts(user_id, attempted_at);
CREATE INDEX IF NOT EXISTS idx_la_ip_time ON login_attempts(ip, attempted_at);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE RESTRICT,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS user_permissions (
    user_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE RESTRICT,
    granted BOOLEAN NOT NULL,
    PRIMARY KEY (user_id, permission_id)
);

CREATE TABLE IF NOT EXISTS password_history (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_password_history_user ON password_history(user_id);

-- ── Seed data: roles & permissions ─────────────────────────────────
INSERT INTO roles (name, slug, is_system, description) VALUES
    ('Superadmin', 'superadmin', true, 'Akses penuh ke seluruh sistem'),
    ('Strategis FMIPA', 'strategis_fmipa', true, 'Pimpinan FMIPA USK'),
    ('Strategis BI', 'strategis_bi', true, 'Pimpinan Bank Indonesia'),
    ('Strategis BPS', 'strategis_bps', true, 'Pimpinan BPS'),
    ('Strategis DJPb', 'strategis_djpb', true, 'Pimpinan DJPb'),
    ('Manajer Program', 'manajer_program', true, 'Manajer program pengendalian inflasi'),
    ('Manajer Data', 'manajer_data', true, 'Manajer data dan analitik'),
    ('Manajer Kebijakan', 'manajer_kebijakan', true, 'Manajer kebijakan'),
    ('Manajer Pembiayaan', 'manajer_pembiayaan', true, 'Manajer pembiayaan'),
    ('Manajer Risiko', 'manajer_risiko', true, 'Manajer risiko'),
    ('Manajer Mutu', 'manajer_mutu', true, 'Manajer mutu'),
    ('Ops Koordinator', 'ops_koordinator', true, 'Koordinator operasional lapangan'),
    ('Ops Agregator', 'ops_agregator', true, 'Agregator data operasional'),
    ('Ops Mahasiswa', 'ops_mahasiswa', true, 'Mahasiswa magang operasional'),
    ('Admin Sistem', 'admin_sistem', true, 'Administrator sistem')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO permissions (name, resource, action, is_system) VALUES
    ('auth:logout', 'auth', 'logout', true),
    ('auth:self', 'auth', 'self', true),
    ('user:read', 'user', 'read', true),
    ('user:write', 'user', 'write', true),
    ('user:delete', 'user', 'delete', true),
    ('role:read', 'role', 'read', true),
    ('role:write', 'role', 'write', true),
    ('role:delete', 'role', 'delete', true),
    ('permission:read', 'permission', 'read', true),
    ('permission:write', 'permission', 'write', true),
    ('permission:delete', 'permission', 'delete', true),
    ('harga:read', 'harga', 'read', true),
    ('harga:write', 'harga', 'write', true),
    ('harga:validate', 'harga', 'validate', true),
    ('harga:publish', 'harga', 'publish', true),
    ('ews:read', 'ews', 'read', true),
    ('ews:write', 'ews', 'write', true),
    ('ews:acknowledge', 'ews', 'acknowledge', true),
    ('prediksi:read', 'prediksi', 'read', true),
    ('intervensi:read', 'intervensi', 'read', true),
    ('intervensi:write', 'intervensi', 'write', true),
    ('dashboard:read', 'dashboard', 'read', true),
    ('coldstorage:read', 'coldstorage', 'read', true),
    ('coldstorage:write', 'coldstorage', 'write', true),
    ('laporan:read', 'laporan', 'read', true),
    ('laporan:generate', 'laporan', 'generate', true),
    ('sumber:read', 'sumber', 'read', true),
    ('sumber:write', 'sumber', 'write', true),
    ('audit:read', 'audit', 'read', true)
ON CONFLICT (resource, action) DO NOTHING;

-- Assign all permissions to superadmin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.slug = 'superadmin'
ON CONFLICT DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS password_history;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS pengguna;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS komoditas;
DROP TABLE IF EXISTS pasar;
DROP TABLE IF EXISTS kabupaten;
DROP TABLE IF EXISTS provinsi;

DROP TYPE IF EXISTS status_model;
DROP TYPE IF EXISTS status_persetujuan;
DROP TYPE IF EXISTS status_sumber;
DROP TYPE IF EXISTS jenis_sumber;
DROP TYPE IF EXISTS status_tugas;
DROP TYPE IF EXISTS status_brief;
DROP TYPE IF EXISTS status_fasilitas;
DROP TYPE IF EXISTS prioritas;
DROP TYPE IF EXISTS siklus;
DROP TYPE IF EXISTS status_ews;
DROP TYPE IF EXISTS jenis_ews;
DROP TYPE IF EXISTS severity;
DROP TYPE IF EXISTS sumber_harga;
DROP TYPE IF EXISTS status_entri;
DROP TYPE IF EXISTS kualitas;
DROP TYPE IF EXISTS tipe_harga;
DROP TYPE IF EXISTS status_master;
DROP TYPE IF EXISTS status_akun;
DROP TYPE IF EXISTS peran;
DROP TYPE IF EXISTS lapis;
-- +goose StatementEnd
