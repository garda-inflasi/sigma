-- 16-B: KPI efisiensi biaya logistik membandingkan biaya rencana optimal dengan
-- rencana pembanding (serakah: pasangan terdekat dulu) untuk volume yang sama.
-- biaya_pembanding NULL = tidak sebanding (volume berbeda) atau rencana dibuat
-- sebelum kolom ini ada; baris itu diabaikan KPI. Tanpa data demo.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE optimasi_rute
    ADD COLUMN IF NOT EXISTS biaya_pembanding BIGINT CHECK (biaya_pembanding IS NULL OR biaya_pembanding >= 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE optimasi_rute DROP COLUMN IF EXISTS biaya_pembanding;
-- +goose StatementEnd
