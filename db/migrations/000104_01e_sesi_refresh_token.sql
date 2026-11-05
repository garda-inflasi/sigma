-- 01-E: sesi aktif dan rotasi refresh token.
-- Satu baris refresh_tokens tetap satu token; satu sesi (perangkat) adalah satu family_id, yang juga menjadi
-- klaim `sid` pada access token sehingga pencabutan sesi berlaku seketika.
--
-- - user_agent: UA peramban saat sesi dibuat atau token terakhir diputar (untuk daftar sesi). Kolom `device`
--   dari 000001 tidak pernah dipakai dan sengaja dibiarkan; ringkasan perangkat ("Chrome di macOS") dihitung
--   dari user_agent saat dibaca, bukan disimpan.
-- - last_used_at: kapan sesi terakhir menukar token (login atau rotasi), selaras dengan umur access token.
-- - session_started_at: awal sesi; diwariskan ke setiap token pengganti dalam family yang sama.
-- - rotated_at: diisi bersama revoked_at bila token dicabut karena DIGANTI oleh rotasi (bukan karena logout,
--   pencabutan, atau penonaktifan). Dipakai untuk membedakan "token lama dipakai ulang" (kemungkinan dicuri,
--   cabut seluruh family) dari "sesi sudah dicabut" (biasa), dan untuk jendela toleransi tab serentak.
-- - idx_rt_user_revoked: daftar sesi aktif dan pencabutan massal per pengguna.
-- - idx_rt_family_aktif (parsial): pemeriksaan "sesi ini masih hidup?" pada setiap request terotentikasi
--   cukup membaca satu entri indeks, berapa pun jumlah baris rotasi lama dalam family.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE refresh_tokens
    ADD COLUMN IF NOT EXISTS user_agent VARCHAR(512),
    ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS session_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ;

-- Baris yang sudah ada: waktu pakai = waktu terbit; awal sesi = token tertua dalam family; token yang
-- sudah dicabut dan punya pengganti dianggap hasil rotasi.
UPDATE refresh_tokens SET last_used_at = created_at;

UPDATE refresh_tokens r
SET session_started_at = f.mulai
FROM (SELECT family_id, MIN(created_at) AS mulai FROM refresh_tokens GROUP BY family_id) f
WHERE f.family_id = r.family_id;

UPDATE refresh_tokens r
SET rotated_at = r.revoked_at
WHERE r.revoked_at IS NOT NULL
  AND EXISTS (
      SELECT 1 FROM refresh_tokens n
      WHERE n.family_id = r.family_id AND n.created_at > r.created_at
  );

CREATE INDEX IF NOT EXISTS idx_rt_user_revoked ON refresh_tokens (user_id, revoked_at);
CREATE INDEX IF NOT EXISTS idx_rt_family_aktif ON refresh_tokens (family_id) WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_rt_family_aktif;
DROP INDEX IF EXISTS idx_rt_user_revoked;
ALTER TABLE refresh_tokens
    DROP COLUMN IF EXISTS rotated_at,
    DROP COLUMN IF EXISTS session_started_at,
    DROP COLUMN IF EXISTS last_used_at,
    DROP COLUMN IF EXISTS user_agent;
-- +goose StatementEnd
