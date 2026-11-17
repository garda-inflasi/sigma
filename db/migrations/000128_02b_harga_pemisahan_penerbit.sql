-- Slice 02-B (docs/todos/02-pengguna-rbac.md): pemisahan tugas penerbit entri harga.
-- Pencatat, validator, dan penerbit harus tiga orang berbeda. CHECK chk_entri_harga_pemisahan_tugas
-- (000061/000110) hanya melarang validator = pencatat; di sini penerbit tidak boleh pencatat maupun validator.
-- NOT VALID: baris lama tidak diperiksa ulang, tetapi setiap tulis baru wajib patuh (pola yang sama dengan
-- chk_entri_harga_pemisahan_tugas, sehingga migrasi aman pada basis data berisi riwayat).

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_entri_harga_pemisahan_penerbit') THEN
        ALTER TABLE entri_harga ADD CONSTRAINT chk_entri_harga_pemisahan_penerbit
            CHECK (
                dipublikasi_oleh IS NULL
                OR (dipublikasi_oleh IS DISTINCT FROM petugas_id AND dipublikasi_oleh IS DISTINCT FROM divalidasi_oleh)
            ) NOT VALID;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE entri_harga DROP CONSTRAINT IF EXISTS chk_entri_harga_pemisahan_penerbit;
-- +goose StatementEnd
