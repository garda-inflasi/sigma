-- +goose Up
-- +goose StatementBegin

-- ── Cold storage: telemetri suhu ──────────────────────────────────────
-- Pembacaan suhu terakhir dari sensor fasilitas (°C). NULL = belum ada
-- telemetri, mis. fasilitas berstatus RENCANA yang belum dibangun.
ALTER TABLE cold_storage
    ADD COLUMN IF NOT EXISTS suhu_aktual_c DOUBLE PRECISION;

-- ── Stok: batas akhir masa simpan per lot ─────────────────────────────
-- Ditetapkan saat stok masuk (masa simpan komoditas dalam rantai dingin).
-- Menjadi dasar "sisa masa simpan" dan pengurutan prioritas penyaluran.
ALTER TABLE stok_cold_storage
    ADD COLUMN IF NOT EXISTS kedaluwarsa_pada TIMESTAMPTZ;

-- Lot yang sudah ada diisi dari masa simpan baku komoditas.
UPDATE stok_cold_storage s
   SET kedaluwarsa_pada = s.masuk_pada + (k.shelf_life_hari * INTERVAL '1 day')
  FROM komoditas k
 WHERE k.kode = s.komoditas_kode
   AND s.kedaluwarsa_pada IS NULL;

ALTER TABLE stok_cold_storage
    ALTER COLUMN kedaluwarsa_pada SET NOT NULL;

-- Antarmuka membaca risiko sebagai pecahan 0–1 (ditampilkan ×100 sebagai %),
-- jadi nilai di luar rentang itu harus ditolak di tingkat basis data.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_stok_cold_storage_risiko') THEN
        ALTER TABLE stok_cold_storage
            ADD CONSTRAINT chk_stok_cold_storage_risiko CHECK (risiko_food_loss BETWEEN 0 AND 1);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_stok_cold_storage_volume') THEN
        ALTER TABLE stok_cold_storage
            ADD CONSTRAINT chk_stok_cold_storage_volume CHECK (volume_ton >= 0);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_stok_cold_storage_fasilitas ON stok_cold_storage(cold_storage_id);
CREATE INDEX IF NOT EXISTS idx_stok_cold_storage_kedaluwarsa ON stok_cold_storage(kedaluwarsa_pada);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_stok_cold_storage_kedaluwarsa;
DROP INDEX IF EXISTS idx_stok_cold_storage_fasilitas;
ALTER TABLE stok_cold_storage DROP CONSTRAINT IF EXISTS chk_stok_cold_storage_volume;
ALTER TABLE stok_cold_storage DROP CONSTRAINT IF EXISTS chk_stok_cold_storage_risiko;
ALTER TABLE stok_cold_storage DROP COLUMN IF EXISTS kedaluwarsa_pada;
ALTER TABLE cold_storage DROP COLUMN IF EXISTS suhu_aktual_c;
-- +goose StatementEnd
