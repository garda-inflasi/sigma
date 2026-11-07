-- Slice 03-C (docs/todos/03-audit.md): data lineage per record harga (PRD §1.4) dan versi entri
-- (backlog docs/todos/06-harga-validasi.md).
--
-- entri_harga_versi menyimpan satu baris tiap kali isi sebuah entri harga berubah: siapa (oleh), kapan (waktu),
-- apa yang dilakukan (aksi), status dan harga saat itu, kolom mana yang berubah (perubahan), dan salinan lengkap
-- barisnya (data = to_jsonb(entri_harga)). Versi ditulis oleh pemicu AFTER INSERT OR UPDATE pada entri_harga,
-- bukan oleh aplikasi, jadi semua jalur tulis (API, seeder, perbaikan data lewat SQL) tercatat dan tidak ada
-- yang bisa lupa memanggilnya.
--
-- Kunci: id bigint identity (log transaksional, SKEMA-DATABASE §2). entri_id bigint REFERENCES entri_harga(id)
-- ON DELETE CASCADE: versi hanya berarti bersama entrinya. UNIQUE (entri_id, versi) menjaga nomor urut per entri
-- dan sekaligus menjadi indeks FK (kolom awalnya entri_id). oleh uuid REFERENCES pengguna(id) ON DELETE RESTRICT
-- (pelaku data harga adalah bukti asal data; sama dengan riwayat_status_entri.oleh) dengan idx_entri_harga_versi_oleh.
--
-- Aksi: INPUT (baris baru), AJUKAN, VALIDASI, PUBLIKASI, TOLAK, KEMBALIKAN (menurut status tujuan), UBAH (isi berubah
-- tanpa pindah status), MIGRASI (versi 1 untuk baris yang sudah ada saat migrasi ini dijalankan; pelaku tidak diketahui).
--
-- Pelaku (oleh): GUC lokal-transaksi garda.aktor_id bila diisi aplikasi, berbentuk UUID, dan ada di pengguna (aplikasi
-- mengisinya di awal transaksi aksi massal, sehingga KEMBALIKAN, yang tidak punya kolom pelaku, tetap tercatat).
-- Selain itu diturunkan dari baris: INPUT dan AJUKAN petugas_id, VALIDASI divalidasi_oleh, PUBLIKASI dipublikasi_oleh,
-- TOLAK ditolak_oleh, lainnya NULL. Nilai GUC yang rusak tidak pernah membatalkan transaksi bisnis: bentuknya
-- diperiksa dulu sebelum dikonversi.
--
-- Kolom yang diabaikan saat menilai perubahan: seluruh kolom waktu (dicatat_pada, dibuat_pada, diupdate_pada,
-- divalidasi_pada, dipublikasi_pada, ditolak_pada). Aplikasi tidak pernah mengubahnya sendirian: setiap transisi
-- status mengubah status dan kolom pelaku, jadi tetap menghasilkan versi, dan salinan lengkapnya memuat waktunya.
-- Hanya seeder demo yang menggeser waktu saja (tanggal relatif terhadap now) pada baris yang isinya sama; tanpa
-- pengabaian ini tiap `make db/seed/demo` menambah puluhan ribu versi. UPDATE yang hanya menyentuh kolom waktu
-- tidak membuat versi; kolom waktu itu tercatat di versi berikutnya. perubahan memuat kolom non-waktu yang berubah.
--
-- Append-only: UPDATE dan DELETE langsung ditolak (restrict_violation), TRUNCATE juga. Satu-satunya penghapusan
-- yang sah adalah ON DELETE CASCADE dari entri_harga (pg_trigger_depth() > 1), pola yang sama dengan audit_log.
--
-- Backfill: satu versi (versi 1, aksi MIGRASI) untuk tiap entri yang sudah ada. Pada basis data kosong tidak ada
-- yang ditulis, jadi seluruh tabel tetap kosong setelah migrasi (gerbang produksi F1).
--
-- Down membuang pemicu, fungsi, dan tabel (seluruh versi hilang; entri_harga tidak tersentuh).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS entri_harga_versi (
    id        bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entri_id  bigint      NOT NULL REFERENCES entri_harga(id) ON DELETE CASCADE,
    versi     integer     NOT NULL CHECK (versi > 0),
    aksi      varchar(20) NOT NULL,
    status    status_entri NOT NULL,
    harga     integer     NOT NULL,
    oleh      uuid        REFERENCES pengguna(id) ON DELETE RESTRICT,
    waktu     timestamptz NOT NULL DEFAULT now(),
    perubahan text[]      NOT NULL DEFAULT '{}',
    data      jsonb       NOT NULL,
    CONSTRAINT uq_entri_harga_versi_entri_versi UNIQUE (entri_id, versi),
    CONSTRAINT chk_entri_harga_versi_aksi CHECK (
        aksi IN ('INPUT', 'AJUKAN', 'VALIDASI', 'PUBLIKASI', 'TOLAK', 'KEMBALIKAN', 'UBAH', 'MIGRASI'))
);

CREATE INDEX IF NOT EXISTS idx_entri_harga_versi_oleh ON entri_harga_versi (oleh);

-- Satu versi untuk tiap entri yang sudah ada; tidak melakukan apa pun pada basis data kosong.
INSERT INTO entri_harga_versi (entri_id, versi, aksi, status, harga, oleh, waktu, perubahan, data)
SELECT e.id, 1, 'MIGRASI', e.status, e.harga, NULL, e.diupdate_pada, '{}', to_jsonb(e)
  FROM entri_harga e
 ORDER BY e.id
