-- Jembatan EWS pipeline ML (docs/PIPELINE-DATA-ML.md §5.4): sinyal aktif `datasafe_ews_signals` (MongoDB, hanya dibaca)
-- diubah job worker `ml_jembatan_ews` menjadi alert `ews_alert` agar ikut alur akui/eskalasi/selesai, SSE, dan
-- notifikasi yang sudah ada.
--
-- - ews_alert.kabupaten_kode boleh NULL: sinyal tingkat sistem (SUMBER_DATA, kegagalan crawler) dan tingkat provinsi
--   (TEKANAN_INFLASI wilayah `11`) tidak punya kabupaten. Alert seperti itu hanya terlihat oleh peran tingkat
--   provinsi (cakupan baris 02-B memakai kecocokan kabupaten, sehingga NULL tidak pernah lolos untuk peran
--   berkabupaten). FK ke kabupaten tetap berlaku untuk nilai yang terisi.
-- - ews_sinyal_ml: keadaan jembatan per `sinyal_kunci` (stabil lintas siklus). Siklus yang sudah diterapkan
--   untuk satu kunci tidak diterapkan ulang, sehingga job yang berjalan tiap beberapa menit tidak membuat alert
--   ganda dalam satu siklus; alert aktif per kondisi tetap dijaga indeks unik parsial ews_alert.dedup_key.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE ews_alert ALTER COLUMN kabupaten_kode DROP NOT NULL;

CREATE TABLE IF NOT EXISTS ews_sinyal_ml (
    sinyal_kunci    VARCHAR(255) PRIMARY KEY,
    jenis_ml        VARCHAR(32)  NOT NULL,
    severity        severity     NOT NULL,
    aktif           BOOLEAN      NOT NULL,
    cycle_id        VARCHAR(32)  NOT NULL,
    cycle_timestamp TIMESTAMPTZ  NOT NULL,
    alert_id        BIGINT REFERENCES ews_alert(id) ON DELETE SET NULL,
    diperbarui_pada TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ews_sinyal_ml_alert ON ews_sinyal_ml (alert_id);

COMMENT ON TABLE ews_sinyal_ml IS 'Keadaan jembatan sinyal datasafe_ews_signals -> ews_alert per sinyal_kunci (job ml_jembatan_ews).';
COMMENT ON COLUMN ews_sinyal_ml.cycle_id IS 'Siklus ML terakhir yang sudah diterapkan untuk kunci ini (C{YYYYMMDD}T{HH}Z).';
COMMENT ON COLUMN ews_sinyal_ml.alert_id IS 'Alert terakhir yang dibuat atau diperbarui untuk kunci ini; NULL bila belum pernah menjadi alert.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ews_sinyal_ml;

-- Alert tanpa kabupaten tidak dapat dikembalikan ke NOT NULL tanpa mengarang wilayah. Rollback dihentikan dengan
-- pesan yang jelas; tutup dan arsipkan alert tersebut secara manual bila rollback memang diperlukan.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM ews_alert WHERE kabupaten_kode IS NULL) THEN
        RAISE EXCEPTION 'ews_alert memuat alert tanpa kabupaten (tingkat sistem/provinsi dari pipeline ML); rollback 000131 dihentikan';
    END IF;
END $$;
ALTER TABLE ews_alert ALTER COLUMN kabupaten_kode SET NOT NULL;
-- +goose StatementEnd
