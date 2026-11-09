-- Slice 20-C (docs/todos/20-profile.md, backlog "Unggah foto profil"): foto profil pemilik akun.
--
-- Mengapa di PostgreSQL, bukan penyimpanan objek (docs/todos/18-platform-lintas-fitur.md §C): foto profil kecil
-- (hasil kompres sisi server, persegi 256x256 JPEG, kira-kira di bawah 100 KB), satu per akun, tanpa infrastruktur
-- baru, dan ikut tercakup backup/restore basis data. Penyimpanan objek tetap untuk foto lapangan dan lampiran brief.
--
-- Tabel terpisah dari pengguna: kolom bytea tidak ikut terbaca setiap kali baris pengguna dimuat (login, token,
-- daftar), dan GET /v1/auth/me hanya membaca sidik jarinya (sha256) untuk foto_versi.
--
--   pengguna_id  pemilik; PK sekaligus FK (1:1 dengan pengguna). Akun dihapus = foto ikut terhapus (CASCADE);
--                akun yang hanya dinonaktifkan tetap menyimpan fotonya.
--   tipe         tipe media hasil olahan; saat ini selalu image/jpeg (unggahan asli tidak pernah disimpan).
--   lebar/tinggi piksel hasil olahan.
--   ukuran       jumlah byte data; CHECK menahannya 1 sampai 512 KiB dan sama dengan octet_length(data).
--   sha256       sidik jari konten (heksadesimal huruf kecil, 64 karakter): ETag GET /v1/auth/me/foto, dan 12 karakter
--                pertamanya menjadi foto_versi (kunci cache ?v= di UI).
--   data         byte JPEG hasil olahan: metadata (EXIF/GPS/ICC) sudah dibuang karena gambar didekode lalu dienkode ulang.
--   diubah_pada  waktu foto terakhir diganti.
--
-- Data pribadi (UU PDP): hanya pemilik akun yang dapat membacanya, lewat GET /v1/auth/me/foto; tidak ada jalur baca
-- untuk pengguna lain. Isinya tidak pernah masuk token, log, maupun audit (audit hanya mencatat ukuran dan tipe).
--
-- Hak garda_aplikasi (000119): hak bawaan (ALTER DEFAULT PRIVILEGES) memberi SELECT, INSERT, UPDATE, DELETE pada tabel
-- yang dibuat migrasi berikutnya, dan garda_terapkan_hak_aplikasi() memberinya pada semua tabel; tabel ini bukan
-- tabel append-only, jadi tidak ada GRANT atau REVOKE tambahan.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pengguna_foto (
    pengguna_id uuid         PRIMARY KEY REFERENCES pengguna (id) ON DELETE CASCADE,
    tipe        varchar(30)  NOT NULL DEFAULT 'image/jpeg',
    lebar       smallint     NOT NULL CONSTRAINT chk_pengguna_foto_lebar CHECK (lebar > 0),
    tinggi      smallint     NOT NULL CONSTRAINT chk_pengguna_foto_tinggi CHECK (tinggi > 0),
    ukuran      integer      NOT NULL CONSTRAINT chk_pengguna_foto_ukuran CHECK (ukuran BETWEEN 1 AND 524288),
    sha256      char(64)     NOT NULL CONSTRAINT chk_pengguna_foto_sha256 CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    data        bytea        NOT NULL,
    diubah_pada timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT chk_pengguna_foto_ukuran_data CHECK (ukuran = octet_length(data))
);

COMMENT ON TABLE pengguna_foto IS 'Foto profil pemilik akun (1:1 dengan pengguna): JPEG persegi hasil olahan server, bukan unggahan asli. Dibaca hanya oleh pemiliknya lewat GET /v1/auth/me/foto.';
COMMENT ON COLUMN pengguna_foto.pengguna_id IS 'Pemilik foto; PK sekaligus FK ke pengguna(id), ikut terhapus bersama akunnya (CASCADE).';
COMMENT ON COLUMN pengguna_foto.tipe IS 'Tipe media hasil olahan; saat ini selalu image/jpeg.';
COMMENT ON COLUMN pengguna_foto.lebar IS 'Lebar hasil olahan dalam piksel.';
COMMENT ON COLUMN pengguna_foto.tinggi IS 'Tinggi hasil olahan dalam piksel.';
COMMENT ON COLUMN pengguna_foto.ukuran IS 'Jumlah byte data (1 sampai 524288); sama dengan octet_length(data).';
COMMENT ON COLUMN pengguna_foto.sha256 IS 'Sidik jari konten (heksadesimal huruf kecil): ETag foto dan, 12 karakter pertamanya, foto_versi pada GET /v1/auth/me.';
COMMENT ON COLUMN pengguna_foto.data IS 'Byte JPEG hasil olahan; EXIF/GPS/ICC sudah dibuang karena didekode lalu dienkode ulang.';
COMMENT ON COLUMN pengguna_foto.diubah_pada IS 'Waktu foto terakhir diganti.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pengguna_foto;
-- +goose StatementEnd
