-- Slice 03-C (docs/todos/03-audit.md): partisi bulanan audit_log, retensi ke penyimpanan dingin, dan riwayat
-- verifikasi rantai hash.
--
-- Keputusan:
--   * audit_log menjadi PARTITION BY RANGE (waktu), satu partisi per bulan UTC bernama audit_log_pYYYYMM. PK menjadi
--     (waktu, id) karena PK tabel berpartisi wajib memuat kolom partisi. id tetap BIGINT identity tunggal (satu
--     sequence di induk; semua penulisan lewat induk), jadi tetap unik walau PK-nya komposit.
--   * Rute GET /v1/audit/:id tetap menerima id saja: pencarian lintas partisi lewat idx_audit_log_id (satu probe
--     indeks per partisi). idx_audit_log_id juga melayani kepala rantai hash dan verifikasi urut id.
--   * kode (ID lama) tetap unik secara GLOBAL: UNIQUE (waktu, kode) hanya unik per waktu, jadi keunikan dijaga
--     pemicu trg_audit_log_kode_unik yang berjalan di bawah kunci rantai (penulis audit sudah berurutan, sehingga
--     pemeriksaannya bebas balapan). Pencarian kode lewat indeks parsial idx_audit_log_kode. Seeder demo memakai
--     NOT EXISTS, bukan ON CONFLICT (kode).
--   * idx_audit_log_waktu_id (waktu DESC, id DESC) dibuang: PK (waktu, id) yang dipindai mundur melayani cursor
--     (waktu, id) yang sama. BRIN waktu tetap ada (per partisi).
--   * Tanpa partisi DEFAULT. Partisi disiapkan di depan: migrasi ini membuat bulan lalu sampai tiga bulan ke depan,
--     job worker audit_partisi menjaga jendela ke depan, dan penulis audit membuat partisi bulannya sendiri bila
--     belum ada (audit_log_siapkan_partisi; termasuk baris bertanggal mundur, kecuali bulan yang sudah diarsipkan). Partisi baru dibuat terpisah lalu di-ATTACH, yang hanya mengambil kunci
--     SHARE UPDATE EXCLUSIVE pada induk (baca/tulis audit tidak terhenti).
--   * TRUNCATE langsung pada partisi tidak memicu pemicu statement induk, jadi tiap partisi diberi
--     trg_audit_log_no_truncate sendiri. Hak akses partisi dicabut: partisi hanya dibaca/ditulis lewat induk.
--   * Retensi (audit_log_lepas_partisi, SECURITY DEFINER): partisi yang seluruh rentangnya lebih tua dari masa
--     retensi (minimal 12 bulan) dilepas SETELAH worker mengekspornya ke berkas arsip dan memberikan bukti (jumlah
--     baris dan ringkasan SHA-256 atas hash seluruh barisnya, urut id). Fungsi memeriksa ulang bukti itu di bawah
--     kunci, mencatat arsip dan segmen rantainya, lalu DETACH dan DROP. Kepala rantai tidak pernah diarsipkan.
--     DROP partisi ber-FK mengambil kunci ACCESS EXCLUSIVE sesaat pada pengguna; lock_timeout 3 detik membuat
--     fungsi gagal cepat (dicoba lagi putaran berikutnya) daripada mengantrekan login.
--   * Segmen arsip (audit_log_arsip_segmen): rangkaian id berurutan dari partisi yang diarsipkan, dengan hash masuk
--     (hash_sebelumnya baris pertama) dan hash keluar (hash baris terakhir). Verifikasi menyambung baris yang masih
--     hidup melewati segmen ini, jadi rantai tetap terverifikasi utuh walau baris lama sudah pindah ke arsip, termasuk
--     bila id dua bulan saling selang-seling di batas bulan.
--   * audit_log_verifikasi menyimpan hasil setiap verifikasi rantai (append-only).
--
-- Konversi menyalin seluruh baris ke tabel berpartisi baru (kunci ACCESS EXCLUSIVE selama migrasi); jalankan di
-- jendela pemeliharaan untuk audit_log besar. Down mengembalikan tabel biasa dan menolak berjalan bila sudah ada
-- partisi yang diarsipkan (catatan arsipnya akan hilang).