ON CONFLICT (entri_id, versi) DO NOTHING;

CREATE OR REPLACE FUNCTION entri_harga_catat_versi() RETURNS trigger AS $$
DECLARE
    -- Kolom waktu: pergeseran tanpa perubahan isi bukan versi baru (lihat header migrasi).
    abaikan     constant text[] := ARRAY['dicatat_pada', 'dibuat_pada', 'diupdate_pada',
                                         'divalidasi_pada', 'dipublikasi_pada', 'ditolak_pada'];
    v_perubahan text[]  := '{}';
    v_aksi      text;
    v_oleh      uuid;
    v_aktor     text;
    v_versi     integer;
BEGIN
    IF TG_OP = 'INSERT' THEN
        v_aksi := 'INPUT';
    ELSE
        SELECT coalesce(array_agg(n.key ORDER BY n.key), '{}')
          INTO v_perubahan
          FROM jsonb_each(to_jsonb(NEW)) AS n(key, value)
          LEFT JOIN jsonb_each(to_jsonb(OLD)) AS o(key, value) ON o.key = n.key
         WHERE n.value IS DISTINCT FROM o.value
           AND n.key <> ALL (abaikan);
        IF cardinality(v_perubahan) = 0 THEN
            RETURN NULL;
        END IF;

        IF NEW.status IS DISTINCT FROM OLD.status THEN
            v_aksi := CASE NEW.status
                WHEN 'DIAJUKAN'    THEN 'AJUKAN'
                WHEN 'DIVALIDASI'  THEN 'VALIDASI'
                WHEN 'DIPUBLIKASI' THEN 'PUBLIKASI'
                WHEN 'DITOLAK'     THEN 'TOLAK'
                WHEN 'DRAF'        THEN 'KEMBALIKAN'
                ELSE 'UBAH'
            END;
        ELSE
            v_aksi := 'UBAH';
        END IF;
    END IF;

    -- Pelaku: GUC dari aplikasi bila sah, selain itu diturunkan dari baris.
    v_aktor := nullif(current_setting('garda.aktor_id', true), '');
    IF v_aktor IS NOT NULL
       AND v_aktor ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
        SELECT p.id INTO v_oleh FROM pengguna p WHERE p.id = v_aktor::uuid;
    END IF;
    IF v_oleh IS NULL THEN
        v_oleh := CASE v_aksi
            WHEN 'INPUT'     THEN NEW.petugas_id
            WHEN 'AJUKAN'    THEN NEW.petugas_id
            WHEN 'VALIDASI'  THEN NEW.divalidasi_oleh
            WHEN 'PUBLIKASI' THEN NEW.dipublikasi_oleh
            WHEN 'TOLAK'     THEN NEW.ditolak_oleh
            ELSE NULL
        END;
    END IF;

    -- Aman tanpa kunci tambahan: UPDATE memegang kunci baris entri, INSERT adalah baris baru.
    -- UNIQUE (entri_id, versi) tetap menjadi pengaman terakhir.
    SELECT coalesce(max(v.versi), 0) + 1 INTO v_versi FROM entri_harga_versi v WHERE v.entri_id = NEW.id;

    INSERT INTO entri_harga_versi (entri_id, versi, aksi, status, harga, oleh, waktu, perubahan, data)
    VALUES (NEW.id, v_versi, v_aksi, NEW.status, NEW.harga, v_oleh, now(), v_perubahan, to_jsonb(NEW));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_entri_harga_versi ON entri_harga;
CREATE TRIGGER trg_entri_harga_versi
    AFTER INSERT OR UPDATE ON entri_harga
    FOR EACH ROW EXECUTE FUNCTION entri_harga_catat_versi();

CREATE OR REPLACE FUNCTION entri_harga_versi_tolak_perubahan() RETURNS trigger AS $$
BEGIN
    -- Satu-satunya penghapusan yang sah: ON DELETE CASCADE dari entri_harga (dijalankan oleh pemicu FK).
    IF TG_OP = 'DELETE' AND pg_trigger_depth() > 1 THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'entri_harga_versi bersifat append-only: % ditolak', TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_entri_harga_versi_append_only ON entri_harga_versi;
CREATE TRIGGER trg_entri_harga_versi_append_only
    BEFORE UPDATE OR DELETE ON entri_harga_versi
    FOR EACH ROW EXECUTE FUNCTION entri_harga_versi_tolak_perubahan();

DROP TRIGGER IF EXISTS trg_entri_harga_versi_no_truncate ON entri_harga_versi;
CREATE TRIGGER trg_entri_harga_versi_no_truncate
    BEFORE TRUNCATE ON entri_harga_versi
    FOR EACH STATEMENT EXECUTE FUNCTION entri_harga_versi_tolak_perubahan();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_entri_harga_versi ON entri_harga;
DROP FUNCTION IF EXISTS entri_harga_catat_versi();
DROP TRIGGER IF EXISTS trg_entri_harga_versi_no_truncate ON entri_harga_versi;
DROP TRIGGER IF EXISTS trg_entri_harga_versi_append_only ON entri_harga_versi;
DROP FUNCTION IF EXISTS entri_harga_versi_tolak_perubahan();
DROP TABLE IF EXISTS entri_harga_versi;
-- +goose StatementEnd
