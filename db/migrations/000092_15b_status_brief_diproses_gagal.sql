-- Slice 15-B (docs/todos/15-policy-brief.md): penyusunan brief berjalan asinkron,
-- jadi brief punya status DIPROSES (sedang disusun) dan GAGAL (penyusunan gagal,
-- dapat diulang). Nilai enum baru dipisah dari migrasi yang memakainya (000093)
-- dan dijalankan di luar transaksi.
--
-- Down membuat ulang enum tanpa kedua nilai (PostgreSQL tidak bisa menghapus nilai
-- enum); baris DIPROSES/GAGAL dipetakan ke DRAF.

-- +goose NO TRANSACTION
-- +goose Up
-- +goose StatementBegin
ALTER TYPE status_brief ADD VALUE IF NOT EXISTS 'DIPROSES';
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TYPE status_brief ADD VALUE IF NOT EXISTS 'GAGAL';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE policy_brief ALTER COLUMN status DROP DEFAULT;
ALTER TYPE status_brief RENAME TO status_brief_lama;
CREATE TYPE status_brief AS ENUM ('DRAF', 'TELAAH', 'DISETUJUI', 'DIDISTRIBUSIKAN');
ALTER TABLE policy_brief ALTER COLUMN status TYPE status_brief
    USING (CASE WHEN status::text IN ('DIPROSES', 'GAGAL') THEN 'DRAF' ELSE status::text END)::status_brief;
ALTER TABLE policy_brief ALTER COLUMN status SET DEFAULT 'DRAF';
DROP TYPE status_brief_lama;
-- +goose StatementEnd
