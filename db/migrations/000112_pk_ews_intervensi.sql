-- Standardisasi kunci primer (docs/SKEMA-DATABASE.md), langkah 3: EWS dan intervensi.
--
-- Kunci primer (ID lama selalu disimpan di kolom kode varchar(64) UNIQUE):
--   aturan_ews          varchar ATR-...  -> BIGINT identity, urut menurut diubah_pada (NULL terakhir)
--   aturan_ews_riwayat  varchar AR-n     -> BIGINT identity, urut menurut waktu; default 'AR-' || nextval dibuang
--   ews_alert           varchar EWS-n    -> BIGINT identity, urut menurut dibuat_pada
--   ews_riwayat         varchar RW-n     -> BIGINT identity, urut menurut waktu; default 'RW-' || nextval dibuang
--   intervensi          varchar INT-n    -> UUID = garda_uuid_stabil('intervensi', id_lama), bawaan gen_random_uuid()
--   ews_event, ews_notifikasi, intervensi_persetujuan sudah BIGINT identity; alur_persetujuan tetap KODE.
-- Pemutus seri urutan BIGINT adalah ID lama menurut urutan alami (awalan teks, lalu angka di ujung: RW-9 < RW-10).
--
-- Kolom anak (semua dikonversi dengan ALTER COLUMN TYPE ... USING, supaya NOT NULL, indeks, dan constraint
-- lain pada kolom itu ikut dibangun ulang):
--   aturan_ews_riwayat.aturan_id, ews_alert.aturan_id                    -> bigint (aturan_ews)
--   ews_event.alert_id, ews_notifikasi.alert_id, ews_riwayat.alert_id,
--   intervensi.ews_alert_id                                              -> bigint (ews_alert)
--   intervensi_persetujuan/intervensi_pj/intervensi_riwayat.intervensi_id -> uuid (intervensi)
--
-- Aturan ON DELETE eksplisit (SKEMA §2). Yang berubah dari NO ACTION:
--   RESTRICT  intervensi.ews_alert_id (alert pemicu adalah bukti asal kartu), kabupaten_kode dan komoditas_kode
--             di aturan_ews, ews_alert, dan intervensi (rujukan master), serta FK komposit ke
--             tahap_intervensi_def dari intervensi (siklus, tahap) dan alur_persetujuan (siklus, tahap_tujuan).
--   Dipertahankan: CASCADE untuk anak milik agregat (riwayat aturan, event/notifikasi/riwayat alert, riwayat,
--   penanggung jawab, dan permintaan persetujuan kartu), SET NULL untuk ews_alert.aturan_id dan
--   intervensi_persetujuan.alur_kode.
--
-- Indeks baru (nama idx_<tabel>_<kolom...>): B-tree untuk FK yang belum terindeks (aturan_ews komoditas/kabupaten,
-- ews_event.alert_id, intervensi komoditas/kabupaten, intervensi_persetujuan.alur_kode); indeks penuh untuk
-- intervensi_persetujuan.intervensi_id dan alur_persetujuan (siklus, tahap_tujuan), karena satu-satunya indeks
-- berawalan kolom itu adalah indeks unik parsial yang tidak bisa dipakai pemeriksaan FK; komposit filter+urut
-- ews_alert (status, dibuat_pada DESC, id) dan (kabupaten_kode, dibuat_pada DESC, id); parsial antrean:
-- ews_alert aktif (status <> 'SELESAI', dipakai ringkasan dasbor dan eskalasi otomatis), kartu aktif yang belum
-- dieskalasi (pemeriksaan SLA tahap), dan permintaan persetujuan MENUNGGU. Indeks yang menjadi awalan indeks lain
-- dibuang: idx_ews_status_severity, dan idx_intervensi_persetujuan_status diganti versi berurutan menurun.
--
-- Kode tampilan baris baru: sequence ews_alert_id_seq dan intervensi_id_seq (nomor EWS-n dan INT-n, tak bisa
-- bentrok antar-instance) diganti nama menjadi ews_alert_kode_seq dan intervensi_kode_seq karena nama aslinya
-- dipakai identity ews_alert.id dan menyesatkan untuk intervensi (id kini UUID). Keduanya dimajukan melewati
-- angka terbesar kode lama. Aplikasi menulis kode = 'EWS-' || nextval(...) dan 'INT-' || nextval(...).
--
-- Rujukan lunak (tanpa FK): audit_log.entitas_id pada baris entitas 'aturan_ews', 'ews_alert', dan 'intervensi'
-- yang berisi ID lama baris yang ada dipetakan ke ID baru (id::text). Snapshot JSON audit_log dan teks bebas
-- (mis. alasan riwayat "Dibuat otomatis dari alert EWS-2000") dibiarkan sebagai sejarah; tautan dari notifikasi
-- (/ews?id=EWS-2000) tetap berlaku lewat alias kode. audit_log append-only: pemicunya dimatikan hanya selama
-- UPDATE pemetaan ini, di transaksi yang sama.
--
-- ALTER COLUMN TYPE menulis ulang tabel dengan kunci ACCESS EXCLUSIVE; jalankan di jendela pemeliharaan untuk
-- basis data besar (ews_riwayat, ews_event). Fungsi bantu sementara (pg_temp.g2_*) dihapus di akhir blok.
--
-- Down mengembalikan varchar(255) dan ID lama dari kode (baris yang dibuat sesudah Up tanpa kode memakai
-- id::text), default dan sequence lama ('AR-' / 'RW-' || nextval, dimajukan melewati angka terbesar),
-- aturan ON DELETE semula, dan membuang indeks serta kolom kode.