-- +goose Up
-- +goose StatementBegin

-- 1. Lepaskan nama kanonik dari tabel lama (dibuang di langkah 5).
ALTER TABLE audit_log RENAME TO audit_log_lama;
ALTER TABLE audit_log_lama RENAME CONSTRAINT audit_log_pkey TO audit_log_lama_pkey;
ALTER TABLE audit_log_lama RENAME CONSTRAINT uq_audit_log_kode TO uq_audit_log_lama_kode;
DROP INDEX idx_audit_log_aksi_waktu;
DROP INDEX idx_audit_log_aktor_waktu;
DROP INDEX idx_audit_log_entitas_waktu;
DROP INDEX idx_audit_log_waktu_brin;
DROP INDEX idx_audit_log_waktu_id;
DO $seq$
BEGIN
    EXECUTE format('ALTER SEQUENCE %s RENAME TO audit_log_lama_id_seq', pg_get_serial_sequence('audit_log_lama', 'id'));
END;
$seq$;

-- 2. Tabel induk berpartisi (urutan kolom sama dengan tabel lama).
CREATE TABLE audit_log (
    id              bigint GENERATED ALWAYS AS IDENTITY,
    waktu           timestamptz NOT NULL DEFAULT now(),
    aktor_id        uuid,
    aksi            varchar(100) NOT NULL,
    entitas         varchar(100) NOT NULL,
    entitas_id      varchar(255) NOT NULL,
    hasil           varchar(20) NOT NULL,
    ip              varchar(45),
    metadata        jsonb,
    aktor_nama      varchar(255),
    aktor_email     varchar(255),
    aktor_peran     varchar(50),
    user_agent      text,
    correlation_id  varchar(100),
    sebelum         jsonb,
    sesudah         jsonb,
    kode            varchar(64),
    hash_sebelumnya bytea,
    hash            bytea NOT NULL,
    CONSTRAINT audit_log_pkey PRIMARY KEY (waktu, id),
    CONSTRAINT chk_audit_log_hash CHECK (octet_length(hash) = 32),
    CONSTRAINT chk_audit_log_hash_sebelumnya CHECK (hash_sebelumnya IS NULL OR octet_length(hash_sebelumnya) = 32)
) PARTITION BY RANGE (waktu);

-- 3. Pembuat satu partisi bulanan (internal; tidak bisa dipanggil peran aplikasi). Idempoten.
CREATE OR REPLACE FUNCTION audit_log_buat_partisi(p_bulan date) RETURNS text
    LANGUAGE plpgsql
    SET search_path = pg_catalog, public
    SET timezone = 'UTC'
AS $$
DECLARE
    v_bulan date := date_trunc('month', p_bulan::timestamp)::date;
    v_nama  text := 'audit_log_p' || to_char(v_bulan, 'YYYYMM');
    v_awal  timestamptz := v_bulan::timestamp AT TIME ZONE 'UTC';
    v_akhir timestamptz := (v_bulan + interval '1 month') AT TIME ZONE 'UTC';
    v_oid   oid;
BEGIN
    IF p_bulan IS NULL THEN
        RAISE EXCEPTION 'bulan partisi audit wajib diisi' USING ERRCODE = 'invalid_parameter_value';
    END IF;
    -- Dua pemanggil serentak (job dan penulis audit) tidak boleh membuat partisi yang sama.
    PERFORM pg_advisory_xact_lock(hashtextextended('garda.audit_log.partisi', 0));
    v_oid := to_regclass('public.' || v_nama);
    IF v_oid IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM pg_inherits WHERE inhrelid = v_oid AND inhparent = 'public.audit_log'::regclass) THEN
            RAISE EXCEPTION 'tabel % ada tetapi bukan partisi audit_log', v_nama;
        END IF;
        RETURN NULL;
    END IF;

    EXECUTE format('CREATE TABLE public.%I (LIKE public.audit_log INCLUDING DEFAULTS INCLUDING CONSTRAINTS)', v_nama);
    EXECUTE format('ALTER TABLE public.audit_log ATTACH PARTITION public.%I FOR VALUES FROM (%L) TO (%L)',
        v_nama, v_awal, v_akhir);
    EXECUTE format('CREATE TRIGGER trg_audit_log_no_truncate BEFORE TRUNCATE ON public.%I '
        'FOR EACH STATEMENT EXECUTE FUNCTION audit_log_tolak_perubahan()', v_nama);
    EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC', v_nama);
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'garda_aplikasi') THEN
        EXECUTE format('REVOKE ALL ON public.%I FROM garda_aplikasi', v_nama);
    END IF;
    RETURN v_nama;
