-- Standardisasi kunci primer (docs/SKEMA-DATABASE.md), langkah 5: prediksi dan policy brief.
--
-- Tabel yang dikonversi, dengan ID lama (varchar buatan aplikasi) disimpan di kolom kode varchar(64) UNIQUE:
--   model_registry      id varchar 'MDL-<komoditas>-<versi>'   -> BIGINT identity (urutan dilatih_pada, id)
--   prediksi_harga      id varchar 'PRD-...' / 'PRM-...'       -> BIGINT identity (urutan dibuat_pada, id)
--   prediksi_evaluasi   PK prediksi_id (1:1 dengan proyeksi)   -> BIGINT id identity + UNIQUE (prediksi_id)
--   model_promosi       id varchar (UUID)                      -> BIGINT identity (urutan waktu, id)
--   skenario_simulasi   id varchar (UUID)                      -> BIGINT identity (urutan dibuat_pada, id)
--   brief_job           id varchar (UUID)                      -> BIGINT identity (urutan mulai, id)
--   policy_brief_versi  PK komposit (brief_id, versi)          -> BIGINT id identity + UNIQUE (brief_id, versi)
--   policy_brief        id varchar 'PB-YYYY-MM'                -> UUID garda_uuid_stabil('policy_brief', id_lama)
--
-- policy_brief adalah entitas bisnis yang terlihat klien, jadi UUID. Kode tampilannya PB-YYYY-MM tetap ada di
-- kolom kode: NOT NULL, UNIQUE, dan dijaga CHECK berbentuk PB-YYYY-MM (satu brief per periode; Generate memakai
-- kode itu sebagai kunci idempotensi). Rute /v1/policy-brief/PB-2026-10 tetap berlaku sebagai alias.
--
-- Kunci asing (semua dengan ON DELETE eksplisit dan indeks berawalan kolom FK):
--   prediksi_harga.model_id -> model_registry(id) RESTRICT. Kolom baru, diisi dari pasangan lama
--     (komoditas_kode, model_versi), NOT NULL seperti model_versi. FK komposit varchar lama fk_prediksi_harga_model
--     dilepas (FK varchar ke tabel ber-surrogate key dilarang). model_versi tetap sebagai kolom informasi yang
--     ditulis konsisten dengan model_id, dan tetap bagian dari UNIQUE uq_prediksi_harga_proyeksi.
--   model_promosi.model_id dan model_digantikan_id -> model_registry(id) RESTRICT (indeks model_digantikan_id baru).
--   prediksi_evaluasi.prediksi_id -> prediksi_harga(id) CASCADE.
--   brief_job.brief_id dan policy_brief_versi.brief_id -> policy_brief(id) CASCADE (sebelumnya NO ACTION).
--   FK ke komoditas dan kabupaten dari tabel di atas (NO ACTION) -> RESTRICT; indeks yang belum ada ditambahkan.
--   FK ke pengguna sudah selesai di 000110 dan tidak disentuh.
--
-- Indeks prediksi_harga mengikuti kueri nyata repositori: idx_prediksi_harga_kabupaten_horizon
-- (kabupaten_kode, horizon_hari, komoditas_kode, tanggal_target DESC) untuk daftar proyeksi terbaru per
-- komoditas, idx_prediksi_harga_terbaru (komoditas_kode, kabupaten_kode, horizon_hari, tanggal_target DESC)
-- untuk satu komoditas dan jalurnya. idx_prediksi_harga_lookup lama menjadi awalan uq_prediksi_harga_proyeksi
-- (dipakai pula oleh kueri KPI), jadi dibuang. brief_job mendapat indeks parsial untuk job BERJALAN.
-- Tanpa BRIN: prediksi_harga bukan time-series bervolume besar (ratusan baris per pelatihan).
--
-- Rujukan lunak (tanpa FK): audit_log.entitas_id pada entitas 'model_registry', 'policy_brief', dan
-- 'skenario_simulasi' yang berisi ID lama dipetakan ke ID baru (id::text). Nilai '-' (prediksi_evaluasi) dan kode
-- komoditas (proyeksi_harga) bukan ID dan dibiarkan; snapshot JSON (audit_log.sebelum/sesudah/metadata,
-- skenario_simulasi.hasil) dibiarkan sebagai sejarah. Isi brief (konten, snapshot_data) memuat id fakta
-- (ANDIL_1_NILAI), bukan ID basis data. audit_log append-only: pemicunya dimatikan hanya selama UPDATE ini.
--
-- ALTER COLUMN TYPE menulis ulang tabel dengan kunci ACCESS EXCLUSIVE; tabel di sini kecil. Fungsi bantu
-- pg_temp.g4_* menggantikan subkueri di USING dan dibuang di akhir Up maupun Down.
--
-- Down mengembalikan varchar(255) dengan ID lama dari kode (baris yang dibuat sesudah Up dan tidak punya kode
-- memakai id::text), FK komposit lama, aturan ON DELETE semula, indeks lama, PK komposit policy_brief_versi dan
-- PK prediksi_evaluasi.prediksi_id, lalu membuang kolom kode, model_id, dan indeks baru.