-- +goose Up
-- +goose StatementBegin
DO $mig$
DECLARE
    r        record;
    n        bigint;
    -- Urutan alami ID lama sebagai pemutus seri: awalan teks, angka di ujung, lalu teks utuh.
    alami    constant text := $u$regexp_replace(id, '[0-9]+$', ''), COALESCE(substring(id from '[0-9]+$')::numeric, 0), id$u$;
BEGIN
    -- 0. Penjaga: baris anak tanpa induk membuat konversi menghasilkan NULL tanpa terasa.
    FOR r IN SELECT * FROM (VALUES
        ('aturan_ews_riwayat',     'aturan_id',     'aturan_ews'),
        ('ews_alert',              'aturan_id',     'aturan_ews'),
        ('ews_event',              'alert_id',      'ews_alert'),
        ('ews_notifikasi',         'alert_id',      'ews_alert'),
        ('ews_riwayat',            'alert_id',      'ews_alert'),
        ('intervensi',             'ews_alert_id',  'ews_alert'),
        ('intervensi_persetujuan', 'intervensi_id', 'intervensi'),
        ('intervensi_pj',          'intervensi_id', 'intervensi'),
        ('intervensi_riwayat',     'intervensi_id', 'intervensi')
    ) AS t(anak, kolom, induk) LOOP
        EXECUTE format('SELECT count(*) FROM %I a WHERE a.%I IS NOT NULL AND NOT EXISTS (SELECT 1 FROM %I p WHERE p.id = a.%I)',
                       r.anak, r.kolom, r.induk, r.kolom) INTO n;
        IF n > 0 THEN
            RAISE EXCEPTION 'migrasi 000112: % baris % punya % yang tidak ada di %', n, r.anak, r.kolom, r.induk;
        END IF;
    END LOOP;

    -- Sequence kode: nama asli dibutuhkan identity (ews_alert_id_seq) dan menyesatkan untuk UUID (intervensi_id_seq).
    ALTER SEQUENCE ews_alert_id_seq RENAME TO ews_alert_kode_seq;
    ALTER SEQUENCE intervensi_id_seq RENAME TO intervensi_kode_seq;

    -- 1. Lepas FK yang akan berubah tipe atau aturan hapusnya (nama sama dipasang lagi di langkah 6).
    FOR r IN SELECT * FROM (VALUES
        ('aturan_ews_riwayat',     'aturan_ews_riwayat_aturan_id_fkey'),
        ('ews_alert',              'ews_alert_aturan_id_fkey'),
        ('ews_event',              'ews_event_alert_id_fkey'),
        ('ews_notifikasi',         'ews_notifikasi_alert_id_fkey'),
        ('ews_riwayat',            'ews_riwayat_alert_id_fkey'),
        ('intervensi',             'intervensi_ews_alert_id_fkey'),
        ('intervensi_persetujuan', 'intervensi_persetujuan_intervensi_id_fkey'),
        ('intervensi_pj',          'intervensi_pj_intervensi_id_fkey'),
        ('intervensi_riwayat',     'intervensi_riwayat_intervensi_id_fkey'),
        ('aturan_ews',             'aturan_ews_komoditas_kode_fkey'),
        ('aturan_ews',             'aturan_ews_kabupaten_kode_fkey'),
        ('ews_alert',              'ews_alert_komoditas_kode_fkey'),
        ('ews_alert',              'ews_alert_kabupaten_kode_fkey'),
        ('intervensi',             'intervensi_komoditas_kode_fkey'),
        ('intervensi',             'intervensi_kabupaten_kode_fkey'),
        ('intervensi',             'fk_intervensi_tahap'),
        ('alur_persetujuan',       'fk_alur_persetujuan_tahap')
    ) AS t(tabel, nama) LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.nama);
    END LOOP;

    -- 2. Fungsi bantu sementara.
    -- g2_kunci_baru: simpan ID lama di kode dan beri nomor BIGINT baru (id_baru) menurut urutan yang diminta.
    CREATE FUNCTION pg_temp.g2_kunci_baru(tabel text, urut text) RETURNS void LANGUAGE plpgsql AS $f$
    BEGIN
        EXECUTE format('ALTER TABLE %I ADD COLUMN kode varchar(64)', tabel);
        EXECUTE format('UPDATE %I SET kode = id', tabel);
        EXECUTE format('ALTER TABLE %I ADD COLUMN id_baru bigint', tabel);
        EXECUTE format(
            'UPDATE %1$I t SET id_baru = u.nomor FROM (SELECT id, row_number() OVER (ORDER BY %2$s) AS nomor FROM %1$I) u WHERE t.id = u.id',
            tabel, urut);
    END
    $f$;
    -- g2_selesai_bigint: id menjadi bigint identity (mulai dari max+1) dan kode menjadi UNIQUE. ALTER ... TYPE ... USING
    -- id_baru mempertahankan PK dan indeks yang memuat id, serta posisi kolom. Sequence default lama (bila ada)
    -- dibuang lebih dulu supaya sequence identity memakai nama baku <tabel>_id_seq.
    CREATE FUNCTION pg_temp.g2_selesai_bigint(tabel text, seq_lama text) RETURNS void LANGUAGE plpgsql AS $f$
    DECLARE
        mulai bigint;
    BEGIN
        EXECUTE format('SELECT COALESCE(MAX(id_baru), 0) + 1 FROM %I', tabel) INTO mulai;
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id DROP DEFAULT', tabel);
        IF seq_lama IS NOT NULL THEN
            EXECUTE format('DROP SEQUENCE %I', seq_lama);
        END IF;
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id TYPE bigint USING id_baru', tabel);
        EXECUTE format('ALTER TABLE %I DROP COLUMN id_baru', tabel);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', tabel, mulai);
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I UNIQUE (kode)', tabel, 'uq_' || tabel || '_kode');
    END
    $f$;
    -- g2_majukan_seq: majukan sequence kode melewati angka terbesar kode lama (tidak pernah memundurkan).
    CREATE FUNCTION pg_temp.g2_majukan_seq(seq text, maks bigint) RETURNS void LANGUAGE plpgsql AS $f$
    DECLARE
        terakhir bigint;
        awal     bigint;
    BEGIN
        SELECT last_value, start_value INTO terakhir, awal FROM pg_sequences WHERE schemaname = 'public' AND sequencename = seq;
        IF maks >= COALESCE(terakhir + 1, awal) THEN
            PERFORM setval(('public.' || seq)::regclass, maks);
        END IF;
    END
    $f$;

    -- 3. Nomor baru dan kode untuk tabel BIGINT. Waktu NULL diurutkan terakhir.
    PERFORM pg_temp.g2_kunci_baru('aturan_ews',         'diubah_pada NULLS LAST, ' || alami);
    PERFORM pg_temp.g2_kunci_baru('aturan_ews_riwayat', 'waktu NULLS LAST, ' || alami);
    PERFORM pg_temp.g2_kunci_baru('ews_alert',          'dibuat_pada NULLS LAST, ' || alami);
    PERFORM pg_temp.g2_kunci_baru('ews_riwayat',        'waktu NULLS LAST, ' || alami);
    -- intervensi: UUID stabil dihitung dari ID lama; cukup menyimpan kode.
    ALTER TABLE intervensi ADD COLUMN kode varchar(64);
    UPDATE intervensi SET kode = id;

    -- USING tidak boleh berisi subkueri, jadi pemetaan lewat fungsi.
    CREATE FUNCTION pg_temp.g2_id_aturan(text) RETURNS bigint LANGUAGE sql STABLE AS $f$ SELECT id_baru FROM aturan_ews WHERE id = $1 $f$;
    CREATE FUNCTION pg_temp.g2_id_alert(text)  RETURNS bigint LANGUAGE sql STABLE AS $f$ SELECT id_baru FROM ews_alert WHERE id = $1 $f$;

    -- 4. Konversi kolom anak.
    ALTER TABLE aturan_ews_riwayat ALTER COLUMN aturan_id TYPE bigint USING pg_temp.g2_id_aturan(aturan_id);
    ALTER TABLE ews_alert ALTER COLUMN aturan_id TYPE bigint USING pg_temp.g2_id_aturan(aturan_id);
    ALTER TABLE ews_event ALTER COLUMN alert_id TYPE bigint USING pg_temp.g2_id_alert(alert_id);
    ALTER TABLE ews_notifikasi ALTER COLUMN alert_id TYPE bigint USING pg_temp.g2_id_alert(alert_id);
    ALTER TABLE ews_riwayat ALTER COLUMN alert_id TYPE bigint USING pg_temp.g2_id_alert(alert_id);
    ALTER TABLE intervensi ALTER COLUMN ews_alert_id TYPE bigint USING pg_temp.g2_id_alert(ews_alert_id);
    ALTER TABLE intervensi_persetujuan ALTER COLUMN intervensi_id TYPE uuid USING garda_uuid_stabil('intervensi', intervensi_id);
    ALTER TABLE intervensi_pj ALTER COLUMN intervensi_id TYPE uuid USING garda_uuid_stabil('intervensi', intervensi_id);
    ALTER TABLE intervensi_riwayat ALTER COLUMN intervensi_id TYPE uuid USING garda_uuid_stabil('intervensi', intervensi_id);

    -- 5. Konversi kunci induk.
    PERFORM pg_temp.g2_selesai_bigint('aturan_ews', NULL);
    PERFORM pg_temp.g2_selesai_bigint('ews_alert', NULL);
    PERFORM pg_temp.g2_selesai_bigint('aturan_ews_riwayat', 'aturan_ews_riwayat_id_seq');
    PERFORM pg_temp.g2_selesai_bigint('ews_riwayat', 'ews_riwayat_id_seq');
    ALTER TABLE intervensi ALTER COLUMN id TYPE uuid USING garda_uuid_stabil('intervensi', id);
    ALTER TABLE intervensi ALTER COLUMN id SET DEFAULT gen_random_uuid();
    ALTER TABLE intervensi ADD CONSTRAINT uq_intervensi_kode UNIQUE (kode);

    -- Kode baru tidak boleh bentrok dengan kode lama.
    SELECT COALESCE(max(substring(kode from '^EWS-([0-9]+)$')::bigint), 0) INTO n FROM ews_alert;
    PERFORM pg_temp.g2_majukan_seq('ews_alert_kode_seq', n);
    SELECT COALESCE(max(substring(kode from '^INT-([0-9]+)$')::bigint), 0) INTO n FROM intervensi;
    PERFORM pg_temp.g2_majukan_seq('intervensi_kode_seq', n);

    -- 6. Pasang FK dengan ON DELETE eksplisit.
    ALTER TABLE aturan_ews_riwayat ADD CONSTRAINT aturan_ews_riwayat_aturan_id_fkey
        FOREIGN KEY (aturan_id) REFERENCES aturan_ews(id) ON DELETE CASCADE;
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_aturan_id_fkey
        FOREIGN KEY (aturan_id) REFERENCES aturan_ews(id) ON DELETE SET NULL;
    ALTER TABLE ews_event ADD CONSTRAINT ews_event_alert_id_fkey
        FOREIGN KEY (alert_id) REFERENCES ews_alert(id) ON DELETE CASCADE;
    ALTER TABLE ews_notifikasi ADD CONSTRAINT ews_notifikasi_alert_id_fkey
        FOREIGN KEY (alert_id) REFERENCES ews_alert(id) ON DELETE CASCADE;
    ALTER TABLE ews_riwayat ADD CONSTRAINT ews_riwayat_alert_id_fkey
        FOREIGN KEY (alert_id) REFERENCES ews_alert(id) ON DELETE CASCADE;
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_ews_alert_id_fkey
        FOREIGN KEY (ews_alert_id) REFERENCES ews_alert(id) ON DELETE RESTRICT;
    ALTER TABLE intervensi_persetujuan ADD CONSTRAINT intervensi_persetujuan_intervensi_id_fkey
        FOREIGN KEY (intervensi_id) REFERENCES intervensi(id) ON DELETE CASCADE;
    ALTER TABLE intervensi_pj ADD CONSTRAINT intervensi_pj_intervensi_id_fkey
        FOREIGN KEY (intervensi_id) REFERENCES intervensi(id) ON DELETE CASCADE;
    ALTER TABLE intervensi_riwayat ADD CONSTRAINT intervensi_riwayat_intervensi_id_fkey
        FOREIGN KEY (intervensi_id) REFERENCES intervensi(id) ON DELETE CASCADE;

    ALTER TABLE aturan_ews ADD CONSTRAINT aturan_ews_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE aturan_ews ADD CONSTRAINT aturan_ews_kabupaten_kode_fkey
        FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_kabupaten_kode_fkey
        FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_kabupaten_kode_fkey
        FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE intervensi ADD CONSTRAINT fk_intervensi_tahap
        FOREIGN KEY (siklus, tahap) REFERENCES tahap_intervensi_def (siklus, kode_tahap) ON DELETE RESTRICT;
    ALTER TABLE alur_persetujuan ADD CONSTRAINT fk_alur_persetujuan_tahap
        FOREIGN KEY (siklus, tahap_tujuan) REFERENCES tahap_intervensi_def (siklus, kode_tahap) ON DELETE RESTRICT;

    -- 7. Indeks.
    CREATE INDEX idx_aturan_ews_komoditas_kode ON aturan_ews (komoditas_kode);
    CREATE INDEX idx_aturan_ews_kabupaten_kode ON aturan_ews (kabupaten_kode);
    CREATE INDEX idx_ews_event_alert ON ews_event (alert_id);
    -- ews_alert: daftar menurut status atau wilayah (terbaru dahulu, urutan sama dengan ORDER BY repositori).
    CREATE INDEX idx_ews_alert_status_dibuat ON ews_alert (status, dibuat_pada DESC, id);
    CREATE INDEX idx_ews_alert_kabupaten_dibuat ON ews_alert (kabupaten_kode, dibuat_pada DESC, id);
    -- Antrean alert aktif: ringkasan dasbor (status <> 'SELESAI' GROUP BY severity) dan pemindaian eskalasi otomatis.
    CREATE INDEX idx_ews_alert_aktif ON ews_alert (severity, dibuat_pada DESC) WHERE status <> 'SELESAI';
    DROP INDEX idx_ews_status_severity; -- awalan idx_ews_alert_status_severity_waktu
    CREATE INDEX idx_intervensi_komoditas_kode ON intervensi (komoditas_kode);
    CREATE INDEX idx_intervensi_kabupaten_kode ON intervensi (kabupaten_kode);
    -- Antrean pemeriksaan SLA: kartu aktif yang belum dieskalasi, tertua di tahapnya lebih dulu.
    CREATE INDEX idx_intervensi_aktif_tahap_sejak ON intervensi (tahap_sejak, id)
        WHERE selesai_pada IS NULL AND dieskalasi_pada IS NULL;
    CREATE INDEX idx_intervensi_persetujuan_intervensi_id ON intervensi_persetujuan (intervensi_id);
    CREATE INDEX idx_intervensi_persetujuan_alur_kode ON intervensi_persetujuan (alur_kode);
    DROP INDEX idx_intervensi_persetujuan_status;
    CREATE INDEX idx_intervensi_persetujuan_status_diajukan ON intervensi_persetujuan (status, diajukan_pada DESC, id DESC);
    CREATE INDEX idx_intervensi_persetujuan_menunggu ON intervensi_persetujuan (diajukan_pada DESC, id DESC)
        WHERE status = 'MENUNGGU';
    CREATE INDEX idx_alur_persetujuan_tahap ON alur_persetujuan (siklus, tahap_tujuan);

    -- 8. Rujukan lunak: entitas_id audit yang berisi ID lama baris yang ada.
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    UPDATE audit_log a SET entitas_id = t.id::text FROM aturan_ews t WHERE a.entitas = 'aturan_ews' AND a.entitas_id = t.kode;
    UPDATE audit_log a SET entitas_id = t.id::text FROM ews_alert t WHERE a.entitas = 'ews_alert' AND a.entitas_id = t.kode;
    UPDATE audit_log a SET entitas_id = t.id::text FROM intervensi t WHERE a.entitas = 'intervensi' AND a.entitas_id = t.kode;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    COMMENT ON COLUMN aturan_ews.kode IS 'ID lama (ATR-...), alias rute /v1/ews/aturan/:id; unik.';
    COMMENT ON COLUMN aturan_ews_riwayat.kode IS 'ID lama (AR-n); NULL untuk baris yang dibuat sesudah migrasi 000112.';
    COMMENT ON COLUMN ews_alert.kode IS 'Kode tampilan EWS-n (ID lama; baris baru dari ews_alert_kode_seq), alias rute /v1/ews/:id; unik.';
    COMMENT ON COLUMN ews_riwayat.kode IS 'ID lama (RW-n, RW-D-...); NULL untuk baris yang dibuat sesudah migrasi 000112.';
    COMMENT ON COLUMN intervensi.kode IS 'Kode tampilan INT-n (ID lama; baris baru dari intervensi_kode_seq), alias rute /v1/intervensi/:id; unik.';

    DROP FUNCTION pg_temp.g2_id_alert(text);
    DROP FUNCTION pg_temp.g2_id_aturan(text);
    DROP FUNCTION pg_temp.g2_majukan_seq(text, bigint);
    DROP FUNCTION pg_temp.g2_selesai_bigint(text, text);
    DROP FUNCTION pg_temp.g2_kunci_baru(text, text);
