-- Slice 20-A (docs/todos/20-profile.md): profil mandiri. pengguna.telepon menyimpan telepon/WhatsApp pemilik akun
-- dalam bentuk E.164 Indonesia: "+62" diikuti 8-13 digit nasional tanpa awalan 0 (mis. +6281234567890). Bentuk itu
-- hasil normalisasi usecase (auth/usecase/profil.go); CHECK di bawah hanya jaring terakhir agar format tidak
-- menyimpang. NULL berarti belum diisi atau dihapus pemilik; string kosong tidak pernah disimpan.
-- Data pribadi (UU PDP): diubah hanya oleh pemilik akun lewat PATCH /v1/auth/me.
--
-- Hak garda_aplikasi (000119) diberikan per tabel (SELECT, INSERT, UPDATE, DELETE ON ALL TABLES), sehingga kolom baru
-- ikut terjangkau tanpa GRANT tambahan. Down membuang kolom beserta nomor yang sudah diisi.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE pengguna
    ADD COLUMN IF NOT EXISTS telepon varchar(20)
        CONSTRAINT chk_pengguna_telepon CHECK (telepon IS NULL OR telepon ~ '^[+]62[1-9][0-9]{7,12}$');
COMMENT ON COLUMN pengguna.telepon IS 'Telepon/WhatsApp pemilik akun, E.164 Indonesia (+62...); NULL = belum diisi. Diubah pemilik lewat PATCH /v1/auth/me.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pengguna DROP COLUMN IF EXISTS telepon;
-- +goose StatementEnd
