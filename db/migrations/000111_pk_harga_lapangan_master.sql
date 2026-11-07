-- Standardisasi kunci primer (docs/SKEMA-DATABASE.md), langkah 2: harga, lapangan, dan master wilayah.
--
-- PK VARCHAR(255) buatan aplikasi menjadi BIGINT GENERATED ALWAYS AS IDENTITY untuk empat tabel transaksional:
--   entri_harga       urutan ID: dicatat_pada, dibuat_pada, id lama (NULL waktu terakhir). ±32 ribu baris demo.
--   tugas_lapangan    urutan ID: dibuat_pada, id lama.
--   batas_harga       urutan ID: berlaku_dari, komoditas_kode, kabupaten_kode (NULL lebih dulu), id lama.
--                     (Tabel ini tidak punya kolom waktu pembuatan; tanggal berlaku adalah kolom waktu satu-satunya.)
--   perubahan_master  urutan ID: dibuat_pada, id lama.
-- ID lama (HRG-D00-00-01-K, UUID dari aplikasi, TGS-<slug>-<n>, BH-<komoditas>, PM-...) dipindah ke kolom
-- kode varchar(64) UNIQUE: kode tampilan, alias tautan lama (/v1/harga/HRG-..., /v1/lapangan/tugas/TGS-...),
-- kunci idempoten seeder (ON CONFLICT (kode)), dan sumber pemulihan ID lama saat Down. Baris yang dibuat sesudah
-- Up berisi kode NULL (indeks unik mengizinkan banyak NULL).
--
-- Tabel anak ikut dikonversi tanpa membongkar indeks dan constraint-nya (ALTER COLUMN TYPE ... USING lewat fungsi
-- bantu pg_temp, karena USING tidak boleh berisi subkueri):
--   riwayat_status_entri.entri_id            -> bigint, FK ke entri_harga ON DELETE CASCADE (PK BIGSERIAL tetap)
--   tugas_entri.entri_id, tugas_entri.tugas_id -> bigint, FK CASCADE (PK komposit (tugas_id, entri_id) tetap)
--
-- FK ke tabel kode (SKEMA §2) mendapat ON DELETE RESTRICT eksplisit; indeks FK yang belum ada ditambahkan:
--   entri_harga.pasar_id -> pasar, entri_harga.komoditas_kode -> komoditas, tugas_lapangan.pasar_id -> pasar,
--   batas_harga.komoditas_kode -> komoditas, batas_harga.kabupaten_kode -> kabupaten (indeks baru),
--   pasar.kabupaten_kode -> kabupaten, kabupaten.provinsi_kode -> provinsi.
-- pasar dan kabupaten tetap berkunci kode (tipe tidak berubah, jadi materialized view agregat_harga_harian yang
-- memakai pasar.id dan pasar.kabupaten_kode tidak tersentuh).
--
-- Indeks baru (nama idx_<tabel>_<kolom...>; yang sudah ada dipertahankan, tidak diduplikasi):
--   entri_harga: BRIN dicatat_pada (time-series, baris masuk kira-kira menurut waktu); (petugas_id, dicatat_pada DESC)
--     untuk /v1/harga/saya; parsial antrean validasi (status = 'DIAJUKAN') menurut skor_anomali DESC, dicatat_pada DESC
--     (urutan antrean validasi); idx_entri_harga_dicatat (dicatat_pada DESC, id) dibuat ulang karena memuat kolom id.
--     idx_entri_harga_komoditas_pasar, idx_entri_harga_status, idx_entri_harga_bentrok, idx_entri_harga_antrean
--     sudah melayani filter komoditas+pasar+tanggal, status+tanggal, deteksi bentrok, dan antrean menurut waktu.
--   tugas_lapangan: parsial tugas aktif per petugas (petugas_id, tenggat) WHERE status <> 'SELESAI', sama dengan
--     predikat kueri tautkan tugas dan tandai selesai.
--   perubahan_master: parsial antrean persetujuan (dibuat_pada) WHERE status = 'MENUNGGU'.
--   batas_harga: (kabupaten_kode) untuk FK. Kunci alami (komoditas_kode, COALESCE(kabupaten_kode, ''), berlaku_dari)
--     tetap dijaga uq_batas_harga_berlaku.
--
-- Rujukan lunak (tanpa FK): audit_log.entitas_id pada baris entitas 'entri_harga', 'tugas_lapangan', 'batas_harga',
-- dan 'perubahan_master' yang berisi ID lama dipetakan ke ID baru (id::text); nilai lain ('3 entri', 'ENT-1335' yang
-- tidak cocok dengan baris mana pun) dibiarkan. audit_log append-only: pemicunya dimatikan hanya selama UPDATE ini,
-- di transaksi yang sama. audit_log.sebelum/sesudah/metadata (snapshot JSON, mis. daftar ids aksi massal) dibiarkan
-- sebagai sejarah; tautan lama tetap berlaku lewat alias kode.
--
-- materialized view agregat_harga_harian dan mv_kpi_operasional merujuk entri_harga tetapi tidak entri_harga.id,
-- sehingga DROP COLUMN id lolos tanpa membuangnya.
--
-- ALTER COLUMN TYPE menulis ulang tabel dengan kunci ACCESS EXCLUSIVE; jalankan di jendela pemeliharaan untuk
-- basis data besar (entri_harga, riwayat_status_entri).
--
-- Down mengembalikan VARCHAR(255) dan ID lama (kode; baris tanpa kode memakai id::text), aturan ON DELETE semula
-- (NO ACTION), PK lama, dan membuang kolom kode serta indeks baru.

