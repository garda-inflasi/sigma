-- Slice 10-C (docs/todos/10-ews.md): aliran real-time EWS (SSE).
-- ews_event adalah outbox peristiwa alert (dibuat atau naik tingkat). Id berurut menjadi
-- Last-Event-ID, sehingga klien yang menyambung ulang mengejar peristiwa yang terlewat
-- tanpa duplikat, dan antar-instance cukup membaca tabel ini (Redis hanya
-- membangunkan pembaca, bukan sumber kebenaran).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS ews_event (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    alert_id VARCHAR(255) NOT NULL REFERENCES ews_alert(id) ON DELETE CASCADE,
    tipe VARCHAR(20) NOT NULL DEFAULT 'alert',
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_ews_event_tipe CHECK (tipe IN ('alert'))
);
CREATE INDEX IF NOT EXISTS idx_ews_event_dibuat_pada ON ews_event (dibuat_pada);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ews_event;
-- +goose StatementEnd
