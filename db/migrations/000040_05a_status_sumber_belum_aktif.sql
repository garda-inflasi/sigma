-- Slice 05-A (docs/todos/05-sumber-data.md): sumber yang baru dikatalogkan dan belum
-- pernah dijalankan tidak boleh tampil SEHAT. Nilai enum baru dipisah dari migrasi
-- yang memakainya (000041) dan dijalankan di luar transaksi.
--
-- Down membuat ulang enum tanpa nilai itu (PostgreSQL tidak bisa menghapus nilai
-- enum); baris BELUM_AKTIF dipetakan ke TERTUNDA.

-- +goose NO TRANSACTION
-- +goose Up
-- +goose StatementBegin
ALTER TYPE status_sumber ADD VALUE IF NOT EXISTS 'BELUM_AKTIF';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sumber_data ALTER COLUMN status DROP DEFAULT;
ALTER TYPE status_sumber RENAME TO status_sumber_lama;
CREATE TYPE status_sumber AS ENUM ('SEHAT', 'TERTUNDA', 'GAGAL', 'DIJEDA');
ALTER TABLE sumber_data ALTER COLUMN status TYPE status_sumber
    USING (CASE WHEN status::text = 'BELUM_AKTIF' THEN 'TERTUNDA' ELSE status::text END)::status_sumber;
ALTER TABLE sumber_data ALTER COLUMN status SET DEFAULT 'SEHAT';
DROP TYPE status_sumber_lama;
-- +goose StatementEnd