-- +goose Up
-- +goose StatementBegin
DO $mig$
DECLARE
    r       record;
    n       bigint;
    terlalu bigint;
BEGIN
    -- Rencana tabel induk: nama tabel, ekspresi urutan ID baru, nama entitas di audit_log.
    CREATE TEMP TABLE g1_induk (
        tabel   text NOT NULL,
        urutan  text NOT NULL,
        entitas text NOT NULL
    ) ON COMMIT DROP;
    INSERT INTO g1_induk (tabel, urutan, entitas) VALUES
        ('entri_harga',      'dicatat_pada NULLS LAST, dibuat_pada, id',                          'entri_harga'),
        ('tugas_lapangan',   'dibuat_pada, id',                                                   'tugas_lapangan'),
        ('batas_harga',      'berlaku_dari, komoditas_kode, kabupaten_kode NULLS FIRST, id',      'batas_harga'),
        ('perubahan_master', 'dibuat_pada, id',                                                   'perubahan_master');

    -- Penjaga: hanya tiga FK anak yang boleh menunjuk tabel induk. FK lain akan membuat konversi gagal di
    -- tengah jalan, jadi berhenti lebih awal dengan pesan yang jelas.
    FOR r IN
        SELECT con.conrelid::regclass::text AS anak, con.conname AS nama, con.confrelid::regclass::text AS induk
          FROM pg_constraint con
         WHERE con.contype = 'f'
           AND con.confrelid IN (SELECT t.tabel::regclass FROM g1_induk t)
           AND con.conname NOT IN ('riwayat_status_entri_entri_id_fkey', 'tugas_entri_entri_id_fkey', 'tugas_entri_tugas_id_fkey')
    LOOP
        RAISE EXCEPTION 'FK % pada % merujuk % tetapi belum ada di rencana migrasi 000111', r.nama, r.anak, r.induk;
    END LOOP;

    -- ID lama harus muat di kode varchar(64).
    FOR r IN SELECT tabel FROM g1_induk LOOP
        EXECUTE format('SELECT count(*) FROM %I WHERE length(id) > 64', r.tabel) INTO terlalu;
        IF terlalu > 0 THEN
            RAISE EXCEPTION '% baris di % punya ID lebih dari 64 karakter; tidak muat di kolom kode', terlalu, r.tabel;
        END IF;
    END LOOP;

    -- Penjaga anak yatim: FK menjamin tidak ada, diperiksa ulang karena konversi memakai pencarian induk.
    SELECT count(*) INTO n FROM riwayat_status_entri c WHERE NOT EXISTS (SELECT 1 FROM entri_harga p WHERE p.id = c.entri_id);
    IF n > 0 THEN RAISE EXCEPTION '% baris riwayat_status_entri tidak punya entri_harga induk', n; END IF;
    SELECT count(*) INTO n FROM tugas_entri c WHERE NOT EXISTS (SELECT 1 FROM entri_harga p WHERE p.id = c.entri_id);
    IF n > 0 THEN RAISE EXCEPTION '% baris tugas_entri tidak punya entri_harga induk', n; END IF;
    SELECT count(*) INTO n FROM tugas_entri c WHERE NOT EXISTS (SELECT 1 FROM tugas_lapangan p WHERE p.id = c.tugas_id);
    IF n > 0 THEN RAISE EXCEPTION '% baris tugas_entri tidak punya tugas_lapangan induk', n; END IF;

    -- 1. kode = ID lama, id_baru = nomor urut menurut kolom waktu tabel.
    FOR r IN SELECT * FROM g1_induk LOOP
        EXECUTE format('ALTER TABLE %I ADD COLUMN kode varchar(64)', r.tabel);
        EXECUTE format('UPDATE %I SET kode = id', r.tabel);
        EXECUTE format('ALTER TABLE %I ADD COLUMN id_baru bigint', r.tabel);
        EXECUTE format(
            'UPDATE %I x SET id_baru = u.nomor FROM (SELECT id, row_number() OVER (ORDER BY %s) AS nomor FROM %I) u WHERE x.id = u.id',
            r.tabel, r.urutan, r.tabel);
    END LOOP;

    -- 2. Lepas FK anak, lalu konversi kolom anak lewat fungsi bantu (PK induk lama masih ada sebagai indeks
    --    pencarian). VOLATILE supaya setiap pemanggilan membaca isi id_baru yang baru saja diisi.
    ALTER TABLE riwayat_status_entri DROP CONSTRAINT riwayat_status_entri_entri_id_fkey;
    ALTER TABLE tugas_entri DROP CONSTRAINT tugas_entri_entri_id_fkey;
    ALTER TABLE tugas_entri DROP CONSTRAINT tugas_entri_tugas_id_fkey;

    CREATE FUNCTION pg_temp.g1_id_baru_entri(text) RETURNS bigint LANGUAGE sql VOLATILE
    AS $fn$ SELECT id_baru FROM entri_harga WHERE id = $1 $fn$;
    CREATE FUNCTION pg_temp.g1_id_baru_tugas(text) RETURNS bigint LANGUAGE sql VOLATILE
    AS $fn$ SELECT id_baru FROM tugas_lapangan WHERE id = $1 $fn$;

    ALTER TABLE riwayat_status_entri ALTER COLUMN entri_id TYPE bigint USING pg_temp.g1_id_baru_entri(entri_id);
    ALTER TABLE tugas_entri ALTER COLUMN entri_id TYPE bigint USING pg_temp.g1_id_baru_entri(entri_id);
    ALTER TABLE tugas_entri ALTER COLUMN tugas_id TYPE bigint USING pg_temp.g1_id_baru_tugas(tugas_id);

    DROP FUNCTION pg_temp.g1_id_baru_entri(text);
    DROP FUNCTION pg_temp.g1_id_baru_tugas(text);

    -- 3. Tukar PK induk: id_baru menjadi id, identity dilanjutkan dari max(id) + 1, kode unik.
    --    DROP COLUMN id ikut membuang indeks yang memuat id (idx_entri_harga_dicatat; dibuat ulang di bawah).
    FOR r IN SELECT * FROM g1_induk LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.tabel || '_pkey');
        EXECUTE format('ALTER TABLE %I DROP COLUMN id', r.tabel);
        EXECUTE format('ALTER TABLE %I RENAME COLUMN id_baru TO id', r.tabel);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id SET NOT NULL', r.tabel);
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I PRIMARY KEY (id)', r.tabel, r.tabel || '_pkey');
        EXECUTE format('SELECT COALESCE(max(id), 0) + 1 FROM %I', r.tabel) INTO n;
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', r.tabel, n);
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I UNIQUE (kode)', r.tabel, 'uq_' || r.tabel || '_kode');
    END LOOP;

    -- 4. FK anak kembali, ON DELETE CASCADE seperti semula. Indeks anak sudah ada: idx_riwayat_status_entri
    --    (entri_id, waktu, id), idx_tugas_entri_entri (entri_id), dan PK tugas_entri (tugas_id, entri_id).
    ALTER TABLE riwayat_status_entri
        ADD CONSTRAINT riwayat_status_entri_entri_id_fkey FOREIGN KEY (entri_id) REFERENCES entri_harga(id) ON DELETE CASCADE;
    ALTER TABLE tugas_entri
        ADD CONSTRAINT tugas_entri_entri_id_fkey FOREIGN KEY (entri_id) REFERENCES entri_harga(id) ON DELETE CASCADE;
    ALTER TABLE tugas_entri
        ADD CONSTRAINT tugas_entri_tugas_id_fkey FOREIGN KEY (tugas_id) REFERENCES tugas_lapangan(id) ON DELETE CASCADE;

    -- 5. FK ke tabel kode: ON DELETE RESTRICT eksplisit (SKEMA §2).
    FOR r IN
        SELECT * FROM (VALUES
            ('entri_harga',    'entri_harga_pasar_id_fkey',          'pasar_id',       'pasar',     'id'),
            ('entri_harga',    'entri_harga_komoditas_kode_fkey',    'komoditas_kode', 'komoditas', 'kode'),
            ('tugas_lapangan', 'tugas_lapangan_pasar_id_fkey',       'pasar_id',       'pasar',     'id'),
            ('batas_harga',    'batas_harga_komoditas_kode_fkey',    'komoditas_kode', 'komoditas', 'kode'),
            ('batas_harga',    'batas_harga_kabupaten_kode_fkey',    'kabupaten_kode', 'kabupaten', 'kode'),
            ('pasar',          'pasar_kabupaten_kode_fkey',          'kabupaten_kode', 'kabupaten', 'kode'),
            ('kabupaten',      'kabupaten_provinsi_kode_fkey',       'provinsi_kode',  'provinsi',  'kode')
        ) v (tabel, nama, kolom, induk, kolom_induk)
    LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.nama);
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %I(%I) ON DELETE RESTRICT',
                       r.tabel, r.nama, r.kolom, r.induk, r.kolom_induk);
    END LOOP;

    -- 6. Indeks.
    CREATE INDEX idx_entri_harga_dicatat ON entri_harga (dicatat_pada DESC, id);
    CREATE INDEX idx_entri_harga_dicatat_brin ON entri_harga USING brin (dicatat_pada) WITH (pages_per_range = 32);
    CREATE INDEX idx_entri_harga_petugas_dicatat ON entri_harga (petugas_id, dicatat_pada DESC);
    CREATE INDEX idx_entri_harga_antrean_skor ON entri_harga (skor_anomali DESC, dicatat_pada DESC, id) WHERE status = 'DIAJUKAN';
    CREATE INDEX idx_tugas_lapangan_aktif ON tugas_lapangan (petugas_id, tenggat) WHERE status <> 'SELESAI';
    CREATE INDEX idx_batas_harga_kabupaten_kode ON batas_harga (kabupaten_kode);
    CREATE INDEX idx_perubahan_master_menunggu ON perubahan_master (dibuat_pada) WHERE status = 'MENUNGGU';

    -- 7. Rujukan lunak: audit_log.entitas_id yang berisi ID lama baris yang ada menjadi ID baru.
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    FOR r IN SELECT * FROM g1_induk LOOP
        EXECUTE format(
            'UPDATE audit_log a SET entitas_id = t.id::text FROM %I t WHERE a.entitas = %L AND a.entitas_id = t.kode',
            r.tabel, r.entitas);
    END LOOP;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;