END;
$$;
REVOKE ALL ON FUNCTION audit_log_buat_partisi(date) FROM PUBLIC;

-- Penyiap partisi untuk worker dan penulis audit (baris bertanggal mundur, mis. impor data lama): bulan mana pun
-- sesudah bulan terakhir yang sudah diarsipkan, sampai 24 bulan ke depan. Bulan yang sudah diarsipkan tidak bisa
-- dibuka lagi lewat jalur ini.
CREATE OR REPLACE FUNCTION audit_log_siapkan_partisi(p_dari timestamptz, p_jumlah_bulan integer)
    RETURNS SETOF text
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = pg_catalog, public
    SET timezone = 'UTC'
    SET lock_timeout = '3s'
AS $$
DECLARE
    v_mulai    date;
    v_minimal  date := COALESCE((SELECT max(sampai) FROM audit_log_arsip), '2000-01-01 00:00:00+00')::date;
    v_maksimal date := (date_trunc('month', now()) + interval '24 month')::date;
    v_nama     text;
BEGIN
    IF p_dari IS NULL OR p_jumlah_bulan IS NULL OR p_jumlah_bulan < 1 OR p_jumlah_bulan > 25 THEN
        RAISE EXCEPTION 'partisi audit: jumlah bulan harus 1 sampai 25' USING ERRCODE = 'invalid_parameter_value';
    END IF;
    v_mulai := date_trunc('month', p_dari)::date;
    IF v_mulai < v_minimal OR (v_mulai + make_interval(months => p_jumlah_bulan - 1))::date > v_maksimal THEN
        RAISE EXCEPTION 'partisi audit hanya boleh disiapkan untuk % sampai %', v_minimal, v_maksimal
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    FOR i IN 0 .. p_jumlah_bulan - 1 LOOP
        v_nama := audit_log_buat_partisi((v_mulai + make_interval(months => i))::date);
        IF v_nama IS NOT NULL THEN
            RETURN NEXT v_nama;
        END IF;
    END LOOP;
END;
$$;
REVOKE ALL ON FUNCTION audit_log_siapkan_partisi(timestamptz, integer) FROM PUBLIC;

-- 4. Partisi untuk data lama, bulan lalu, bulan ini, dan tiga bulan ke depan; lalu salin baris lama apa adanya
--    (id, hash, dan rantai tidak berubah).
DO $partisi$
DECLARE
    v_bulan date := date_trunc('month', LEAST(
        COALESCE((SELECT min(waktu) FROM audit_log_lama), now()),
        now() - interval '1 month') AT TIME ZONE 'UTC')::date;
    v_akhir date := (date_trunc('month', now() AT TIME ZONE 'UTC') + interval '3 month')::date;
BEGIN
    WHILE v_bulan <= v_akhir LOOP
        PERFORM audit_log_buat_partisi(v_bulan);
        v_bulan := (v_bulan + interval '1 month')::date;
    END LOOP;
END;
$partisi$;

INSERT INTO audit_log (
    id, waktu, aktor_id, aksi, entitas, entitas_id, hasil, ip, metadata, aktor_nama, aktor_email, aktor_peran,
    user_agent, correlation_id, sebelum, sesudah, kode, hash_sebelumnya, hash)
OVERRIDING SYSTEM VALUE
SELECT id, waktu, aktor_id, aksi, entitas, entitas_id, hasil, ip, metadata, aktor_nama, aktor_email, aktor_peran,
       user_agent, correlation_id, sebelum, sesudah, kode, hash_sebelumnya, hash
  FROM audit_log_lama
 ORDER BY id;

