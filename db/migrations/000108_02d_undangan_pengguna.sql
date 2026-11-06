-- Slice 02-D (docs/todos/02-pengguna-rbac.md, "Undang pengguna lewat email"): admin membuat akun berstatus
-- MENUNGGU tanpa kata sandi, lalu pemilik email menetapkan kata sandi pertamanya lewat tautan sekali pakai
-- (/aktivasi?token=...). Status `MENUNGGU` sudah ada di enum status_akun sejak 000001, jadi tabel pengguna
-- tidak berubah.
--
-- Mengapa tabel sendiri, bukan password_reset_tokens + kolom jenis:
--   * siklus hidupnya berbeda (TTL 72 jam, bukan 30 menit; undangan lama dicabut saat dikirim ulang; hanya
--     berlaku untuk akun MENUNGGU) dan tabel reset dibaca oleh ResetPassword/ConsumeResetToken tanpa
--     penyaring jenis, sehingga token undangan bisa dipakai di jalur atur ulang bila satu saja filter terlewat;
--   * memisahkannya membuat jalur atur ulang kata sandi tetap tidak bisa mengaktifkan akun.
--
--   token_hash     SHA-256 heksadesimal dari token acak 256-bit; token mentah hanya ada di email.
--   email          alamat tujuan saat undangan dikirim. Aktivasi mensyaratkan email akun masih sama: mengubah
--                  email akun MENUNGGU otomatis menghanguskan tautan yang terlanjur dikirim ke alamat lama.
--   expires_at     akhir masa berlaku (72 jam sejak dikirim).
--   used_at        diisi saat tautan dipakai untuk aktivasi (sekali pakai).
--   dicabut_pada   diisi saat undangan diganti undangan baru (kirim ulang) atau dibatalkan.
--   diundang_oleh  admin yang mengirim; NULL bila akunnya sudah dihapus.
--
-- uq_undangan_pengguna_aktif: paling banyak satu undangan hidup per akun (jaring pengaman atas kirim ulang
-- serentak; penggantinya selalu mencabut yang lama dalam transaksi yang sama).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS undangan_pengguna (
    id            BIGSERIAL    PRIMARY KEY,
    user_id       VARCHAR(255) NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    token_hash    VARCHAR(64)  NOT NULL UNIQUE,
    email         VARCHAR(255) NOT NULL,
    diundang_oleh VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    expires_at    TIMESTAMPTZ  NOT NULL,
    used_at       TIMESTAMPTZ,
    dicabut_pada  TIMESTAMPTZ,
    dibuat_pada   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_undangan_pengguna_user ON undangan_pengguna (user_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_undangan_pengguna_aktif
    ON undangan_pengguna (user_id)
    WHERE used_at IS NULL AND dicabut_pada IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS undangan_pengguna;
-- +goose StatementEnd