END
$mig$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $mig$
DECLARE
    r record;
BEGIN
    CREATE TEMP TABLE g1_induk (
        tabel   text NOT NULL,
        entitas text NOT NULL
    ) ON COMMIT DROP;
    INSERT INTO g1_induk (tabel, entitas) VALUES
        ('entri_harga',      'entri_harga'),
        ('tugas_lapangan',   'tugas_lapangan'),
        ('batas_harga',      'batas_harga'),
        ('perubahan_master', 'perubahan_master');

    -- 1. Buang indeks baru dan lepas FK sebelum tipe kolom berubah.
    DROP INDEX idx_entri_harga_dicatat;
    DROP INDEX idx_entri_harga_dicatat_brin;
    DROP INDEX idx_entri_harga_petugas_dicatat;
    DROP INDEX idx_entri_harga_antrean_skor;
    DROP INDEX idx_tugas_lapangan_aktif;
    DROP INDEX idx_batas_harga_kabupaten_kode;
    DROP INDEX idx_perubahan_master_menunggu;

    ALTER TABLE riwayat_status_entri DROP CONSTRAINT riwayat_status_entri_entri_id_fkey;
    ALTER TABLE tugas_entri DROP CONSTRAINT tugas_entri_entri_id_fkey;
    ALTER TABLE tugas_entri DROP CONSTRAINT tugas_entri_tugas_id_fkey;

    -- 2. Pulihkan ID lama: kode, atau id::text untuk baris yang dibuat sesudah Up.
    FOR r IN SELECT * FROM g1_induk LOOP
        EXECUTE format('ALTER TABLE %I ADD COLUMN id_lama varchar(255)', r.tabel);
        EXECUTE format('UPDATE %I SET id_lama = COALESCE(kode, id::text)', r.tabel);
    END LOOP;

    CREATE FUNCTION pg_temp.g1_id_lama_entri(bigint) RETURNS varchar(255) LANGUAGE sql VOLATILE
    AS $fn$ SELECT id_lama FROM entri_harga WHERE id = $1 $fn$;
    CREATE FUNCTION pg_temp.g1_id_lama_tugas(bigint) RETURNS varchar(255) LANGUAGE sql VOLATILE
    AS $fn$ SELECT id_lama FROM tugas_lapangan WHERE id = $1 $fn$;

    ALTER TABLE riwayat_status_entri ALTER COLUMN entri_id TYPE varchar(255) USING pg_temp.g1_id_lama_entri(entri_id);
    ALTER TABLE tugas_entri ALTER COLUMN entri_id TYPE varchar(255) USING pg_temp.g1_id_lama_entri(entri_id);
    ALTER TABLE tugas_entri ALTER COLUMN tugas_id TYPE varchar(255) USING pg_temp.g1_id_lama_tugas(tugas_id);

    DROP FUNCTION pg_temp.g1_id_lama_entri(bigint);
    DROP FUNCTION pg_temp.g1_id_lama_tugas(bigint);

    -- 3. Rujukan lunak kembali ke ID lama (baris tanpa kode tetap id::text, sama dengan id lama hasil pemulihan).
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    FOR r IN SELECT * FROM g1_induk LOOP
        EXECUTE format(
            'UPDATE audit_log a SET entitas_id = t.kode FROM %I t WHERE a.entitas = %L AND a.entitas_id = t.id::text AND t.kode IS NOT NULL',
            r.tabel, r.entitas);
    END LOOP;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    -- 4. Tukar PK induk kembali ke VARCHAR(255); kolom kode dan constraint uniknya dibuang.
    FOR r IN SELECT * FROM g1_induk LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, 'uq_' || r.tabel || '_kode');
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.tabel || '_pkey');
        EXECUTE format('ALTER TABLE %I DROP COLUMN id', r.tabel);
        EXECUTE format('ALTER TABLE %I RENAME COLUMN id_lama TO id', r.tabel);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id SET NOT NULL', r.tabel);
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I PRIMARY KEY (id)', r.tabel, r.tabel || '_pkey');
        EXECUTE format('ALTER TABLE %I DROP COLUMN kode', r.tabel);
    END LOOP;

    -- 5. FK anak dengan aturan semula (CASCADE).
    ALTER TABLE riwayat_status_entri
        ADD CONSTRAINT riwayat_status_entri_entri_id_fkey FOREIGN KEY (entri_id) REFERENCES entri_harga(id) ON DELETE CASCADE;
    ALTER TABLE tugas_entri
        ADD CONSTRAINT tugas_entri_entri_id_fkey FOREIGN KEY (entri_id) REFERENCES entri_harga(id) ON DELETE CASCADE;
    ALTER TABLE tugas_entri
        ADD CONSTRAINT tugas_entri_tugas_id_fkey FOREIGN KEY (tugas_id) REFERENCES tugas_lapangan(id) ON DELETE CASCADE;

    -- 6. FK ke tabel kode kembali ke aturan semula (NO ACTION, bawaan PostgreSQL).
    FOR r IN
        SELECT * FROM (VALUES
            ('entri_harga',    'entri_harga_pasar_id_fkey',          'pasar_id',       'pasar',     'id'),
            ('entri_harga',    'entri_harga_komoditas_kode_fkey',    'komoditas_kode', 'komoditas', 'kode'),
            ('tugas_lapangan', 'tugas_lapangan_pasar_id_fkey',       'pasar_id',       'pasar',     'id'),
            ('batas_harga',    'batas_harga_komoditas_kode_fkey',    'komoditas_kode', 'komoditas', 'kode'),
            ('batas_harga',    'batas_harga_kabupaten_kode_fkey',    'kabupaten_kode', 'kabupaten', 'kode'),
            ('pasar',          'pasar_kabupaten_kode_fkey',          'kabupaten_kode', 'kabupaten', 'kode'),
            ('kabupaten',      'kabupaten_provinsi_kode_fkey',       'provinsi_kode',  'provinsi',  'kode')
        ) v (tabel, nama, kolom, induk, kolom_induk)
    LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.nama);
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES %I(%I)',
                       r.tabel, r.nama, r.kolom, r.induk, r.kolom_induk);
    END LOOP;

    -- 7. Indeks lama yang memuat id (dibuang bersama kolom id lama pada langkah 4).
    CREATE INDEX idx_entri_harga_dicatat ON entri_harga (dicatat_pada DESC, id);
END
$mig$;
-- +goose StatementEnd
