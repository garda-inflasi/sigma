-- 13-B: telemetri suhu/kelembapan per fasilitas dan transaksi stok masuk/keluar/pindah.
-- Lihat docs/todos/13-cold-storage.md. Tanpa data demo (seeder coldstorage.demo).

-- +goose Up
-- +goose StatementBegin

-- ── Fasilitas: batas alarm dan telemetri terakhir ─────────────────────
-- suhu_min_c/suhu_maks_c NULL = pakai suhu_target_c ± toleransi bawaan (aplikasi).
-- suhu_aktual_c, kelembapan_aktual_pct, telemetri_pada diturunkan dari baris
-- telemetri_suhu terbaru oleh aplikasi saat telemetri masuk.
ALTER TABLE cold_storage
    ADD COLUMN IF NOT EXISTS suhu_min_c DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS suhu_maks_c DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS kelembapan_aktual_pct DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS telemetri_pada TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_cold_storage_batas_suhu') THEN
        ALTER TABLE cold_storage
            ADD CONSTRAINT chk_cold_storage_batas_suhu
            CHECK (suhu_min_c IS NULL OR suhu_maks_c IS NULL OR suhu_min_c < suhu_maks_c);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_cold_storage_kelembapan') THEN
        ALTER TABLE cold_storage
            ADD CONSTRAINT chk_cold_storage_kelembapan
            CHECK (kelembapan_aktual_pct IS NULL OR kelembapan_aktual_pct BETWEEN 0 AND 100);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_cold_storage_kapasitas') THEN
        ALTER TABLE cold_storage
            ADD CONSTRAINT chk_cold_storage_kapasitas CHECK (kapasitas_ton > 0);
    END IF;
END $$;

-- ── Telemetri suhu ────────────────────────────────────────────────────
-- Satu baris = satu pembacaan. UNIQUE (fasilitas_id, waktu) membuat pengiriman
-- ulang oleh sensor idempoten. Partisi bulanan dan retensi belum diterapkan
-- (lihat docs/todos/13-cold-storage.md, 13-B).
CREATE TABLE IF NOT EXISTS telemetri_suhu (
    id BIGSERIAL PRIMARY KEY,
    fasilitas_id VARCHAR(255) NOT NULL REFERENCES cold_storage(id) ON DELETE CASCADE,
    waktu TIMESTAMPTZ NOT NULL,
    suhu_c DOUBLE PRECISION NOT NULL CHECK (suhu_c BETWEEN -80 AND 60),
    kelembapan_pct DOUBLE PRECISION CHECK (kelembapan_pct BETWEEN 0 AND 100),
    sumber VARCHAR(10) NOT NULL DEFAULT 'SENSOR' CHECK (sumber IN ('SENSOR', 'MANUAL', 'DEMO')),
    UNIQUE (fasilitas_id, waktu)
);

-- ── Stok: status lot ──────────────────────────────────────────────────
-- AKTIF: tersimpan; PRIORITAS_JUAL: ditandai untuk disalurkan lebih dulu;
-- HABIS: volume 0 setelah transaksi keluar (baris dipertahankan untuk jejak).
ALTER TABLE stok_cold_storage
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'AKTIF';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_stok_cold_storage_status') THEN
        ALTER TABLE stok_cold_storage
            ADD CONSTRAINT chk_stok_cold_storage_status CHECK (status IN ('AKTIF', 'PRIORITAS_JUAL', 'HABIS'));
    END IF;
END $$;

-- ── Transaksi stok ────────────────────────────────────────────────────
-- Buku besar perpindahan stok. lot_id = lot yang menjadi sumber/asal (untuk
-- MASUK: lot yang dibuat). lot_baru_id terisi pada PINDAH sebagian, yaitu lot
-- yang dibentuk di fasilitas tujuan.
CREATE TABLE IF NOT EXISTS transaksi_stok (
    id VARCHAR(64) PRIMARY KEY,
    lot_id VARCHAR(255) NOT NULL REFERENCES stok_cold_storage(id),
    lot_baru_id VARCHAR(255) REFERENCES stok_cold_storage(id),
    jenis VARCHAR(10) NOT NULL CHECK (jenis IN ('MASUK', 'KELUAR', 'PINDAH')),
    volume_ton DOUBLE PRECISION NOT NULL CHECK (volume_ton > 0),
    fasilitas_asal_id VARCHAR(255) REFERENCES cold_storage(id),
    fasilitas_tujuan_id VARCHAR(255) REFERENCES cold_storage(id),
    oleh_id VARCHAR(255) REFERENCES pengguna(id) ON DELETE SET NULL,
    waktu TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alasan TEXT NOT NULL DEFAULT '',
    sumber VARCHAR(10) NOT NULL DEFAULT 'APLIKASI' CHECK (sumber IN ('APLIKASI', 'DEMO')),
    CONSTRAINT chk_transaksi_stok_lokasi CHECK (
        (jenis = 'MASUK' AND fasilitas_tujuan_id IS NOT NULL AND fasilitas_asal_id IS NULL)
        OR (jenis = 'KELUAR' AND fasilitas_asal_id IS NOT NULL AND fasilitas_tujuan_id IS NULL)
        OR (jenis = 'PINDAH' AND fasilitas_asal_id IS NOT NULL AND fasilitas_tujuan_id IS NOT NULL
            AND fasilitas_asal_id <> fasilitas_tujuan_id)
    )
);

CREATE INDEX IF NOT EXISTS idx_transaksi_stok_lot ON transaksi_stok (lot_id, waktu DESC);
CREATE INDEX IF NOT EXISTS idx_transaksi_stok_asal ON transaksi_stok (fasilitas_asal_id, waktu DESC);
CREATE INDEX IF NOT EXISTS idx_transaksi_stok_tujuan ON transaksi_stok (fasilitas_tujuan_id, waktu DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Down menghapus tabel telemetri dan transaksi beserta isinya: lakukan hanya
-- pada lingkungan yang datanya boleh hilang, atau setelah dicadangkan.
DROP TABLE IF EXISTS transaksi_stok;
DROP TABLE IF EXISTS telemetri_suhu;
ALTER TABLE stok_cold_storage DROP CONSTRAINT IF EXISTS chk_stok_cold_storage_status;
ALTER TABLE stok_cold_storage DROP COLUMN IF EXISTS status;
ALTER TABLE cold_storage DROP CONSTRAINT IF EXISTS chk_cold_storage_kapasitas;
ALTER TABLE cold_storage DROP CONSTRAINT IF EXISTS chk_cold_storage_kelembapan;
ALTER TABLE cold_storage DROP CONSTRAINT IF EXISTS chk_cold_storage_batas_suhu;
ALTER TABLE cold_storage
    DROP COLUMN IF EXISTS telemetri_pada,
    DROP COLUMN IF EXISTS kelembapan_aktual_pct,
    DROP COLUMN IF EXISTS suhu_maks_c,
    DROP COLUMN IF EXISTS suhu_min_c;
-- +goose StatementEnd