-- Sequence identity melanjutkan keadaan sequence lama (>= id terbesar).
SELECT setval(pg_get_serial_sequence('audit_log', 'id'), s.last_value, s.is_called)
  FROM audit_log_lama_id_seq s;

-- 5. Buang tabel lama (beserta pemicu dan sequence-nya).
DROP TABLE audit_log_lama;

-- 6. Indeks, FK, dan komentar pada induk (turun ke semua partisi).
CREATE INDEX idx_audit_log_id ON audit_log (id);
CREATE INDEX idx_audit_log_kode ON audit_log (kode) WHERE kode IS NOT NULL;
CREATE INDEX idx_audit_log_aksi_waktu ON audit_log (aksi, waktu DESC);
CREATE INDEX idx_audit_log_aktor_waktu ON audit_log (aktor_id, waktu DESC);
CREATE INDEX idx_audit_log_entitas_waktu ON audit_log (entitas, entitas_id, waktu DESC);
CREATE INDEX idx_audit_log_waktu_brin ON audit_log USING brin (waktu) WITH (pages_per_range = 32);
ALTER TABLE audit_log
    ADD CONSTRAINT audit_log_aktor_id_fkey FOREIGN KEY (aktor_id) REFERENCES pengguna(id) ON DELETE SET NULL;

COMMENT ON TABLE audit_log IS 'Jejak audit append-only, berpartisi bulanan (UTC) audit_log_pYYYYMM, berantai hash (000116).';
COMMENT ON COLUMN audit_log.kode IS 'ID lama (UUID teks atau demo-audit-NNNN); alias rute /v1/audit/:id. Unik global (pemicu trg_audit_log_kode_unik). NULL untuk baris baru.';
COMMENT ON COLUMN audit_log.hash IS 'SHA-256 garda-audit-v1 atas hash_sebelumnya + isi baris (lihat migrasi 000116); diisi pemicu.';
COMMENT ON COLUMN audit_log.hash_sebelumnya IS 'hash baris dengan id sebelumnya; NULL hanya untuk baris pertama rantai.';

-- 7. Pemicu: append-only (baris diklon ke tiap partisi), TRUNCATE, kunci rantai, keunikan kode, dan hash.
CREATE OR REPLACE FUNCTION audit_log_kode_unik() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF NEW.kode IS NOT NULL AND EXISTS (SELECT 1 FROM audit_log a WHERE a.kode = NEW.kode) THEN
        RAISE EXCEPTION 'kode audit % sudah dipakai', NEW.kode
            USING ERRCODE = 'unique_violation', CONSTRAINT = 'uq_audit_log_kode';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_tolak_perubahan();
CREATE TRIGGER trg_audit_log_no_truncate
    BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_tolak_perubahan();
CREATE TRIGGER trg_audit_log_rantai_kunci
    BEFORE INSERT ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_kunci_rantai();
-- Nama pemicu baris menentukan urutan: kode_unik lalu rantai (alfabetis).
CREATE TRIGGER trg_audit_log_kode_unik
    BEFORE INSERT ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_kode_unik();
CREATE TRIGGER trg_audit_log_rantai
    BEFORE INSERT ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_rantai_hash();

-- 8. Tabel pendukung integritas: append-only.
CREATE OR REPLACE FUNCTION audit_tolak_ubah_baris() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    RAISE EXCEPTION '% bersifat append-only: % ditolak', TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$;

CREATE TABLE audit_log_verifikasi (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mulai          timestamptz NOT NULL,
    selesai        timestamptz NOT NULL DEFAULT now(),
    pemicu         varchar(10) NOT NULL,
    status         varchar(10) NOT NULL,
    jumlah_baris   bigint NOT NULL DEFAULT 0,
    jumlah_segmen  bigint NOT NULL DEFAULT 0,
    jumlah_masalah bigint NOT NULL DEFAULT 0,
    id_awal        bigint,
    id_akhir       bigint,
    hash_akhir     bytea,
    masalah        jsonb,
    keterangan     text,
    CONSTRAINT chk_audit_log_verifikasi_pemicu CHECK (pemicu IN ('JADWAL', 'MANUAL')),
    CONSTRAINT chk_audit_log_verifikasi_status CHECK (status IN ('VALID', 'RUSAK', 'GAGAL'))
);
CREATE INDEX idx_audit_log_verifikasi_mulai ON audit_log_verifikasi (mulai DESC, id DESC);
COMMENT ON TABLE audit_log_verifikasi IS 'Hasil verifikasi rantai hash audit_log (worker audit_verifikasi); append-only.';

