-- Slice 15-B (docs/todos/15-policy-brief.md): generator brief dari data nyata dan LLM.
--   * policy_brief.konten: narasi per bagian (JSON) beserta lampiran sitasi sumber;
--     snapshot_data: fakta (angka dasar) yang dikumpulkan saat penyusunan, bahan
--     verifikasi bahwa angka di narasi cocok dengan data sumber.
--   * sumber_narasi: LLM, TEMPLAT (deterministik), atau DEMO (brief contoh seeder);
--     model_llm/prompt_versi/prompt_hash: jejak model dan prompt bila LLM dipakai;
--     galat: catatan kegagalan penyusunan, atau alasan LLM diganti templat.
--   * brief_job: satu baris per percobaan penyusunan (status, waktu, galat).
--   * policy_brief_versi: riwayat isi brief, satu baris per versi.
-- Nilai status DIPROSES/GAGAL ada di migrasi 000092. Tidak ada data di sini.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE policy_brief ADD COLUMN IF NOT EXISTS konten JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE policy_brief ADD COLUMN IF NOT EXISTS snapshot_data JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE policy_brief ADD COLUMN IF NOT EXISTS sumber_narasi VARCHAR(10) NOT NULL DEFAULT 'TEMPLAT';
ALTER TABLE policy_brief ADD COLUMN IF NOT EXISTS model_llm VARCHAR(100);
ALTER TABLE policy_brief ADD COLUMN IF NOT EXISTS prompt_versi VARCHAR(50);
ALTER TABLE policy_brief ADD COLUMN IF NOT EXISTS prompt_hash VARCHAR(64);
ALTER TABLE policy_brief ADD COLUMN IF NOT EXISTS galat TEXT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_policy_brief_sumber_narasi') THEN
        ALTER TABLE policy_brief ADD CONSTRAINT chk_policy_brief_sumber_narasi
            CHECK (sumber_narasi IN ('LLM', 'TEMPLAT', 'DEMO'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS brief_job (
    id VARCHAR(255) PRIMARY KEY,
    brief_id VARCHAR(255) NOT NULL REFERENCES policy_brief(id),
    status VARCHAR(10) NOT NULL,
    mulai TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    selesai TIMESTAMPTZ,
    galat TEXT,
    CONSTRAINT chk_brief_job_status CHECK (status IN ('BERJALAN', 'SELESAI', 'GAGAL'))
);
CREATE INDEX IF NOT EXISTS idx_brief_job_brief ON brief_job (brief_id, mulai DESC);

CREATE TABLE IF NOT EXISTS policy_brief_versi (
    brief_id VARCHAR(255) NOT NULL REFERENCES policy_brief(id),
    versi INT NOT NULL,
    konten JSONB NOT NULL,
    oleh VARCHAR(255) NOT NULL REFERENCES pengguna(id),
    waktu TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (brief_id, versi),
    CONSTRAINT chk_policy_brief_versi_versi CHECK (versi > 0)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS policy_brief_versi;
DROP TABLE IF EXISTS brief_job;
ALTER TABLE policy_brief DROP CONSTRAINT IF EXISTS chk_policy_brief_sumber_narasi;
ALTER TABLE policy_brief DROP COLUMN IF EXISTS galat;
ALTER TABLE policy_brief DROP COLUMN IF EXISTS prompt_hash;
ALTER TABLE policy_brief DROP COLUMN IF EXISTS prompt_versi;
ALTER TABLE policy_brief DROP COLUMN IF EXISTS model_llm;
ALTER TABLE policy_brief DROP COLUMN IF EXISTS sumber_narasi;
ALTER TABLE policy_brief DROP COLUMN IF EXISTS snapshot_data;
ALTER TABLE policy_brief DROP COLUMN IF EXISTS konten;
-- +goose StatementEnd
