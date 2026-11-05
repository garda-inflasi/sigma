-- Slice 15-A (docs/todos/15-policy-brief.md): policy_brief mendapat waktu
-- pembuatan. Kolom diperbarui yang sudah ada tetap menjadi waktu ubah terakhir.
-- Baris lama (bila ada) memakai diperbarui sebagai nilai awal dibuat_pada.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE policy_brief ADD COLUMN IF NOT EXISTS dibuat_pada TIMESTAMPTZ;
UPDATE policy_brief SET dibuat_pada = diperbarui WHERE dibuat_pada IS NULL;
ALTER TABLE policy_brief ALTER COLUMN dibuat_pada SET DEFAULT NOW();
ALTER TABLE policy_brief ALTER COLUMN dibuat_pada SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE policy_brief DROP COLUMN IF EXISTS dibuat_pada;
-- +goose StatementEnd