CREATE TABLE audit_log_arsip (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    partisi         varchar(63) NOT NULL,
    dari            timestamptz NOT NULL,
    sampai          timestamptz NOT NULL,
    jumlah_baris    bigint NOT NULL,
    id_pertama      bigint,
    id_terakhir     bigint,
    ringkasan       bytea NOT NULL,
    berkas          text NOT NULL,
    sha256_berkas   varchar(64) NOT NULL,
    retensi_bulan   integer NOT NULL,
    diarsipkan_pada timestamptz NOT NULL DEFAULT now(),
    diarsipkan_oleh text NOT NULL DEFAULT session_user,
    CONSTRAINT chk_audit_log_arsip_rentang CHECK (dari < sampai)
);
CREATE INDEX idx_audit_log_arsip_dari ON audit_log_arsip (dari);
COMMENT ON TABLE audit_log_arsip IS 'Partisi audit_log yang dilepas ke penyimpanan dingin (berkas arsip); append-only.';
COMMENT ON COLUMN audit_log_arsip.ringkasan IS 'SHA-256 atas gabungan hash seluruh baris partisi, urut id.';

CREATE TABLE audit_log_arsip_segmen (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    arsip_id     bigint NOT NULL REFERENCES audit_log_arsip(id) ON DELETE RESTRICT,
    id_awal      bigint NOT NULL,
    id_akhir     bigint NOT NULL,
    jumlah_baris bigint NOT NULL,
    hash_masuk   bytea,
    hash_keluar  bytea NOT NULL,
    CONSTRAINT chk_audit_log_arsip_segmen_rentang CHECK (id_awal <= id_akhir)
);
CREATE INDEX idx_audit_log_arsip_segmen_arsip_id ON audit_log_arsip_segmen (arsip_id);
CREATE INDEX idx_audit_log_arsip_segmen_id_awal ON audit_log_arsip_segmen (id_awal);
COMMENT ON TABLE audit_log_arsip_segmen IS 'Rangkaian id berurutan yang diarsipkan; hash_masuk/hash_keluar menyambung rantai yang hidup.';

CREATE TRIGGER trg_audit_log_verifikasi_append_only BEFORE UPDATE OR DELETE ON audit_log_verifikasi
    FOR EACH ROW EXECUTE FUNCTION audit_tolak_ubah_baris();
CREATE TRIGGER trg_audit_log_verifikasi_no_truncate BEFORE TRUNCATE ON audit_log_verifikasi
    FOR EACH STATEMENT EXECUTE FUNCTION audit_tolak_ubah_baris();
CREATE TRIGGER trg_audit_log_arsip_append_only BEFORE UPDATE OR DELETE ON audit_log_arsip
    FOR EACH ROW EXECUTE FUNCTION audit_tolak_ubah_baris();
CREATE TRIGGER trg_audit_log_arsip_no_truncate BEFORE TRUNCATE ON audit_log_arsip
    FOR EACH STATEMENT EXECUTE FUNCTION audit_tolak_ubah_baris();
CREATE TRIGGER trg_audit_log_arsip_segmen_append_only BEFORE UPDATE OR DELETE ON audit_log_arsip_segmen
    FOR EACH ROW EXECUTE FUNCTION audit_tolak_ubah_baris();
CREATE TRIGGER trg_audit_log_arsip_segmen_no_truncate BEFORE TRUNCATE ON audit_log_arsip_segmen
    FOR EACH STATEMENT EXECUTE FUNCTION audit_tolak_ubah_baris();

