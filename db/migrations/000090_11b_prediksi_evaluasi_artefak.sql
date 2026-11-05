-- Slice 11-B (docs/todos/11-prediksi-xai.md): model prediksi pertama.
--   * prediksi_evaluasi: galat nyata sebuah proyeksi setelah tanggal targetnya
--     lewat (realisasi = harga konsumen rata-rata hari itu). Dasar MAPE
--     realisasi per model; satu evaluasi per proyeksi.
--   * model_registry.artefak_uri: penanda implementasi/artefak yang menghasilkan
--     model (layanan ML dan versinya); catatan: keterangan pelatihan.
-- Tidak ada data di sini; baris prediksi_harga/model_registry bersumber 'MODEL'
-- ditulis API saat pelatihan.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE model_registry ADD COLUMN IF NOT EXISTS artefak_uri TEXT;
ALTER TABLE model_registry ADD COLUMN IF NOT EXISTS catatan TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS prediksi_evaluasi (
    prediksi_id VARCHAR(255) PRIMARY KEY REFERENCES prediksi_harga(id) ON DELETE CASCADE,
    realisasi INT NOT NULL,
    -- (nilai - realisasi) / realisasi * 100: positif = proyeksi terlalu tinggi.
    galat_pct NUMERIC(8, 2) NOT NULL,
    dievaluasi_pada TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_prediksi_evaluasi_realisasi CHECK (realisasi > 0)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS prediksi_evaluasi;
ALTER TABLE model_registry DROP COLUMN IF EXISTS catatan;
ALTER TABLE model_registry DROP COLUMN IF EXISTS artefak_uri;
-- +goose StatementEnd
