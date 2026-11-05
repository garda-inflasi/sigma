-- Slice 06-C (docs/todos/06-harga-validasi.md): validasi otomatis harga.
-- - batas_harga: rentang harga wajar per komoditas (dan kabupaten bila diisi), berlaku dari tanggal tertentu;
-- - entri_harga.flag_validasi: hasil cek otomatis (rentang, outlier, duplikat, geofence) sebagai JSONB.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS batas_harga (
    id VARCHAR(255) PRIMARY KEY,
    komoditas_kode VARCHAR(255) NOT NULL REFERENCES komoditas(kode),
    kabupaten_kode VARCHAR(4) REFERENCES kabupaten(kode),
    batas_min INT NOT NULL,
    batas_maks INT NOT NULL,
    berlaku_dari DATE NOT NULL,
    CONSTRAINT chk_batas_harga_rentang CHECK (batas_min > 0 AND batas_maks > batas_min)
);
-- kabupaten_kode NULL berarti berlaku untuk semua kabupaten; NULL dipetakan agar ikut unik.
CREATE UNIQUE INDEX IF NOT EXISTS uq_batas_harga_berlaku
    ON batas_harga (komoditas_kode, COALESCE(kabupaten_kode, ''), berlaku_dari);

ALTER TABLE entri_harga ADD COLUMN IF NOT EXISTS flag_validasi JSONB NOT NULL DEFAULT '{}'::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE entri_harga DROP COLUMN IF EXISTS flag_validasi;
DROP INDEX IF EXISTS uq_batas_harga_berlaku;
DROP TABLE IF EXISTS batas_harga;
-- +goose StatementEnd