-- 9. Retensi: lepas satu partisi yang sudah diekspor. Dipanggil worker (audit_retensi) setelah berkas arsip ditulis.
CREATE OR REPLACE FUNCTION audit_log_lepas_partisi(
    p_partisi text, p_retensi_bulan integer, p_jumlah_baris bigint, p_ringkasan bytea,
    p_berkas text, p_sha256_berkas text
) RETURNS bigint
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = pg_catalog, public
    SET timezone = 'UTC'
    SET lock_timeout = '3s'
AS $$
DECLARE
    v_bulan     date;
    v_awal      timestamptz;
    v_akhir     timestamptz;
    v_oid       oid;
    v_jumlah    bigint;
    v_id_min    bigint;
    v_id_max    bigint;
    v_ringkasan bytea;
    v_arsip_id  bigint;
BEGIN
    IF p_retensi_bulan IS NULL OR p_retensi_bulan < 12 THEN
        RAISE EXCEPTION 'masa retensi audit minimal 12 bulan (diminta %)', p_retensi_bulan
            USING ERRCODE = 'invalid_parameter_value';
    END IF;
    IF p_partisi IS NULL OR p_partisi !~ '^audit_log_p[0-9]{6}$' THEN
        RAISE EXCEPTION 'nama partisi audit tidak dikenal: %', p_partisi USING ERRCODE = 'invalid_parameter_value';
    END IF;
    IF p_jumlah_baris IS NULL OR p_jumlah_baris < 0 OR p_ringkasan IS NULL OR octet_length(p_ringkasan) <> 32
       OR coalesce(p_berkas, '') = '' OR coalesce(p_sha256_berkas, '') !~ '^[0-9a-f]{64}$' THEN
        RAISE EXCEPTION 'bukti arsip partisi % tidak lengkap', p_partisi USING ERRCODE = 'invalid_parameter_value';
    END IF;

    v_bulan := to_date(substr(p_partisi, 12, 6), 'YYYYMM');
    v_awal := v_bulan::timestamp AT TIME ZONE 'UTC';
    v_akhir := (v_bulan + interval '1 month') AT TIME ZONE 'UTC';
    IF v_akhir > date_trunc('month', now()) - make_interval(months => p_retensi_bulan) THEN
        RAISE EXCEPTION 'partisi % masih dalam masa retensi % bulan', p_partisi, p_retensi_bulan
            USING ERRCODE = 'invalid_parameter_value';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended('garda.audit_log.partisi', 0));
    v_oid := to_regclass('public.' || p_partisi);
    IF v_oid IS NULL OR NOT EXISTS (
        SELECT 1 FROM pg_inherits WHERE inhrelid = v_oid AND inhparent = 'public.audit_log'::regclass) THEN
        RAISE EXCEPTION 'tabel % bukan partisi audit_log', p_partisi USING ERRCODE = 'invalid_parameter_value';
    END IF;
    IF (SELECT pg_get_expr(relpartbound, oid) FROM pg_class WHERE oid = v_oid)
       IS DISTINCT FROM format('FOR VALUES FROM (%L) TO (%L)', v_awal, v_akhir) THEN
        RAISE EXCEPTION 'rentang partisi % tidak sesuai dengan namanya', p_partisi;
    END IF;

    -- Kunci induk lebih dulu (dibutuhkan DETACH) agar urutan kunci sama dengan penulis audit; tanpa deadlock.
    LOCK TABLE public.audit_log IN ACCESS EXCLUSIVE MODE;

    EXECUTE format('SELECT count(*), min(id), max(id), '
        'sha256(coalesce(string_agg(hash, ''''::bytea ORDER BY id), ''''::bytea)) FROM public.%I', p_partisi)
        INTO v_jumlah, v_id_min, v_id_max, v_ringkasan;
    IF v_jumlah <> p_jumlah_baris OR v_ringkasan <> p_ringkasan THEN
        RAISE EXCEPTION 'isi partisi % berbeda dengan berkas arsip (% baris di basis data, % di berkas)',
            p_partisi, v_jumlah, p_jumlah_baris USING ERRCODE = 'data_exception';
    END IF;
    IF v_jumlah > 0 AND v_id_max >= (SELECT max(id) FROM public.audit_log) THEN
        RAISE EXCEPTION 'partisi % memuat kepala rantai audit; tidak diarsipkan', p_partisi;
    END IF;

    INSERT INTO audit_log_arsip (partisi, dari, sampai, jumlah_baris, id_pertama, id_terakhir, ringkasan, berkas,
                                 sha256_berkas, retensi_bulan)
    VALUES (p_partisi, v_awal, v_akhir, v_jumlah, v_id_min, v_id_max, v_ringkasan, p_berkas, p_sha256_berkas,
            p_retensi_bulan)
    RETURNING id INTO v_arsip_id;

    -- Segmen: baris partisi berurutan id, dipisah oleh id yang masih hidup di partisi lain atau sudah diarsipkan
    -- lebih dulu.
    IF v_jumlah > 0 THEN
        EXECUTE format($q$
            WITH p AS (SELECT id, hash_sebelumnya, hash FROM public.%I),
            pemisah AS (
                SELECT a.id FROM public.audit_log a
                 WHERE a.id BETWEEN $1 AND $2 AND a.tableoid <> $3
                UNION ALL
                SELECT s.id_awal FROM public.audit_log_arsip_segmen s WHERE s.id_awal BETWEEN $1 AND $2
            ),
            bertanda AS (SELECT p.*, (SELECT count(*) FROM pemisah m WHERE m.id < p.id) AS grup FROM p)
            INSERT INTO public.audit_log_arsip_segmen (arsip_id, id_awal, id_akhir, jumlah_baris, hash_masuk, hash_keluar)
            SELECT $4, min(id), max(id), count(*),
                   (array_agg(hash_sebelumnya ORDER BY id))[1], (array_agg(hash ORDER BY id DESC))[1]
              FROM bertanda
             GROUP BY grup
             ORDER BY min(id)
        $q$, p_partisi) USING v_id_min, v_id_max, v_oid, v_arsip_id;
    END IF;

    -- Pemicu tolak-TRUNCATE milik partisi ikut terbuang bersama tabelnya.
    EXECUTE format('ALTER TABLE public.audit_log DETACH PARTITION public.%I', p_partisi);
    EXECUTE format('DROP TABLE public.%I', p_partisi);
    RETURN v_arsip_id;