-- +goose Up
-- +goose StatementBegin
DO $mig$
DECLARE
    r     record;
    n     bigint;
    maks  bigint;
BEGIN
    -- Penjaga 1: ID lama harus muat di kode varchar(64), dan brief harus berbentuk PB-YYYY-MM.
    FOR r IN SELECT nama FROM (VALUES ('model_registry'), ('prediksi_harga'), ('model_promosi'),
                                      ('skenario_simulasi'), ('brief_job'), ('policy_brief')) t(nama)
    LOOP
        EXECUTE format('SELECT count(*) FROM %I WHERE length(id) > 64', r.nama) INTO n;
        IF n > 0 THEN
            RAISE EXCEPTION 'migrasi 000114: % baris di % punya id lebih dari 64 karakter; tidak muat di kolom kode', n, r.nama;
        END IF;
    END LOOP;
    SELECT count(*) INTO n FROM policy_brief WHERE id !~ '^PB-[0-9]{4}-(0[1-9]|1[0-2])$';
    IF n > 0 THEN
        RAISE EXCEPTION 'migrasi 000114: % brief punya id yang bukan PB-YYYY-MM; perbaiki dulu sebelum migrasi', n;
    END IF;

    -- Penjaga 2: tidak ada baris anak yatim (FK lama sudah mencegahnya; ini pagar terakhir sebelum FK dilepas).
    SELECT count(*) INTO n FROM model_promosi p
     WHERE NOT EXISTS (SELECT 1 FROM model_registry m WHERE m.id = p.model_id)
        OR (p.model_digantikan_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM model_registry m WHERE m.id = p.model_digantikan_id));
    IF n > 0 THEN RAISE EXCEPTION 'migrasi 000114: % baris model_promosi menunjuk model_registry yang tidak ada', n; END IF;
    SELECT count(*) INTO n FROM prediksi_harga p
     WHERE NOT EXISTS (SELECT 1 FROM model_registry m WHERE m.komoditas_kode = p.komoditas_kode AND m.versi = p.model_versi);
    IF n > 0 THEN RAISE EXCEPTION 'migrasi 000114: % baris prediksi_harga menunjuk model (komoditas, versi) yang tidak ada', n; END IF;
    SELECT count(*) INTO n FROM prediksi_evaluasi e WHERE NOT EXISTS (SELECT 1 FROM prediksi_harga p WHERE p.id = e.prediksi_id);
    IF n > 0 THEN RAISE EXCEPTION 'migrasi 000114: % baris prediksi_evaluasi menunjuk prediksi_harga yang tidak ada', n; END IF;
    SELECT count(*) INTO n FROM brief_job j WHERE NOT EXISTS (SELECT 1 FROM policy_brief b WHERE b.id = j.brief_id);
    IF n > 0 THEN RAISE EXCEPTION 'migrasi 000114: % baris brief_job menunjuk policy_brief yang tidak ada', n; END IF;
    SELECT count(*) INTO n FROM policy_brief_versi v WHERE NOT EXISTS (SELECT 1 FROM policy_brief b WHERE b.id = v.brief_id);
    IF n > 0 THEN RAISE EXCEPTION 'migrasi 000114: % baris policy_brief_versi menunjuk policy_brief yang tidak ada', n; END IF;

    ---------------------------------------------------------------------------------------------------
    -- 1. model_registry: id varchar -> bigint. Anak: model_promosi (dua kolom) dan prediksi_harga (model_id baru).
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE model_registry ADD COLUMN kode varchar(64);
    UPDATE model_registry SET kode = id;
    ALTER TABLE model_registry ADD COLUMN id_baru bigint;
    UPDATE model_registry m SET id_baru = u.nomor
      FROM (SELECT id, row_number() OVER (ORDER BY dilatih_pada NULLS LAST, id) AS nomor FROM model_registry) u
     WHERE u.id = m.id;

    CREATE FUNCTION pg_temp.g4_id_baru_model(text) RETURNS bigint LANGUAGE sql STABLE
    AS $f$ SELECT id_baru FROM model_registry WHERE id = $1 $f$;

    ALTER TABLE model_promosi DROP CONSTRAINT model_promosi_model_id_fkey;
    ALTER TABLE model_promosi DROP CONSTRAINT model_promosi_model_digantikan_id_fkey;
    ALTER TABLE prediksi_harga DROP CONSTRAINT fk_prediksi_harga_model;

    ALTER TABLE model_promosi ALTER COLUMN model_id TYPE bigint USING pg_temp.g4_id_baru_model(model_id);
    ALTER TABLE model_promosi ALTER COLUMN model_digantikan_id TYPE bigint USING pg_temp.g4_id_baru_model(model_digantikan_id);

    -- prediksi_harga.model_id: dari pasangan lama (komoditas_kode, model_versi), NOT NULL seperti model_versi.
    ALTER TABLE prediksi_harga ADD COLUMN model_id bigint;
    UPDATE prediksi_harga p SET model_id = m.id_baru
      FROM model_registry m WHERE m.komoditas_kode = p.komoditas_kode AND m.versi = p.model_versi;
    ALTER TABLE prediksi_harga ALTER COLUMN model_id SET NOT NULL;

    ALTER TABLE model_registry DROP CONSTRAINT model_registry_pkey;
    ALTER TABLE model_registry DROP COLUMN id;
    ALTER TABLE model_registry RENAME COLUMN id_baru TO id;
    ALTER TABLE model_registry ALTER COLUMN id SET NOT NULL;
    ALTER TABLE model_registry ADD CONSTRAINT model_registry_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO maks FROM model_registry;
    EXECUTE format('ALTER TABLE model_registry ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', maks);
    ALTER TABLE model_registry ADD CONSTRAINT uq_model_registry_kode UNIQUE (kode);

    ALTER TABLE model_registry DROP CONSTRAINT model_registry_komoditas_kode_fkey;
    ALTER TABLE model_registry ADD CONSTRAINT model_registry_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    -- (komoditas_kode, versi) UNIQUE yang sudah ada melayani indeks FK komoditas_kode.

    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES model_registry(id) ON DELETE RESTRICT;
    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_model_digantikan_id_fkey
        FOREIGN KEY (model_digantikan_id) REFERENCES model_registry(id) ON DELETE RESTRICT;
    CREATE INDEX idx_model_promosi_model_digantikan_id ON model_promosi (model_digantikan_id);
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES model_registry(id) ON DELETE RESTRICT;
    CREATE INDEX idx_prediksi_harga_model_id ON prediksi_harga (model_id);

    ---------------------------------------------------------------------------------------------------
    -- 2. prediksi_harga: id varchar -> bigint. Anak: prediksi_evaluasi.
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE prediksi_harga ADD COLUMN kode varchar(64);
    UPDATE prediksi_harga SET kode = id;
    ALTER TABLE prediksi_harga ADD COLUMN id_baru bigint;
    UPDATE prediksi_harga p SET id_baru = u.nomor
      FROM (SELECT id, row_number() OVER (ORDER BY dibuat_pada NULLS LAST, id) AS nomor FROM prediksi_harga) u
     WHERE u.id = p.id;

    CREATE FUNCTION pg_temp.g4_id_baru_prediksi(text) RETURNS bigint LANGUAGE sql STABLE
    AS $f$ SELECT id_baru FROM prediksi_harga WHERE id = $1 $f$;

    ALTER TABLE prediksi_evaluasi DROP CONSTRAINT prediksi_evaluasi_prediksi_id_fkey;
    ALTER TABLE prediksi_evaluasi ALTER COLUMN prediksi_id TYPE bigint USING pg_temp.g4_id_baru_prediksi(prediksi_id);

    ALTER TABLE prediksi_harga DROP CONSTRAINT prediksi_harga_pkey;
    ALTER TABLE prediksi_harga DROP COLUMN id;
    ALTER TABLE prediksi_harga RENAME COLUMN id_baru TO id;
    ALTER TABLE prediksi_harga ALTER COLUMN id SET NOT NULL;
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO maks FROM prediksi_harga;
    EXECUTE format('ALTER TABLE prediksi_harga ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', maks);
    ALTER TABLE prediksi_harga ADD CONSTRAINT uq_prediksi_harga_kode UNIQUE (kode);

    ALTER TABLE prediksi_harga DROP CONSTRAINT prediksi_harga_kabupaten_kode_fkey;
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_kabupaten_kode_fkey
        FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    ALTER TABLE prediksi_harga DROP CONSTRAINT prediksi_harga_komoditas_kode_fkey;
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;

    DROP INDEX idx_prediksi_harga_lookup;
    CREATE INDEX idx_prediksi_harga_terbaru
        ON prediksi_harga (komoditas_kode, kabupaten_kode, horizon_hari, tanggal_target DESC);
    CREATE INDEX idx_prediksi_harga_kabupaten_horizon
        ON prediksi_harga (kabupaten_kode, horizon_hari, komoditas_kode, tanggal_target DESC);

    ---------------------------------------------------------------------------------------------------
    -- 3. prediksi_evaluasi: PK prediksi_id (1:1) -> id bigint identity + UNIQUE (prediksi_id).
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE prediksi_evaluasi DROP CONSTRAINT prediksi_evaluasi_pkey;
    ALTER TABLE prediksi_evaluasi ALTER COLUMN prediksi_id SET NOT NULL;
    ALTER TABLE prediksi_evaluasi ADD COLUMN id bigint;
    UPDATE prediksi_evaluasi e SET id = u.nomor
      FROM (SELECT prediksi_id, row_number() OVER (ORDER BY dievaluasi_pada, prediksi_id) AS nomor FROM prediksi_evaluasi) u
     WHERE u.prediksi_id = e.prediksi_id;
    ALTER TABLE prediksi_evaluasi ALTER COLUMN id SET NOT NULL;
    ALTER TABLE prediksi_evaluasi ADD CONSTRAINT prediksi_evaluasi_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO maks FROM prediksi_evaluasi;
    EXECUTE format('ALTER TABLE prediksi_evaluasi ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', maks);
    ALTER TABLE prediksi_evaluasi ADD CONSTRAINT uq_prediksi_evaluasi_prediksi UNIQUE (prediksi_id);
    ALTER TABLE prediksi_evaluasi ADD CONSTRAINT prediksi_evaluasi_prediksi_id_fkey
        FOREIGN KEY (prediksi_id) REFERENCES prediksi_harga(id) ON DELETE CASCADE;

    ---------------------------------------------------------------------------------------------------
    -- 4. model_promosi: id varchar -> bigint (tanpa anak).
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE model_promosi ADD COLUMN kode varchar(64);
    UPDATE model_promosi SET kode = id;
    ALTER TABLE model_promosi ADD COLUMN id_baru bigint;
    UPDATE model_promosi t SET id_baru = u.nomor
      FROM (SELECT id, row_number() OVER (ORDER BY waktu NULLS LAST, id) AS nomor FROM model_promosi) u
     WHERE u.id = t.id;
    ALTER TABLE model_promosi DROP CONSTRAINT model_promosi_pkey;
    ALTER TABLE model_promosi DROP COLUMN id;
    ALTER TABLE model_promosi RENAME COLUMN id_baru TO id;
    ALTER TABLE model_promosi ALTER COLUMN id SET NOT NULL;
    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO maks FROM model_promosi;
    EXECUTE format('ALTER TABLE model_promosi ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', maks);
    ALTER TABLE model_promosi ADD CONSTRAINT uq_model_promosi_kode UNIQUE (kode);

    ALTER TABLE model_promosi DROP CONSTRAINT model_promosi_komoditas_kode_fkey;
    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    -- idx_model_promosi_komoditas (komoditas_kode, waktu DESC) dan idx_model_promosi_model (model_id, waktu DESC)
    -- sudah berawalan kolom FK.

    ---------------------------------------------------------------------------------------------------
    -- 5. skenario_simulasi: id varchar -> bigint (tanpa anak).
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE skenario_simulasi ADD COLUMN kode varchar(64);
    UPDATE skenario_simulasi SET kode = id;
    ALTER TABLE skenario_simulasi ADD COLUMN id_baru bigint;
    UPDATE skenario_simulasi t SET id_baru = u.nomor
      FROM (SELECT id, row_number() OVER (ORDER BY dibuat_pada NULLS LAST, id) AS nomor FROM skenario_simulasi) u
     WHERE u.id = t.id;
    ALTER TABLE skenario_simulasi DROP CONSTRAINT skenario_simulasi_pkey;
    ALTER TABLE skenario_simulasi DROP COLUMN id;
    ALTER TABLE skenario_simulasi RENAME COLUMN id_baru TO id;
    ALTER TABLE skenario_simulasi ALTER COLUMN id SET NOT NULL;
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO maks FROM skenario_simulasi;
    EXECUTE format('ALTER TABLE skenario_simulasi ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', maks);
    ALTER TABLE skenario_simulasi ADD CONSTRAINT uq_skenario_simulasi_kode UNIQUE (kode);

    ALTER TABLE skenario_simulasi DROP CONSTRAINT skenario_simulasi_komoditas_kode_fkey;
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_komoditas_kode_fkey
        FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode) ON DELETE RESTRICT;
    ALTER TABLE skenario_simulasi DROP CONSTRAINT skenario_simulasi_kabupaten_kode_fkey;
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_kabupaten_kode_fkey
        FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    CREATE INDEX idx_skenario_simulasi_komoditas_kode ON skenario_simulasi (komoditas_kode);
    CREATE INDEX idx_skenario_simulasi_kabupaten_kode ON skenario_simulasi (kabupaten_kode);
    -- idx_skenario_simulasi_pembuat (dibuat_oleh, dibuat_pada DESC) melayani FK dibuat_oleh (000110).

    ---------------------------------------------------------------------------------------------------
    -- 6. policy_brief: id varchar 'PB-YYYY-MM' -> uuid garda_uuid_stabil('policy_brief', id_lama).
    --    Anak: brief_job dan policy_brief_versi (brief_id). Deterministik, jadi induk dan anak bertemu di UUID yang sama.
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE policy_brief ADD COLUMN kode varchar(64);
    UPDATE policy_brief SET kode = id;

    ALTER TABLE brief_job DROP CONSTRAINT brief_job_brief_id_fkey;
    ALTER TABLE policy_brief_versi DROP CONSTRAINT policy_brief_versi_brief_id_fkey;
    ALTER TABLE policy_brief_versi DROP CONSTRAINT policy_brief_versi_pkey;

    ALTER TABLE brief_job ALTER COLUMN brief_id TYPE uuid USING garda_uuid_stabil('policy_brief', brief_id);
    ALTER TABLE policy_brief_versi ALTER COLUMN brief_id TYPE uuid USING garda_uuid_stabil('policy_brief', brief_id);
    ALTER TABLE policy_brief ALTER COLUMN id TYPE uuid USING garda_uuid_stabil('policy_brief', id);
    ALTER TABLE policy_brief ALTER COLUMN id SET DEFAULT gen_random_uuid();
    ALTER TABLE policy_brief ALTER COLUMN kode SET NOT NULL;
    ALTER TABLE policy_brief ADD CONSTRAINT uq_policy_brief_kode UNIQUE (kode);
    ALTER TABLE policy_brief ADD CONSTRAINT chk_policy_brief_kode_format
        CHECK (kode ~ '^PB-[0-9]{4}-(0[1-9]|1[0-2])$');

    ALTER TABLE brief_job ADD CONSTRAINT brief_job_brief_id_fkey
        FOREIGN KEY (brief_id) REFERENCES policy_brief(id) ON DELETE CASCADE;
    -- idx_brief_job_brief (brief_id, mulai DESC) sudah berawalan kolom FK.

    ---------------------------------------------------------------------------------------------------
    -- 7. policy_brief_versi: PK komposit (brief_id, versi) -> id bigint identity + UNIQUE (brief_id, versi).
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE policy_brief_versi ADD COLUMN id bigint;
    UPDATE policy_brief_versi v SET id = u.nomor
      FROM (SELECT brief_id, versi, row_number() OVER (ORDER BY waktu NULLS LAST, brief_id, versi) AS nomor
              FROM policy_brief_versi) u
     WHERE u.brief_id = v.brief_id AND u.versi = v.versi;
    ALTER TABLE policy_brief_versi ALTER COLUMN id SET NOT NULL;
    ALTER TABLE policy_brief_versi ADD CONSTRAINT policy_brief_versi_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO maks FROM policy_brief_versi;
    EXECUTE format('ALTER TABLE policy_brief_versi ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', maks);
    ALTER TABLE policy_brief_versi ADD CONSTRAINT uq_policy_brief_versi_alami UNIQUE (brief_id, versi);
    ALTER TABLE policy_brief_versi ADD CONSTRAINT policy_brief_versi_brief_id_fkey
        FOREIGN KEY (brief_id) REFERENCES policy_brief(id) ON DELETE CASCADE;

    ---------------------------------------------------------------------------------------------------
    -- 8. brief_job: id varchar (UUID) -> bigint (tanpa anak), indeks parsial job BERJALAN.
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE brief_job ADD COLUMN kode varchar(64);
    UPDATE brief_job SET kode = id;
    ALTER TABLE brief_job ADD COLUMN id_baru bigint;
    UPDATE brief_job t SET id_baru = u.nomor
      FROM (SELECT id, row_number() OVER (ORDER BY mulai NULLS LAST, id) AS nomor FROM brief_job) u
     WHERE u.id = t.id;
    ALTER TABLE brief_job DROP CONSTRAINT brief_job_pkey;
    ALTER TABLE brief_job DROP COLUMN id;
    ALTER TABLE brief_job RENAME COLUMN id_baru TO id;
    ALTER TABLE brief_job ALTER COLUMN id SET NOT NULL;
    ALTER TABLE brief_job ADD CONSTRAINT brief_job_pkey PRIMARY KEY (id);
    SELECT COALESCE(max(id), 0) + 1 INTO maks FROM brief_job;
    EXECUTE format('ALTER TABLE brief_job ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (START WITH %s)', maks);
    ALTER TABLE brief_job ADD CONSTRAINT uq_brief_job_kode UNIQUE (kode);
    CREATE INDEX idx_brief_job_berjalan ON brief_job (mulai) WHERE status = 'BERJALAN';

    ---------------------------------------------------------------------------------------------------
    -- 9. Rujukan lunak: audit_log.entitas_id yang berisi ID lama baris yang ada dipetakan ke ID baru.
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    UPDATE audit_log a SET entitas_id = m.id::text
      FROM model_registry m WHERE a.entitas = 'model_registry' AND a.entitas_id = m.kode;
    UPDATE audit_log a SET entitas_id = b.id::text
      FROM policy_brief b WHERE a.entitas = 'policy_brief' AND a.entitas_id = b.kode;
    UPDATE audit_log a SET entitas_id = s.id::text
      FROM skenario_simulasi s WHERE a.entitas = 'skenario_simulasi' AND a.entitas_id = s.kode;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    COMMENT ON COLUMN model_registry.kode IS 'Kode tampilan model (MDL-<komoditas>-<versi>); ID lama sebelum migrasi 000114.';
    COMMENT ON COLUMN prediksi_harga.kode IS 'ID lama proyeksi sebelum migrasi 000114 (PRD-... / PRM-...) dan kunci idempoten seeder; NULL untuk proyeksi hasil pipeline ML.';
    COMMENT ON COLUMN prediksi_harga.model_versi IS 'Versi model (informasi); ditulis konsisten dengan model_id, yang menjadi kunci asing.';
    COMMENT ON COLUMN policy_brief.kode IS 'Kode tampilan PB-YYYY-MM; satu brief per periode. Alias URL /v1/policy-brief/PB-2026-10.';

    DROP FUNCTION pg_temp.g4_id_baru_model(text);
    DROP FUNCTION pg_temp.g4_id_baru_prediksi(text);
