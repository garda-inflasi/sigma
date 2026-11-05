-- Slice 07-B (docs/todos/07-lapangan-pwa.md): kiriman harga dari lapangan.
-- - kunci idempotensi unik per petugas (dua petugas boleh memakai kunci yang sama);
-- - tugas_entri menautkan entri ke tugas supaya progres dihitung dari entri yang masuk;
-- - indeks untuk deteksi bentrok antarpetugas pada pasar, komoditas, dan tipe yang sama.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE entri_harga DROP CONSTRAINT IF EXISTS entri_harga_idempotency_key_key;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_entri_harga_petugas_kunci') THEN
        ALTER TABLE entri_harga
            ADD CONSTRAINT uq_entri_harga_petugas_kunci UNIQUE (petugas_id, idempotency_key);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_entri_harga_bentrok ON entri_harga(pasar_id, komoditas_kode, tipe, dicatat_pada);

CREATE TABLE IF NOT EXISTS tugas_entri (
    tugas_id VARCHAR(255) NOT NULL REFERENCES tugas_lapangan(id) ON DELETE CASCADE,
    entri_id VARCHAR(255) NOT NULL REFERENCES entri_harga(id) ON DELETE CASCADE,
    ditautkan_pada TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tugas_id, entri_id)
);
CREATE INDEX IF NOT EXISTS idx_tugas_entri_entri ON tugas_entri(entri_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tugas_entri;
DROP INDEX IF EXISTS idx_entri_harga_bentrok;
ALTER TABLE entri_harga DROP CONSTRAINT IF EXISTS uq_entri_harga_petugas_kunci;

-- Pemulihan batas unik global gagal bila dua petugas telah memakai kunci yang sama.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'entri_harga_idempotency_key_key') THEN
        ALTER TABLE entri_harga ADD CONSTRAINT entri_harga_idempotency_key_key UNIQUE (idempotency_key);
    END IF;
END $$;
-- +goose StatementEnd