END;
$$;
REVOKE ALL ON FUNCTION audit_log_lepas_partisi(text, integer, bigint, bytea, text, text) FROM PUBLIC;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $jaga$
BEGIN
    IF EXISTS (SELECT 1 FROM audit_log_arsip) THEN
        RAISE EXCEPTION 'audit_log sudah punya partisi yang diarsipkan; Down 000117 akan menghapus catatan arsipnya. '
            'Pulihkan dari backup, jangan goose down.';
    END IF;
END;
$jaga$;

DROP FUNCTION IF EXISTS audit_log_lepas_partisi(text, integer, bigint, bytea, text, text);
DROP TABLE IF EXISTS audit_log_arsip_segmen;
DROP TABLE IF EXISTS audit_log_arsip;
DROP TABLE IF EXISTS audit_log_verifikasi;
DROP FUNCTION IF EXISTS audit_tolak_ubah_baris();

-- Tabel berpartisi -> tabel biasa (keadaan sesudah 000116).
ALTER TABLE audit_log RENAME TO audit_log_berpartisi;
ALTER TABLE audit_log_berpartisi RENAME CONSTRAINT audit_log_pkey TO audit_log_berpartisi_pkey;
DROP INDEX idx_audit_log_id;
DROP INDEX idx_audit_log_kode;
DROP INDEX idx_audit_log_aksi_waktu;
DROP INDEX idx_audit_log_aktor_waktu;
DROP INDEX idx_audit_log_entitas_waktu;
DROP INDEX idx_audit_log_waktu_brin;
DO $seq$
BEGIN
    EXECUTE format('ALTER SEQUENCE %s RENAME TO audit_log_berpartisi_id_seq',
        pg_get_serial_sequence('audit_log_berpartisi', 'id'));
END;
$seq$;

