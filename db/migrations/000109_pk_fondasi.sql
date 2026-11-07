-- Standardisasi kunci primer (docs/SKEMA-DATABASE.md), langkah fondasi.
--
-- garda_uuid_stabil(ruang, kunci) menghitung UUID versi 3 (RFC 9562, MD5) dari "<ruang>:<kunci>" di namespace
-- Garda (5b2f8f3e-6a1d-4c7e-9a52-3f0d1c8e7b14). Nilainya identik dengan idx.UUIDStabil di Go
-- (apps/api/pkg/idx), dan kesamaan itu diuji terhadap PostgreSQL di internal/platform/schematest.
--
-- Dipakai migrasi berikut untuk mengubah ID VARCHAR lama menjadi UUID secara deterministik. Basis data yang
-- dimigrasi dan basis data baru yang di-seed lalu memakai UUID yang sama untuk baris yang sama, sehingga seed
-- tetap idempoten dan rujukan antar-seed tidak putus. Baris yang dibuat aplikasi memakai gen_random_uuid().
--
-- Fungsinya IMMUTABLE dan tanpa ekstensi: md5(bytea) dan operasi bit bawaan PostgreSQL. Byte ke-7 diberi versi
-- 3 dan byte ke-9 diberi varian RFC 4122, sama dengan uuid.NewMD5 di Go.

-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION garda_uuid_stabil(ruang text, kunci text)
RETURNS uuid
LANGUAGE sql
IMMUTABLE STRICT PARALLEL SAFE
AS $fn$
    SELECT (
        substr(h, 1, 12)
        || '3'
        || substr(h, 14, 3)
        || to_hex(((('x' || substr(h, 17, 1))::bit(4)::int) & 3) | 8)
        || substr(h, 18, 15)
    )::uuid
    FROM (
        SELECT md5(
            decode('5b2f8f3e6a1d4c7e9a523f0d1c8e7b14', 'hex') || convert_to(ruang || ':' || kunci, 'UTF8')
        ) AS h
    ) s
$fn$;
-- +goose StatementEnd

COMMENT ON FUNCTION garda_uuid_stabil(text, text) IS
    'UUID v3 deterministik untuk seed dan migrasi data ID lama; identik dengan idx.UUIDStabil (Go).';

-- +goose Down
DROP FUNCTION IF EXISTS garda_uuid_stabil(text, text);
