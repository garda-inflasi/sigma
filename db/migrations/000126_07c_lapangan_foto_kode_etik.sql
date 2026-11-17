-- Slice 07-C (docs/todos/07-lapangan-pwa.md): lampiran foto entri lapangan, kode etik pencacah, dan view kinerja petugas.
-- Hak garda_aplikasi (000119) diberikan per skema (ALL TABLES), sehingga tabel dan view baru otomatis terjangkau.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS lampiran_entri (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entri_id BIGINT NOT NULL REFERENCES entri_harga(id) ON DELETE CASCADE,
    lokasi_objek VARCHAR(255) NOT NULL,
    mime VARCHAR(64) NOT NULL,
    ukuran INT NOT NULL CHECK (ukuran > 0),
    checksum VARCHAR(64) NOT NULL,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_lampiran_entri_entri_id ON lampiran_entri (entri_id);

CREATE TABLE IF NOT EXISTS kode_etik (
    versi VARCHAR(32) PRIMARY KEY,
    judul VARCHAR(255) NOT NULL,
    isi TEXT NOT NULL,
    aktif BOOLEAN NOT NULL DEFAULT true,
    dibuat_pada TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS persetujuan_kode_etik (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE,
    versi VARCHAR(32) NOT NULL REFERENCES kode_etik(versi) ON DELETE RESTRICT,
    disetujui_pada TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, versi)
);

CREATE INDEX IF NOT EXISTS idx_persetujuan_kode_etik_user_id ON persetujuan_kode_etik (user_id);

CREATE OR REPLACE VIEW kinerja_petugas AS
SELECT
    p.id AS petugas_id,
    p.nama AS nama_petugas,
    p.kabupaten_kode,
    COUNT(DISTINCT e.id)::int AS total_entri,
    COUNT(DISTINCT e.id) FILTER (WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI'))::int AS entri_valid,
    COUNT(DISTINCT e.id) FILTER (WHERE e.status = 'DITOLAK')::int AS entri_ditolak,
    CASE
        WHEN COUNT(DISTINCT e.id) FILTER (WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI', 'DITOLAK')) = 0 THEN 0::double precision
        ELSE ROUND(
            (COUNT(DISTINCT e.id) FILTER (WHERE e.status = 'DITOLAK')::numeric /
             COUNT(DISTINCT e.id) FILTER (WHERE e.status IN ('DIVALIDASI', 'DIPUBLIKASI', 'DITOLAK'))::numeric) * 100,
            2
        )::double precision
    END AS rasio_penolakan_pct,
    COUNT(DISTINCT t.id)::int AS total_tugas,
    COUNT(DISTINCT t.id) FILTER (WHERE t.status = 'SELESAI')::int AS tugas_selesai,
    COUNT(DISTINCT t.id) FILTER (WHERE t.status = 'SELESAI' AND (t.selesai_pada IS NULL OR t.selesai_pada <= t.tenggat))::int AS tugas_tepat_waktu,
    CASE
        WHEN COUNT(DISTINCT t.id) FILTER (WHERE t.status = 'SELESAI') = 0 THEN 100::double precision
        ELSE ROUND(
            (COUNT(DISTINCT t.id) FILTER (WHERE t.status = 'SELESAI' AND (t.selesai_pada IS NULL OR t.selesai_pada <= t.tenggat))::numeric /
             COUNT(DISTINCT t.id) FILTER (WHERE t.status = 'SELESAI')::numeric) * 100,
            2
        )::double precision
    END AS ketepatan_tenggat_pct
FROM pengguna p
LEFT JOIN entri_harga e ON e.petugas_id = p.id
LEFT JOIN tugas_lapangan t ON t.petugas_id = p.id
WHERE p.peran IN ('MAHASISWA', 'KOORDINATOR_WILAYAH')
  AND p.status = 'AKTIF'
GROUP BY p.id, p.nama, p.kabupaten_kode;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS kinerja_petugas;
DROP TABLE IF EXISTS persetujuan_kode_etik;
DROP TABLE IF EXISTS kode_etik;
DROP TABLE IF EXISTS lampiran_entri;
-- +goose StatementEnd