CREATE TABLE audit_log (
    id              bigint GENERATED ALWAYS AS IDENTITY,
    waktu           timestamptz NOT NULL DEFAULT now(),
    aktor_id        uuid,
    aksi            varchar(100) NOT NULL,
    entitas         varchar(100) NOT NULL,
    entitas_id      varchar(255) NOT NULL,
    hasil           varchar(20) NOT NULL,
    ip              varchar(45),
    metadata        jsonb,
    aktor_nama      varchar(255),
    aktor_email     varchar(255),
    aktor_peran     varchar(50),
    user_agent      text,
    correlation_id  varchar(100),
    sebelum         jsonb,
    sesudah         jsonb,
    kode            varchar(64),
    hash_sebelumnya bytea,
    hash            bytea NOT NULL,
    CONSTRAINT audit_log_pkey PRIMARY KEY (id),
    CONSTRAINT uq_audit_log_kode UNIQUE (kode),
    CONSTRAINT chk_audit_log_hash CHECK (octet_length(hash) = 32),
    CONSTRAINT chk_audit_log_hash_sebelumnya CHECK (hash_sebelumnya IS NULL OR octet_length(hash_sebelumnya) = 32)
);

INSERT INTO audit_log (
    id, waktu, aktor_id, aksi, entitas, entitas_id, hasil, ip, metadata, aktor_nama, aktor_email, aktor_peran,
    user_agent, correlation_id, sebelum, sesudah, kode, hash_sebelumnya, hash)
OVERRIDING SYSTEM VALUE
SELECT id, waktu, aktor_id, aksi, entitas, entitas_id, hasil, ip, metadata, aktor_nama, aktor_email, aktor_peran,
       user_agent, correlation_id, sebelum, sesudah, kode, hash_sebelumnya, hash
  FROM audit_log_berpartisi
 ORDER BY id;

SELECT setval(pg_get_serial_sequence('audit_log', 'id'), s.last_value, s.is_called)
  FROM audit_log_berpartisi_id_seq s;

DROP TABLE audit_log_berpartisi;
DROP FUNCTION IF EXISTS audit_log_siapkan_partisi(timestamptz, integer);
DROP FUNCTION IF EXISTS audit_log_buat_partisi(date);
DROP FUNCTION IF EXISTS audit_log_kode_unik();

CREATE INDEX idx_audit_log_aksi_waktu ON audit_log (aksi, waktu DESC);
CREATE INDEX idx_audit_log_aktor_waktu ON audit_log (aktor_id, waktu DESC);
CREATE INDEX idx_audit_log_entitas_waktu ON audit_log (entitas, entitas_id, waktu DESC);
CREATE INDEX idx_audit_log_waktu_id ON audit_log (waktu DESC, id DESC);
CREATE INDEX idx_audit_log_waktu_brin ON audit_log USING brin (waktu) WITH (pages_per_range = 32);
ALTER TABLE audit_log
    ADD CONSTRAINT audit_log_aktor_id_fkey FOREIGN KEY (aktor_id) REFERENCES pengguna(id) ON DELETE SET NULL;

COMMENT ON COLUMN audit_log.kode IS 'ID lama (UUID teks atau demo-audit-NNNN); alias rute /v1/audit/:id dan kunci idempoten seeder demo. NULL untuk baris baru.';
COMMENT ON COLUMN audit_log.hash IS 'SHA-256 garda-audit-v1 atas hash_sebelumnya + isi baris (lihat migrasi 000116); diisi pemicu.';
COMMENT ON COLUMN audit_log.hash_sebelumnya IS 'hash baris dengan id sebelumnya; NULL hanya untuk baris pertama rantai.';

CREATE TRIGGER trg_audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_tolak_perubahan();
CREATE TRIGGER trg_audit_log_no_truncate
    BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_tolak_perubahan();
CREATE TRIGGER trg_audit_log_rantai_kunci
    BEFORE INSERT ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION audit_log_kunci_rantai();
CREATE TRIGGER trg_audit_log_rantai
    BEFORE INSERT ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_rantai_hash();
-- +goose StatementEnd
