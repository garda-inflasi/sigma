-- Slice 04-B (docs/todos/04-master-data.md): versioning dan persetujuan perubahan master data, serta kecamatan.
--
-- perubahan_master menjadi antrean persetujuan yang dipakai aplikasi:
--   * jenis: TAMBAH (entitas baru), UBAH (isi bidang), TINJAU (status saja, dari aksi massal "ajukan perubahan"
--     tanpa isi). Baris lama (sebelum migrasi ini) diberi UBAH.
--   * perubahan: nilai baru per bidang (JSON objek); sebelum: nilai bidang yang sama saat diajukan (untuk diff).
--   * versi: versi entitas yang berlaku bila usulan disetujui (versi saat diajukan + 1). Persetujuan ditolak bila
--     versi entitas sudah berubah sejak diajukan (usulan kedaluwarsa).
--   * alasan (pengaju), alasan_penolakan, disetujui_pada, ditolak_oleh_id, ditolak_pada. DITOLAK wajib beralasan.
--     CHECK baru NOT VALID: baris lama tidak diperiksa ulang.
--   * Satu usulan MENUNGGU per entitas (indeks unik parsial uq_perubahan_master_menunggu_entitas), sehingga dua
--     usulan tidak saling menimpa.
--   Referensi entitas tetap polimorfik (entitas + entitas_id tanpa FK): usulan TAMBAH merujuk kode yang belum ada.
--
-- kecamatan: kode BPS 7 digit (PK KODE), diimpor dari BPS lewat skrip (seeder master.reference), bukan migrasi.
-- pasar.kecamatan_kode (opsional) melengkapi hierarki provinsi > kabupaten > kecamatan > pasar. pasar.versi
-- mengikuti komoditas.versi: naik setiap usulan pasar disetujui.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE perubahan_master
    ADD COLUMN IF NOT EXISTS jenis varchar(10) NOT NULL DEFAULT 'UBAH',
    ADD COLUMN IF NOT EXISTS sebelum jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS alasan text,
    ADD COLUMN IF NOT EXISTS alasan_penolakan text,
    ADD COLUMN IF NOT EXISTS disetujui_pada timestamptz,
    ADD COLUMN IF NOT EXISTS ditolak_oleh_id uuid REFERENCES pengguna(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS ditolak_pada timestamptz;

ALTER TABLE perubahan_master
    ADD CONSTRAINT chk_perubahan_master_jenis CHECK (jenis IN ('TAMBAH', 'UBAH', 'TINJAU'));
-- NOT VALID: berlaku untuk baris baru; baris lama (data contoh sebelum F1) tidak diperiksa ulang.
ALTER TABLE perubahan_master
    ADD CONSTRAINT chk_perubahan_master_perubahan_objek CHECK (jsonb_typeof(perubahan) = 'object') NOT VALID,
    ADD CONSTRAINT chk_perubahan_master_versi CHECK (versi > 0) NOT VALID,
    ADD CONSTRAINT chk_perubahan_master_alasan_penolakan
        CHECK (status <> 'DITOLAK' OR alasan_penolakan IS NOT NULL) NOT VALID;

CREATE INDEX IF NOT EXISTS idx_perubahan_master_ditolak_oleh_id ON perubahan_master (ditolak_oleh_id);
-- Satu usulan menunggu per entitas. Bila data lama sudah melanggarnya, indeks gagal dibuat dan migrasi berhenti
-- dengan pesan Postgres yang menyebut entitasnya; selesaikan dulu usulan gandanya.
CREATE UNIQUE INDEX IF NOT EXISTS uq_perubahan_master_menunggu_entitas
    ON perubahan_master (entitas, entitas_id) WHERE status = 'MENUNGGU';

COMMENT ON COLUMN perubahan_master.versi IS 'Versi entitas yang berlaku bila usulan disetujui (versi saat diajukan + 1).';
COMMENT ON COLUMN perubahan_master.perubahan IS 'Nilai baru per bidang (objek JSON); kosong untuk jenis TINJAU.';
COMMENT ON COLUMN perubahan_master.sebelum IS 'Nilai bidang yang sama saat diajukan, untuk diff.';

CREATE TABLE IF NOT EXISTS kecamatan (
    kode           varchar(7) PRIMARY KEY,
    nama           varchar(255) NOT NULL,
    kabupaten_kode varchar(4) NOT NULL REFERENCES kabupaten(kode) ON DELETE RESTRICT,
    kode_dagri     varchar(10),
    CONSTRAINT chk_kecamatan_kode CHECK (kode ~ '^[0-9]{7}$'),
    CONSTRAINT chk_kecamatan_kabupaten CHECK (left(kode, 4) = kabupaten_kode)
);
CREATE INDEX IF NOT EXISTS idx_kecamatan_kabupaten_kode ON kecamatan (kabupaten_kode);
COMMENT ON TABLE kecamatan IS 'Kecamatan (kode BPS 7 digit); diimpor dari BPS lewat seeder master.reference.';
COMMENT ON COLUMN kecamatan.kode_dagri IS 'Kode Kemendagri padanan (dari API bridging BPS), untuk pencocokan sumber lain.';

ALTER TABLE pasar
    ADD COLUMN IF NOT EXISTS kecamatan_kode varchar(7) REFERENCES kecamatan(kode) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS versi integer NOT NULL DEFAULT 1;
ALTER TABLE pasar
    ADD CONSTRAINT chk_pasar_versi CHECK (versi > 0),
    ADD CONSTRAINT chk_pasar_kecamatan_kabupaten CHECK (kecamatan_kode IS NULL OR left(kecamatan_kode, 4) = kabupaten_kode);
CREATE INDEX IF NOT EXISTS idx_pasar_kecamatan_kode ON pasar (kecamatan_kode);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pasar
    DROP CONSTRAINT IF EXISTS chk_pasar_kecamatan_kabupaten,
    DROP CONSTRAINT IF EXISTS chk_pasar_versi;
DROP INDEX IF EXISTS idx_pasar_kecamatan_kode;
ALTER TABLE pasar
    DROP COLUMN IF EXISTS versi,
    DROP COLUMN IF EXISTS kecamatan_kode;
DROP TABLE IF EXISTS kecamatan;

DROP INDEX IF EXISTS uq_perubahan_master_menunggu_entitas;
DROP INDEX IF EXISTS idx_perubahan_master_ditolak_oleh_id;
ALTER TABLE perubahan_master
    DROP CONSTRAINT IF EXISTS chk_perubahan_master_alasan_penolakan,
    DROP CONSTRAINT IF EXISTS chk_perubahan_master_versi,
    DROP CONSTRAINT IF EXISTS chk_perubahan_master_perubahan_objek,
    DROP CONSTRAINT IF EXISTS chk_perubahan_master_jenis,
    DROP COLUMN IF EXISTS ditolak_pada,
    DROP COLUMN IF EXISTS ditolak_oleh_id,
    DROP COLUMN IF EXISTS disetujui_pada,
    DROP COLUMN IF EXISTS alasan_penolakan,
    DROP COLUMN IF EXISTS alasan,
    DROP COLUMN IF EXISTS sebelum,
    DROP COLUMN IF EXISTS jenis;
-- +goose StatementEnd
