-- Slice 02-C (docs/todos/02-pengguna-rbac.md): RBAC berpindah dari "Opsi A" (peta di kode, tabel dibuang di
-- 000010) ke "Opsi B": basis data adalah sumber izin yang berlaku, matriks peran x izin dapat diedit di UI,
-- dan admin dapat memberi izin langsung per pengguna dengan masa berlaku dan alasan.
--
-- Katalog izin tetap didefinisikan di kode (route merujuk nama-namanya); tabel ini menyimpan deskripsi, kelompok,
-- dan penugasan. Migrasi hanya membuat skema: seeder lapis reference (auth.reference) mengisi 14 peran, 35 izin,
-- dan matriks bawaan dengan ON CONFLICT DO NOTHING agar perubahan admin tidak pernah tertimpa.
--
--   roles             kunci = nilai enum peran (14 baris). versi naik setiap matriks peran diubah (kunci
--                     optimistis: dua admin tidak saling menimpa).
--   permissions       nama izin domain.aksi (35 baris), deskripsi, kelompok.
--   role_permissions  izin yang dimiliki sebuah peran.
--   user_permissions  izin langsung per pengguna: berlaku_sampai NULL = tanpa batas, alasan wajib. Satu baris per
--                     (pengguna, izin); baris kedaluwarsa tetap tersimpan sebagai riwayat sampai dicabut atau
--                     diganti pemberian baru.
--
-- Pencarian per request (izin efektif satu pengguna) memakai awalan PK kedua tabel penugasan:
--   role_permissions (role_kunci, ...) dan user_permissions (pengguna_id, ...).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS roles (
    kunci peran PRIMARY KEY,
    label VARCHAR(100) NOT NULL CHECK (btrim(label) <> ''),
    lapis lapis NOT NULL,
    urutan SMALLINT NOT NULL CHECK (urutan > 0),
    versi BIGINT NOT NULL DEFAULT 1 CHECK (versi > 0),
    diperbarui_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    diperbarui_oleh VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS permissions (
    nama VARCHAR(64) PRIMARY KEY CHECK (nama ~ '^[a-z]+\.[a-z_]+$'),
    deskripsi TEXT NOT NULL CHECK (btrim(deskripsi) <> ''),
    kelompok VARCHAR(64) NOT NULL CHECK (btrim(kelompok) <> '')
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_kunci peran NOT NULL REFERENCES roles(kunci) ON DELETE CASCADE,
    permission_nama VARCHAR(64) NOT NULL REFERENCES permissions(nama) ON DELETE CASCADE,
    PRIMARY KEY (role_kunci, permission_nama)
);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission ON role_permissions (permission_nama);

CREATE TABLE IF NOT EXISTS user_permissions (
    pengguna_id VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    permission_nama VARCHAR(64) NOT NULL REFERENCES permissions(nama) ON DELETE CASCADE,
    berlaku_sampai TIMESTAMPTZ,
    alasan TEXT NOT NULL CHECK (btrim(alasan) <> '' AND char_length(alasan) <= 500),
    diberikan_oleh VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    diberikan_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (pengguna_id, permission_nama)
);
CREATE INDEX IF NOT EXISTS idx_user_permissions_permission ON user_permissions (permission_nama);
CREATE INDEX IF NOT EXISTS idx_user_permissions_berlaku ON user_permissions (berlaku_sampai) WHERE berlaku_sampai IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
-- +goose StatementEnd
