-- 01-F: lockout akun satu mekanisme. Percobaan gagal dihitung per email ternormalisasi
-- (huruf kecil, tanpa spasi tepi), bukan per pengguna, sehingga email terdaftar dan tak
-- terdaftar berperilaku identik (anti-enumerasi, docs/prd-monorepo.md §6.1). Kunci tabel
-- adalah SHA-256 heksadesimal email itu: email yang tak dikenal tidak disimpan polos.
--
--   gagal            percobaan gagal pada jendela hitung saat ini
--   jendela_mulai    awal jendela hitung (percobaan gagal pertama); jendela berumur satu
--                    LockoutDuration, lalu hitungan mulai dari nol
--   terkunci_sampai  akhir kunci; NULL = tidak terkunci. Dihitung sekali, saat percobaan gagal
--                    ke-N tercatat, sehingga percobaan selama terkunci tidak memperpanjangnya
--
-- Baris dihapus saat login berhasil lengkap (termasuk langkah MFA) dan saat admin membuka
-- kunci. Baris yang sudah lewat dua kali LockoutDuration tanpa pembaruan tidak berarti apa
-- pun dan dipangkas (UseCase.BersihkanKunciKedaluwarsa). login_attempts tetap menjadi catatan
-- peristiwa (IP, alasan) dan tidak lagi dipakai untuk menghitung kunci.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS kunci_login (
    email_hash      VARCHAR(64)  PRIMARY KEY,
    gagal           INTEGER      NOT NULL DEFAULT 0 CHECK (gagal >= 0),
    jendela_mulai   TIMESTAMPTZ  NOT NULL,
    terkunci_sampai TIMESTAMPTZ,
    diperbarui_pada TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_kunci_login_diperbarui ON kunci_login (diperbarui_pada);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS kunci_login;
-- +goose StatementEnd