END
$mig$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $mig$
DECLARE
    r record;
BEGIN
    -- Rujukan lunak kembali ke ID lama (baris tanpa kode tetap id::text, sama dengan ID lama hasil Down ini).
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    UPDATE audit_log a SET entitas_id = m.kode
      FROM model_registry m WHERE a.entitas = 'model_registry' AND a.entitas_id = m.id::text AND m.kode IS NOT NULL;
    UPDATE audit_log a SET entitas_id = b.kode
      FROM policy_brief b WHERE a.entitas = 'policy_brief' AND a.entitas_id = b.id::text AND b.kode IS NOT NULL;
    UPDATE audit_log a SET entitas_id = s.kode
      FROM skenario_simulasi s WHERE a.entitas = 'skenario_simulasi' AND a.entitas_id = s.id::text AND s.kode IS NOT NULL;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    -- Pemulih ID: ID lama dari kode, atau id::text untuk baris yang dibuat sesudah Up. Fungsi (bukan subkueri)
    -- karena USING pada ALTER COLUMN TYPE tidak boleh berisi subkueri.
    CREATE FUNCTION pg_temp.g4_id_lama_model(bigint) RETURNS varchar(255) LANGUAGE sql STABLE
    AS $f$ SELECT COALESCE(kode, id::text)::varchar(255) FROM model_registry WHERE id = $1 $f$;
    CREATE FUNCTION pg_temp.g4_id_lama_prediksi(bigint) RETURNS varchar(255) LANGUAGE sql STABLE
    AS $f$ SELECT COALESCE(kode, id::text)::varchar(255) FROM prediksi_harga WHERE id = $1 $f$;
    CREATE FUNCTION pg_temp.g4_id_lama_brief(uuid) RETURNS varchar(255) LANGUAGE sql STABLE
    AS $f$ SELECT COALESCE(kode, id::text)::varchar(255) FROM policy_brief WHERE id = $1 $f$;

    ---------------------------------------------------------------------------------------------------
    -- policy_brief dan anaknya: uuid -> varchar 'PB-YYYY-MM'.
    ---------------------------------------------------------------------------------------------------
    -- brief_job: kembali ke id varchar, FK NO ACTION.
    DROP INDEX idx_brief_job_berjalan;
    ALTER TABLE brief_job DROP CONSTRAINT brief_job_brief_id_fkey;
    ALTER TABLE brief_job ALTER COLUMN brief_id TYPE varchar(255) USING pg_temp.g4_id_lama_brief(brief_id);
    ALTER TABLE brief_job ADD COLUMN id_lama varchar(255);
    UPDATE brief_job SET id_lama = COALESCE(kode, id::text);
    ALTER TABLE brief_job DROP CONSTRAINT brief_job_pkey;
    ALTER TABLE brief_job DROP CONSTRAINT uq_brief_job_kode;
    ALTER TABLE brief_job DROP COLUMN id;
    ALTER TABLE brief_job RENAME COLUMN id_lama TO id;
    ALTER TABLE brief_job ALTER COLUMN id SET NOT NULL;
    ALTER TABLE brief_job ADD CONSTRAINT brief_job_pkey PRIMARY KEY (id);
    ALTER TABLE brief_job DROP COLUMN kode;

    -- policy_brief_versi: kembali ke PK komposit (brief_id, versi), FK NO ACTION.
    ALTER TABLE policy_brief_versi DROP CONSTRAINT policy_brief_versi_brief_id_fkey;
    ALTER TABLE policy_brief_versi DROP CONSTRAINT uq_policy_brief_versi_alami;
    ALTER TABLE policy_brief_versi DROP CONSTRAINT policy_brief_versi_pkey;
    ALTER TABLE policy_brief_versi DROP COLUMN id;
    ALTER TABLE policy_brief_versi ALTER COLUMN brief_id TYPE varchar(255) USING pg_temp.g4_id_lama_brief(brief_id);
    ALTER TABLE policy_brief_versi ADD CONSTRAINT policy_brief_versi_pkey PRIMARY KEY (brief_id, versi);

    ALTER TABLE policy_brief DROP CONSTRAINT chk_policy_brief_kode_format;
    ALTER TABLE policy_brief DROP CONSTRAINT uq_policy_brief_kode;
    ALTER TABLE policy_brief ALTER COLUMN id DROP DEFAULT;
    ALTER TABLE policy_brief ALTER COLUMN id TYPE varchar(255) USING COALESCE(kode, id::text);
    ALTER TABLE policy_brief DROP COLUMN kode;
    ALTER TABLE brief_job ADD CONSTRAINT brief_job_brief_id_fkey FOREIGN KEY (brief_id) REFERENCES policy_brief(id);
    ALTER TABLE policy_brief_versi ADD CONSTRAINT policy_brief_versi_brief_id_fkey FOREIGN KEY (brief_id) REFERENCES policy_brief(id);

    ---------------------------------------------------------------------------------------------------
    -- prediksi_evaluasi dan prediksi_harga: bigint -> varchar.
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE prediksi_evaluasi DROP CONSTRAINT prediksi_evaluasi_prediksi_id_fkey;
    ALTER TABLE prediksi_evaluasi DROP CONSTRAINT uq_prediksi_evaluasi_prediksi;
    ALTER TABLE prediksi_evaluasi DROP CONSTRAINT prediksi_evaluasi_pkey;
    ALTER TABLE prediksi_evaluasi DROP COLUMN id;
    ALTER TABLE prediksi_evaluasi ALTER COLUMN prediksi_id TYPE varchar(255) USING pg_temp.g4_id_lama_prediksi(prediksi_id);
    ALTER TABLE prediksi_evaluasi ADD CONSTRAINT prediksi_evaluasi_pkey PRIMARY KEY (prediksi_id);

    ALTER TABLE prediksi_harga DROP CONSTRAINT prediksi_harga_model_id_fkey;
    DROP INDEX idx_prediksi_harga_model_id;
    DROP INDEX idx_prediksi_harga_terbaru;
    DROP INDEX idx_prediksi_harga_kabupaten_horizon;
    ALTER TABLE prediksi_harga DROP COLUMN model_id;
    CREATE INDEX idx_prediksi_harga_lookup ON prediksi_harga (komoditas_kode, kabupaten_kode, tanggal_target);

    ALTER TABLE prediksi_harga DROP CONSTRAINT prediksi_harga_kabupaten_kode_fkey;
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE prediksi_harga DROP CONSTRAINT prediksi_harga_komoditas_kode_fkey;
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);

    ALTER TABLE prediksi_harga ADD COLUMN id_lama varchar(255);
    UPDATE prediksi_harga SET id_lama = COALESCE(kode, id::text);
    ALTER TABLE prediksi_harga DROP CONSTRAINT prediksi_harga_pkey;
    ALTER TABLE prediksi_harga DROP CONSTRAINT uq_prediksi_harga_kode;
    ALTER TABLE prediksi_harga DROP COLUMN id;
    ALTER TABLE prediksi_harga RENAME COLUMN id_lama TO id;
    ALTER TABLE prediksi_harga ALTER COLUMN id SET NOT NULL;
    ALTER TABLE prediksi_harga ADD CONSTRAINT prediksi_harga_pkey PRIMARY KEY (id);
    ALTER TABLE prediksi_harga DROP COLUMN kode;
    ALTER TABLE prediksi_evaluasi ADD CONSTRAINT prediksi_evaluasi_prediksi_id_fkey
        FOREIGN KEY (prediksi_id) REFERENCES prediksi_harga(id) ON DELETE CASCADE;

    ---------------------------------------------------------------------------------------------------
    -- model_promosi, skenario_simulasi, model_registry: bigint -> varchar.
    ---------------------------------------------------------------------------------------------------
    ALTER TABLE model_promosi DROP CONSTRAINT model_promosi_model_id_fkey;
    ALTER TABLE model_promosi DROP CONSTRAINT model_promosi_model_digantikan_id_fkey;
    DROP INDEX idx_model_promosi_model_digantikan_id;
    ALTER TABLE model_promosi ALTER COLUMN model_id TYPE varchar(255) USING pg_temp.g4_id_lama_model(model_id);
    ALTER TABLE model_promosi ALTER COLUMN model_digantikan_id TYPE varchar(255) USING pg_temp.g4_id_lama_model(model_digantikan_id);

    ALTER TABLE model_promosi ADD COLUMN id_lama varchar(255);
    UPDATE model_promosi SET id_lama = COALESCE(kode, id::text);
    ALTER TABLE model_promosi DROP CONSTRAINT model_promosi_pkey;
    ALTER TABLE model_promosi DROP CONSTRAINT uq_model_promosi_kode;
    ALTER TABLE model_promosi DROP COLUMN id;
    ALTER TABLE model_promosi RENAME COLUMN id_lama TO id;
    ALTER TABLE model_promosi ALTER COLUMN id SET NOT NULL;
    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_pkey PRIMARY KEY (id);
    ALTER TABLE model_promosi DROP COLUMN kode;
    ALTER TABLE model_promosi DROP CONSTRAINT model_promosi_komoditas_kode_fkey;
    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);

    DROP INDEX idx_skenario_simulasi_komoditas_kode;
    DROP INDEX idx_skenario_simulasi_kabupaten_kode;
    ALTER TABLE skenario_simulasi ADD COLUMN id_lama varchar(255);
    UPDATE skenario_simulasi SET id_lama = COALESCE(kode, id::text);
    ALTER TABLE skenario_simulasi DROP CONSTRAINT skenario_simulasi_pkey;
    ALTER TABLE skenario_simulasi DROP CONSTRAINT uq_skenario_simulasi_kode;
    ALTER TABLE skenario_simulasi DROP COLUMN id;
    ALTER TABLE skenario_simulasi RENAME COLUMN id_lama TO id;
    ALTER TABLE skenario_simulasi ALTER COLUMN id SET NOT NULL;
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_pkey PRIMARY KEY (id);
    ALTER TABLE skenario_simulasi DROP COLUMN kode;
    ALTER TABLE skenario_simulasi DROP CONSTRAINT skenario_simulasi_komoditas_kode_fkey;
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);
    ALTER TABLE skenario_simulasi DROP CONSTRAINT skenario_simulasi_kabupaten_kode_fkey;
    ALTER TABLE skenario_simulasi ADD CONSTRAINT skenario_simulasi_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);

    ALTER TABLE model_registry ADD COLUMN id_lama varchar(255);
    UPDATE model_registry SET id_lama = COALESCE(kode, id::text);
    ALTER TABLE model_registry DROP CONSTRAINT model_registry_pkey;
    ALTER TABLE model_registry DROP CONSTRAINT uq_model_registry_kode;
    ALTER TABLE model_registry DROP COLUMN id;
    ALTER TABLE model_registry RENAME COLUMN id_lama TO id;
    ALTER TABLE model_registry ALTER COLUMN id SET NOT NULL;
    ALTER TABLE model_registry ADD CONSTRAINT model_registry_pkey PRIMARY KEY (id);
    ALTER TABLE model_registry DROP COLUMN kode;
    ALTER TABLE model_registry DROP CONSTRAINT model_registry_komoditas_kode_fkey;
    ALTER TABLE model_registry ADD CONSTRAINT model_registry_komoditas_kode_fkey FOREIGN KEY (komoditas_kode) REFERENCES komoditas(kode);

    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_model_id_fkey FOREIGN KEY (model_id) REFERENCES model_registry(id);
    ALTER TABLE model_promosi ADD CONSTRAINT model_promosi_model_digantikan_id_fkey FOREIGN KEY (model_digantikan_id) REFERENCES model_registry(id);
    ALTER TABLE prediksi_harga ADD CONSTRAINT fk_prediksi_harga_model
        FOREIGN KEY (komoditas_kode, model_versi) REFERENCES model_registry (komoditas_kode, versi);

    COMMENT ON COLUMN prediksi_harga.model_versi IS NULL;

    DROP FUNCTION pg_temp.g4_id_lama_model(bigint);
    DROP FUNCTION pg_temp.g4_id_lama_prediksi(bigint);
    DROP FUNCTION pg_temp.g4_id_lama_brief(uuid);
END
$mig$;
-- +goose StatementEnd