END
$mig$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $mig$
DECLARE
    r record;
    n bigint;
BEGIN
    -- Pemulih ID: kode (ID lama), atau id::text untuk baris yang dibuat sesudah Up. Fungsi karena USING
    -- pada ALTER COLUMN TYPE tidak boleh berisi subkueri.
    CREATE FUNCTION pg_temp.g2_lama_aturan(bigint) RETURNS varchar(255) LANGUAGE sql STABLE AS
        $f$ SELECT COALESCE(kode, id::text)::varchar(255) FROM aturan_ews WHERE id = $1 $f$;
    CREATE FUNCTION pg_temp.g2_lama_alert(bigint) RETURNS varchar(255) LANGUAGE sql STABLE AS
        $f$ SELECT COALESCE(kode, id::text)::varchar(255) FROM ews_alert WHERE id = $1 $f$;
    CREATE FUNCTION pg_temp.g2_lama_intervensi(uuid) RETURNS varchar(255) LANGUAGE sql STABLE AS
        $f$ SELECT COALESCE(kode, id::text)::varchar(255) FROM intervensi WHERE id = $1 $f$;
    CREATE FUNCTION pg_temp.g2_majukan_seq(seq text, maks bigint) RETURNS void LANGUAGE plpgsql AS $f$
    DECLARE
        terakhir bigint;
        awal     bigint;
    BEGIN
        SELECT last_value, start_value INTO terakhir, awal FROM pg_sequences WHERE schemaname = 'public' AND sequencename = seq;
        IF maks >= COALESCE(terakhir + 1, awal) THEN
            PERFORM setval(('public.' || seq)::regclass, maks);
        END IF;
    END
    $f$;

    -- 1. Rujukan lunak kembali ke ID lama (sebelum ID berubah tipe).
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    UPDATE audit_log a SET entitas_id = t.kode FROM aturan_ews t
     WHERE a.entitas = 'aturan_ews' AND a.entitas_id = t.id::text AND t.kode IS NOT NULL;
    UPDATE audit_log a SET entitas_id = t.kode FROM ews_alert t
     WHERE a.entitas = 'ews_alert' AND a.entitas_id = t.id::text AND t.kode IS NOT NULL;
    UPDATE audit_log a SET entitas_id = t.kode FROM intervensi t
     WHERE a.entitas = 'intervensi' AND a.entitas_id = t.id::text AND t.kode IS NOT NULL;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    -- 2. Lepas FK dan indeks yang dibuat Up.
    FOR r IN SELECT * FROM (VALUES
        ('aturan_ews_riwayat',     'aturan_ews_riwayat_aturan_id_fkey'),
        ('ews_alert',              'ews_alert_aturan_id_fkey'),
        ('ews_event',              'ews_event_alert_id_fkey'),
        ('ews_notifikasi',         'ews_notifikasi_alert_id_fkey'),
        ('ews_riwayat',            'ews_riwayat_alert_id_fkey'),
        ('intervensi',             'intervensi_ews_alert_id_fkey'),
        ('intervensi_persetujuan', 'intervensi_persetujuan_intervensi_id_fkey'),
        ('intervensi_pj',          'intervensi_pj_intervensi_id_fkey'),
        ('intervensi_riwayat',     'intervensi_riwayat_intervensi_id_fkey'),
        ('aturan_ews',             'aturan_ews_komoditas_kode_fkey'),
        ('aturan_ews',             'aturan_ews_kabupaten_kode_fkey'),
        ('ews_alert',              'ews_alert_komoditas_kode_fkey'),
        ('ews_alert',              'ews_alert_kabupaten_kode_fkey'),
        ('intervensi',             'intervensi_komoditas_kode_fkey'),
        ('intervensi',             'intervensi_kabupaten_kode_fkey'),
        ('intervensi',             'fk_intervensi_tahap'),
        ('alur_persetujuan',       'fk_alur_persetujuan_tahap')
    ) AS t(tabel, nama) LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.nama);
    END LOOP;
    DROP INDEX idx_aturan_ews_komoditas_kode;
    DROP INDEX idx_aturan_ews_kabupaten_kode;
    DROP INDEX idx_ews_event_alert;
    DROP INDEX idx_ews_alert_status_dibuat;
    DROP INDEX idx_ews_alert_kabupaten_dibuat;
    DROP INDEX idx_ews_alert_aktif;
    DROP INDEX idx_intervensi_komoditas_kode;
    DROP INDEX idx_intervensi_kabupaten_kode;
    DROP INDEX idx_intervensi_aktif_tahap_sejak;
    DROP INDEX idx_intervensi_persetujuan_intervensi_id;
    DROP INDEX idx_intervensi_persetujuan_alur_kode;
    DROP INDEX idx_intervensi_persetujuan_status_diajukan;
    DROP INDEX idx_intervensi_persetujuan_menunggu;
    DROP INDEX idx_alur_persetujuan_tahap;
    CREATE INDEX idx_ews_status_severity ON ews_alert (status, severity);
    CREATE INDEX idx_intervensi_persetujuan_status ON intervensi_persetujuan (status, diajukan_pada);

    -- 3. Kolom anak kembali ke varchar(255) (pemetaan memakai induk yang masih bertipe baru).
    ALTER TABLE aturan_ews_riwayat ALTER COLUMN aturan_id TYPE varchar(255) USING pg_temp.g2_lama_aturan(aturan_id);
    ALTER TABLE ews_alert ALTER COLUMN aturan_id TYPE varchar(255) USING pg_temp.g2_lama_aturan(aturan_id);
    ALTER TABLE ews_event ALTER COLUMN alert_id TYPE varchar(255) USING pg_temp.g2_lama_alert(alert_id);
    ALTER TABLE ews_notifikasi ALTER COLUMN alert_id TYPE varchar(255) USING pg_temp.g2_lama_alert(alert_id);
    ALTER TABLE ews_riwayat ALTER COLUMN alert_id TYPE varchar(255) USING pg_temp.g2_lama_alert(alert_id);
    ALTER TABLE intervensi ALTER COLUMN ews_alert_id TYPE varchar(255) USING pg_temp.g2_lama_alert(ews_alert_id);
    ALTER TABLE intervensi_persetujuan ALTER COLUMN intervensi_id TYPE varchar(255) USING pg_temp.g2_lama_intervensi(intervensi_id);
    ALTER TABLE intervensi_pj ALTER COLUMN intervensi_id TYPE varchar(255) USING pg_temp.g2_lama_intervensi(intervensi_id);
    ALTER TABLE intervensi_riwayat ALTER COLUMN intervensi_id TYPE varchar(255) USING pg_temp.g2_lama_intervensi(intervensi_id);

    -- 4. Kunci induk kembali ke varchar(255); identity dan kolom kode dibuang.
    ALTER TABLE intervensi ALTER COLUMN id DROP DEFAULT;
    ALTER TABLE intervensi ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text);
    ALTER TABLE intervensi DROP CONSTRAINT uq_intervensi_kode;
    ALTER TABLE intervensi DROP COLUMN kode;
    ALTER SEQUENCE intervensi_kode_seq RENAME TO intervensi_id_seq;

    FOR r IN SELECT * FROM (VALUES ('aturan_ews'), ('ews_alert'), ('aturan_ews_riwayat'), ('ews_riwayat')) AS t(tabel) LOOP
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id DROP IDENTITY', r.tabel);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text)', r.tabel);
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, 'uq_' || r.tabel || '_kode');
        EXECUTE format('ALTER TABLE %I DROP COLUMN kode', r.tabel);
    END LOOP;
    -- DROP IDENTITY membuang sequence ews_alert_id_seq; nama itu kembali ke sequence kode.
    ALTER SEQUENCE ews_alert_kode_seq RENAME TO ews_alert_id_seq;

    -- Tabel riwayat memulihkan default 'AR-' / 'RW-' || nextval, dimajukan melewati nomor yang sudah terpakai.
    CREATE SEQUENCE aturan_ews_riwayat_id_seq;
    SELECT COALESCE(max(substring(id from '^AR-([0-9]+)$')::bigint), 0) INTO n FROM aturan_ews_riwayat;
    PERFORM pg_temp.g2_majukan_seq('aturan_ews_riwayat_id_seq', n);
    ALTER TABLE aturan_ews_riwayat ALTER COLUMN id SET DEFAULT ('AR-' || nextval('aturan_ews_riwayat_id_seq')::text);
    CREATE SEQUENCE ews_riwayat_id_seq;
    SELECT COALESCE(max(substring(id from '^RW-([0-9]+)$')::bigint), 0) INTO n FROM ews_riwayat;
    PERFORM pg_temp.g2_majukan_seq('ews_riwayat_id_seq', n);
    ALTER TABLE ews_riwayat ALTER COLUMN id SET DEFAULT ('RW-' || nextval('ews_riwayat_id_seq')::text);

    -- 5. FK dengan aturan semula (NO ACTION = bawaan PostgreSQL).
    ALTER TABLE aturan_ews_riwayat ADD CONSTRAINT aturan_ews_riwayat_aturan_id_fkey
        FOREIGN KEY (aturan_id) REFERENCES aturan_ews(id) ON DELETE CASCADE;
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_aturan_id_fkey
        FOREIGN KEY (aturan_id) REFERENCES aturan_ews(id) ON DELETE SET NULL;
    ALTER TABLE ews_event ADD CONSTRAINT ews_event_alert_id_fkey
        FOREIGN KEY (alert_id) REFERENCES ews_alert(id) ON DELETE CASCADE;
    ALTER TABLE ews_notifikasi ADD CONSTRAINT ews_notifikasi_alert_id_fkey
        FOREIGN KEY (alert_id) REFERENCES ews_alert(id) ON DELETE CASCADE;
    ALTER TABLE ews_riwayat ADD CONSTRAINT ews_riwayat_alert_id_fkey
        FOREIGN KEY (alert_id) REFERENCES ews_alert(id) ON DELETE CASCADE;
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_ews_alert_id_fkey
        FOREIGN KEY (ews_alert_id) REFERENCES ews_alert(id);
    ALTER TABLE intervensi_persetujuan ADD CONSTRAINT intervensi_persetujuan_intervensi_id_fkey
        FOREIGN KEY (intervensi_id) REFERENCES intervensi(id) ON DELETE CASCADE;
    ALTER TABLE intervensi_pj ADD CONSTRAINT intervensi_pj_intervensi_id_fkey
        FOREIGN KEY (intervensi_id) REFERENCES intervensi(id) ON DELETE CASCADE;
    ALTER TABLE intervensi_riwayat ADD CONSTRAINT intervensi_riwayat_intervensi_id_fkey
        FOREIGN KEY (intervensi_id) REFERENCES intervensi(id) ON DELETE CASCADE;

    ALTER TABLE aturan_ews ADD CONSTRAINT aturan_ews_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE aturan_ews ADD CONSTRAINT aturan_ews_kabupaten_kode_fkey
        FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE ews_alert ADD CONSTRAINT ews_alert_kabupaten_kode_fkey
        FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE intervensi ADD CONSTRAINT intervensi_kabupaten_kode_fkey
        FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE intervensi ADD CONSTRAINT fk_intervensi_tahap
        FOREIGN KEY (siklus, tahap) REFERENCES tahap_intervensi_def (siklus, kode_tahap);
    ALTER TABLE alur_persetujuan ADD CONSTRAINT fk_alur_persetujuan_tahap
        FOREIGN KEY (siklus, tahap_tujuan) REFERENCES tahap_intervensi_def (siklus, kode_tahap);

    DROP FUNCTION pg_temp.g2_majukan_seq(text, bigint);
    DROP FUNCTION pg_temp.g2_lama_intervensi(uuid);
    DROP FUNCTION pg_temp.g2_lama_alert(bigint);
    DROP FUNCTION pg_temp.g2_lama_aturan(bigint);
END
$mig$;
-- +goose StatementEnd
