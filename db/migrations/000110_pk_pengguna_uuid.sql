-- Standardisasi kunci primer (docs/SKEMA-DATABASE.md), langkah 1: pengguna.id VARCHAR(255) -> UUID beserta
-- seluruh kolom yang merujuknya.
--
-- ID lama adalah slug buatan aplikasi (superadmin, agregator_data, mahasiswa_9, test-user-001, USR-...).
-- ID baru = garda_uuid_stabil('pengguna', id_lama) (migrasi 000109): deterministik, jadi basis data yang
-- dimigrasi dan basis data baru yang di-seed (idx.UUIDStabil("pengguna", "<slug>")) memakai UUID yang sama.
-- Akun yang dibuat sesudah migrasi memakai gen_random_uuid().
--
-- Yang dilakukan, semuanya dalam satu transaksi:
--   1. pengguna_id_lama (id uuid, id_lama) menyimpan ID lama untuk Down.
--   2. Seluruh FK ke pengguna(id) (41 kolom di 32 tabel) dilepas, kolomnya dikonversi ke uuid dengan rumus
--      yang sama (USING garda_uuid_stabil), lalu FK dipasang lagi dengan ON DELETE eksplisit (SKEMA §2):
--        CASCADE   milik akun: sesi, token, kode MFA, izin langsung, undangan, preferensi, push, notifikasi,
--                  penugasan PJ intervensi;
--        SET NULL  jejak pelaku opsional yang tidak boleh menghalangi penghapusan akun (audit, log eksekusi,
--                  pengakuan/eskalasi alert, riwayat EWS/intervensi, siapa yang mengubah aturan/izin/peran);
--        RESTRICT  bukti asal data: penginput, validator, penerbit, penyusun, pembuat, pengaju, pemromosi.
--      Aturan yang sebelumnya CASCADE atau SET NULL dipertahankan. NO ACTION diganti: RESTRICT (perilaku
--      sama, tetapi eksplisit) untuk bukti asal data, SET NULL hanya untuk eksekusi_sumber.aktor_id (eksekusi
--      terjadwal tidak punya aktor, dan log eksekusi tidak boleh menahan penghapusan akun) dan
--      ews_alert.diakui_oleh_id (rekam jejak perubahan status tetap di ews_riwayat.oleh yang sudah SET NULL).
--   3. Indeks B-tree idx_<tabel>_<kolom> untuk setiap kolom FK ke pengguna yang belum punya indeks berawalan
--      kolom itu. pengguna.kabupaten_kode ikut dijadikan RESTRICT dan diberi indeks.
--   4. Rujukan lunak (tanpa FK): audit_log.entitas_id pada baris entitas = 'pengguna' dipetakan ke UUID bila
--      nilainya ID lama akun yang ada (dipakai riwayat peran dan filter audit). audit_log.sebelum/sesudah/
--      metadata dibiarkan sebagai sejarah (snapshot JSON; riwayat peran membaca isi peran, bukan ID di dalamnya).
--      audit_log append-only: pemicunya dimatikan hanya selama UPDATE pemetaan ini, di transaksi yang sama.
--      mfa_kode_cadangan.kode_hash dihitung dari "<id pengguna>:<kode>" sehingga tidak bisa dihitung ulang;
--      aplikasi menerima hash dengan ID lama lewat pengguna_id_lama (usecase.verifySecondFactor).
--
-- ALTER COLUMN TYPE menulis ulang tabel dengan kunci ACCESS EXCLUSIVE; jalankan di jendela pemeliharaan
-- untuk basis data besar (entri_harga, audit_log).
--
-- Down mengembalikan VARCHAR(255) dan ID lama dari pengguna_id_lama (baris yang dibuat sesudah Up memakai
-- id::text), aturan ON DELETE semula, lalu menghapus indeks baru dan tabel pemetaan.

-- +goose Up
-- +goose StatementBegin
DO $mig$
DECLARE
    r record;
BEGIN
    -- Rencana per kolom FK: tabel, kolom, nama constraint, ON DELETE baru, perlu indeks baru.
    CREATE TEMP TABLE rencana_fk_pengguna (
        tabel       text    NOT NULL,
        kolom       text    NOT NULL,
        nama        text    NOT NULL,
        hapus_baru  text    NOT NULL,
        buat_indeks boolean NOT NULL
    ) ON COMMIT DROP;

    INSERT INTO rencana_fk_pengguna (tabel, kolom, nama, hapus_baru, buat_indeks) VALUES
        ('aturan_ews',                     'diubah_oleh_id',    'aturan_ews_diubah_oleh_id_fkey',            'SET NULL', true),
        ('aturan_ews_riwayat',             'diubah_oleh_id',    'aturan_ews_riwayat_diubah_oleh_id_fkey',    'SET NULL', true),
        ('audit_log',                      'aktor_id',          'audit_log_aktor_id_fkey',                   'SET NULL', false),
        ('eksekusi_sumber',                'aktor_id',          'eksekusi_sumber_aktor_id_fkey',             'SET NULL', true),
        ('entri_harga',                    'dipublikasi_oleh',  'entri_harga_dipublikasi_oleh_fkey',         'RESTRICT', true),
        ('entri_harga',                    'ditolak_oleh',      'entri_harga_ditolak_oleh_fkey',             'RESTRICT', true),
        ('entri_harga',                    'divalidasi_oleh',   'entri_harga_divalidasi_oleh_fkey',          'RESTRICT', true),
        ('entri_harga',                    'petugas_id',        'entri_harga_petugas_id_fkey',               'RESTRICT', false),
        ('ews_alert',                      'diakui_oleh_id',    'ews_alert_diakui_oleh_id_fkey',             'SET NULL', true),
        ('ews_alert',                      'dieskalasi_oleh_id','ews_alert_dieskalasi_oleh_id_fkey',         'SET NULL', true),
        ('ews_notifikasi',                 'pengguna_id',       'ews_notifikasi_pengguna_id_fkey',           'CASCADE',  true),
        ('ews_riwayat',                    'oleh',              'ews_riwayat_oleh_fkey',                     'SET NULL', true),
        ('intervensi',                     'dibuat_oleh_id',    'intervensi_dibuat_oleh_id_fkey',            'RESTRICT', true),
        ('intervensi',                     'dieskalasi_oleh_id','intervensi_dieskalasi_oleh_id_fkey',        'SET NULL', true),
        ('intervensi_persetujuan',         'diajukan_oleh',     'intervensi_persetujuan_diajukan_oleh_fkey', 'SET NULL', true),
        ('intervensi_persetujuan_langkah', 'oleh',              'intervensi_persetujuan_langkah_oleh_fkey',  'SET NULL', true),
        ('intervensi_pj',                  'pengguna_id',       'intervensi_pj_pengguna_id_fkey',            'CASCADE',  true),
        ('intervensi_riwayat',             'oleh',              'intervensi_riwayat_oleh_fkey',              'SET NULL', true),
        ('langganan_push',                 'pengguna_id',       'langganan_push_pengguna_id_fkey',           'CASCADE',  false),
        ('login_attempts',                 'user_id',           'login_attempts_user_id_fkey',               'SET NULL', false),
        ('mfa_kode_cadangan',              'user_id',           'mfa_kode_cadangan_user_id_fkey',            'CASCADE',  false),
        ('model_promosi',                  'oleh',              'model_promosi_oleh_fkey',                   'RESTRICT', true),
        ('model_registry',                 'dipromosikan_oleh', 'model_registry_dipromosikan_oleh_fkey',     'RESTRICT', true),
        ('optimasi_rute',                  'dibuat_oleh',       'optimasi_rute_dibuat_oleh_fkey',            'SET NULL', true),
        ('password_reset_tokens',          'user_id',           'password_reset_tokens_user_id_fkey',        'CASCADE',  false),
        ('perubahan_master',               'diajukan_oleh_id',  'perubahan_master_diajukan_oleh_id_fkey',    'RESTRICT', true),
        ('perubahan_master',               'disetujui_oleh_id', 'perubahan_master_disetujui_oleh_id_fkey',   'RESTRICT', true),
        ('policy_brief',                   'disusun_oleh_id',   'policy_brief_disusun_oleh_id_fkey',         'RESTRICT', true),
        ('policy_brief_versi',             'oleh',              'policy_brief_versi_oleh_fkey',              'RESTRICT', true),
        ('preferensi_notifikasi',          'pengguna_id',       'preferensi_notifikasi_pengguna_id_fkey',    'CASCADE',  false),
        ('refresh_tokens',                 'user_id',           'refresh_tokens_user_id_fkey',               'CASCADE',  false),
        ('riwayat_status_entri',           'oleh',              'riwayat_status_entri_oleh_fkey',            'RESTRICT', true),
        ('roles',                          'diperbarui_oleh',   'roles_diperbarui_oleh_fkey',                'SET NULL', true),
        ('skenario_simulasi',              'dibuat_oleh',       'skenario_simulasi_dibuat_oleh_fkey',        'RESTRICT', false),
        ('transaksi_stok',                 'oleh_id',           'transaksi_stok_oleh_id_fkey',               'SET NULL', true),
        ('tugas_lapangan',                 'dibuat_oleh_id',    'tugas_lapangan_dibuat_oleh_id_fkey',        'RESTRICT', true),
        ('tugas_lapangan',                 'petugas_id',        'tugas_lapangan_petugas_id_fkey',            'RESTRICT', false),
        ('undangan_pengguna',              'diundang_oleh',     'undangan_pengguna_diundang_oleh_fkey',      'SET NULL', true),
        ('undangan_pengguna',              'user_id',           'undangan_pengguna_user_id_fkey',            'CASCADE',  false),
        ('user_permissions',               'diberikan_oleh',    'user_permissions_diberikan_oleh_fkey',      'SET NULL', true),
        ('user_permissions',               'pengguna_id',       'user_permissions_pengguna_id_fkey',         'CASCADE',  false);

    -- Penjaga: setiap FK ke pengguna di katalog harus ada di rencana. FK yang terlewat akan membuat
    -- konversi tipe gagal di tengah jalan, jadi lebih baik berhenti dengan pesan yang jelas.
    FOR r IN
        SELECT con.conrelid::regclass::text AS tabel, con.conname AS nama
          FROM pg_constraint con
         WHERE con.contype = 'f' AND con.confrelid = 'pengguna'::regclass
           AND NOT EXISTS (SELECT 1 FROM rencana_fk_pengguna p
                            WHERE p.tabel = con.conrelid::regclass::text AND p.nama = con.conname)
    LOOP
        RAISE EXCEPTION 'FK % pada % merujuk pengguna tetapi belum ada di rencana migrasi 000110', r.nama, r.tabel;
    END LOOP;

    -- 1. Pemetaan ID lama. FK ke pengguna dipasang sesudah pengguna.id menjadi uuid.
    CREATE TABLE pengguna_id_lama (
        id      uuid         PRIMARY KEY,
        id_lama varchar(255) NOT NULL UNIQUE
    );
    INSERT INTO pengguna_id_lama (id, id_lama)
    SELECT garda_uuid_stabil('pengguna', id), id FROM pengguna;

    -- 2. Lepas FK dan CHECK yang membandingkan dua kolom yang akan berubah tipe.
    ALTER TABLE entri_harga DROP CONSTRAINT IF EXISTS chk_entri_harga_pemisahan_tugas;
    ALTER TABLE pengguna DROP CONSTRAINT pengguna_kabupaten_kode_fkey;
    FOR r IN SELECT * FROM rencana_fk_pengguna LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.nama);
    END LOOP;

    -- 3. Konversi tipe. Rumusnya deterministik, jadi anak dan induk bertemu di UUID yang sama; NULL tetap NULL.
    ALTER TABLE pengguna ALTER COLUMN id TYPE uuid USING garda_uuid_stabil('pengguna', id);
    ALTER TABLE pengguna ALTER COLUMN id SET DEFAULT gen_random_uuid();
    FOR r IN SELECT * FROM rencana_fk_pengguna LOOP
        EXECUTE format('ALTER TABLE %I ALTER COLUMN %I TYPE uuid USING garda_uuid_stabil(''pengguna'', %I)',
                       r.tabel, r.kolom, r.kolom);
    END LOOP;

    ALTER TABLE pengguna_id_lama
        ADD CONSTRAINT pengguna_id_lama_id_fkey FOREIGN KEY (id) REFERENCES pengguna(id) ON DELETE CASCADE;

    -- 4. Pasang FK dengan ON DELETE eksplisit, lalu indeks yang belum ada.
    FOR r IN SELECT * FROM rencana_fk_pengguna LOOP
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES pengguna(id) ON DELETE %s',
                       r.tabel, r.nama, r.kolom, r.hapus_baru);
        IF r.buat_indeks THEN
            EXECUTE format('CREATE INDEX %I ON %I (%I)', 'idx_' || r.tabel || '_' || r.kolom, r.tabel, r.kolom);
        END IF;
    END LOOP;

    ALTER TABLE pengguna
        ADD CONSTRAINT pengguna_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode) ON DELETE RESTRICT;
    CREATE INDEX idx_pengguna_kabupaten_kode ON pengguna (kabupaten_kode);

    -- Pemisahan tugas: penginput tidak boleh memvalidasi entrinya sendiri. NOT VALID seperti aslinya (000061):
    -- baris lama tidak diperiksa ulang, setiap tulis baru wajib patuh.
    ALTER TABLE entri_harga
        ADD CONSTRAINT chk_entri_harga_pemisahan_tugas
        CHECK (divalidasi_oleh IS NULL OR divalidasi_oleh IS DISTINCT FROM petugas_id) NOT VALID;

    -- 5. Rujukan lunak: entitas_id baris audit entitas 'pengguna' yang berisi ID lama akun yang ada. Nilai lain
    --    (mis. 'ekspor', 'impor', '3 pengguna', 'sso_google') bukan ID dan tidak disentuh.
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    UPDATE audit_log a
       SET entitas_id = l.id::text
      FROM pengguna_id_lama l
     WHERE a.entitas = 'pengguna' AND a.entitas_id = l.id_lama;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    COMMENT ON TABLE pengguna_id_lama IS
        'ID lama (slug) tiap pengguna sebelum migrasi 000110; 1:1 dengan pengguna. Dipakai Down dan kompatibilitas hash kode cadangan MFA.';
END
$mig$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $mig$
DECLARE
    r record;
BEGIN
    -- Rencana per kolom FK: tabel, kolom, nama constraint, ON DELETE semula, indeks baru yang dibuat Up.
    CREATE TEMP TABLE rencana_fk_pengguna (
        tabel       text    NOT NULL,
        kolom       text    NOT NULL,
        nama        text    NOT NULL,
        hapus_lama  text    NOT NULL,
        buat_indeks boolean NOT NULL
    ) ON COMMIT DROP;

    INSERT INTO rencana_fk_pengguna (tabel, kolom, nama, hapus_lama, buat_indeks) VALUES
        ('aturan_ews',                     'diubah_oleh_id',    'aturan_ews_diubah_oleh_id_fkey',            'SET NULL',  true),
        ('aturan_ews_riwayat',             'diubah_oleh_id',    'aturan_ews_riwayat_diubah_oleh_id_fkey',    'SET NULL',  true),
        ('audit_log',                      'aktor_id',          'audit_log_aktor_id_fkey',                   'SET NULL',  false),
        ('eksekusi_sumber',                'aktor_id',          'eksekusi_sumber_aktor_id_fkey',             'NO ACTION', true),
        ('entri_harga',                    'dipublikasi_oleh',  'entri_harga_dipublikasi_oleh_fkey',         'NO ACTION', true),
        ('entri_harga',                    'ditolak_oleh',      'entri_harga_ditolak_oleh_fkey',             'NO ACTION', true),
        ('entri_harga',                    'divalidasi_oleh',   'entri_harga_divalidasi_oleh_fkey',          'NO ACTION', true),
        ('entri_harga',                    'petugas_id',        'entri_harga_petugas_id_fkey',               'NO ACTION', false),
        ('ews_alert',                      'diakui_oleh_id',    'ews_alert_diakui_oleh_id_fkey',             'NO ACTION', true),
        ('ews_alert',                      'dieskalasi_oleh_id','ews_alert_dieskalasi_oleh_id_fkey',         'SET NULL',  true),
        ('ews_notifikasi',                 'pengguna_id',       'ews_notifikasi_pengguna_id_fkey',           'CASCADE',   true),
        ('ews_riwayat',                    'oleh',              'ews_riwayat_oleh_fkey',                     'SET NULL',  true),
        ('intervensi',                     'dibuat_oleh_id',    'intervensi_dibuat_oleh_id_fkey',            'NO ACTION', true),
        ('intervensi',                     'dieskalasi_oleh_id','intervensi_dieskalasi_oleh_id_fkey',        'SET NULL',  true),
        ('intervensi_persetujuan',         'diajukan_oleh',     'intervensi_persetujuan_diajukan_oleh_fkey', 'SET NULL',  true),
        ('intervensi_persetujuan_langkah', 'oleh',              'intervensi_persetujuan_langkah_oleh_fkey',  'SET NULL',  true),
        ('intervensi_pj',                  'pengguna_id',       'intervensi_pj_pengguna_id_fkey',            'CASCADE',   true),
        ('intervensi_riwayat',             'oleh',              'intervensi_riwayat_oleh_fkey',              'SET NULL',  true),
        ('langganan_push',                 'pengguna_id',       'langganan_push_pengguna_id_fkey',           'CASCADE',   false),
        ('login_attempts',                 'user_id',           'login_attempts_user_id_fkey',               'SET NULL',  false),
        ('mfa_kode_cadangan',              'user_id',           'mfa_kode_cadangan_user_id_fkey',            'CASCADE',   false),
        ('model_promosi',                  'oleh',              'model_promosi_oleh_fkey',                   'NO ACTION', true),
        ('model_registry',                 'dipromosikan_oleh', 'model_registry_dipromosikan_oleh_fkey',     'NO ACTION', true),
        ('optimasi_rute',                  'dibuat_oleh',       'optimasi_rute_dibuat_oleh_fkey',            'SET NULL',  true),
        ('password_reset_tokens',          'user_id',           'password_reset_tokens_user_id_fkey',        'CASCADE',   false),
        ('perubahan_master',               'diajukan_oleh_id',  'perubahan_master_diajukan_oleh_id_fkey',    'NO ACTION', true),
        ('perubahan_master',               'disetujui_oleh_id', 'perubahan_master_disetujui_oleh_id_fkey',   'NO ACTION', true),
        ('policy_brief',                   'disusun_oleh_id',   'policy_brief_disusun_oleh_id_fkey',         'NO ACTION', true),
        ('policy_brief_versi',             'oleh',              'policy_brief_versi_oleh_fkey',              'NO ACTION', true),
        ('preferensi_notifikasi',          'pengguna_id',       'preferensi_notifikasi_pengguna_id_fkey',    'CASCADE',   false),
        ('refresh_tokens',                 'user_id',           'refresh_tokens_user_id_fkey',               'CASCADE',   false),
        ('riwayat_status_entri',           'oleh',              'riwayat_status_entri_oleh_fkey',            'NO ACTION', true),
        ('roles',                          'diperbarui_oleh',   'roles_diperbarui_oleh_fkey',                'SET NULL',  true),
        ('skenario_simulasi',              'dibuat_oleh',       'skenario_simulasi_dibuat_oleh_fkey',        'NO ACTION', false),
        ('transaksi_stok',                 'oleh_id',           'transaksi_stok_oleh_id_fkey',               'SET NULL',  true),
        ('tugas_lapangan',                 'dibuat_oleh_id',    'tugas_lapangan_dibuat_oleh_id_fkey',        'NO ACTION', true),
        ('tugas_lapangan',                 'petugas_id',        'tugas_lapangan_petugas_id_fkey',            'NO ACTION', false),
        ('undangan_pengguna',              'diundang_oleh',     'undangan_pengguna_diundang_oleh_fkey',      'SET NULL',  true),
        ('undangan_pengguna',              'user_id',           'undangan_pengguna_user_id_fkey',            'CASCADE',   false),
        ('user_permissions',               'diberikan_oleh',    'user_permissions_diberikan_oleh_fkey',      'SET NULL',  true),
        ('user_permissions',               'pengguna_id',       'user_permissions_pengguna_id_fkey',         'CASCADE',   false);

    -- Pemulih ID: ID lama dari pemetaan, atau id::text untuk akun yang dibuat sesudah Up. Fungsi (bukan subkueri)
    -- karena USING pada ALTER COLUMN TYPE tidak boleh berisi subkueri.
    CREATE FUNCTION garda_id_lama_pengguna(u uuid) RETURNS varchar(255)
    LANGUAGE sql VOLATILE
    AS $fn$
        SELECT COALESCE((SELECT id_lama FROM pengguna_id_lama WHERE id = u), u::text)::varchar(255)
    $fn$;

    -- 1. Lepas FK, indeks baru, dan CHECK sebelum tipe kolom berubah.
    ALTER TABLE entri_harga DROP CONSTRAINT IF EXISTS chk_entri_harga_pemisahan_tugas;
    ALTER TABLE pengguna_id_lama DROP CONSTRAINT pengguna_id_lama_id_fkey;
    ALTER TABLE pengguna DROP CONSTRAINT pengguna_kabupaten_kode_fkey;
    DROP INDEX IF EXISTS idx_pengguna_kabupaten_kode;
    FOR r IN SELECT * FROM rencana_fk_pengguna LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.tabel, r.nama);
        IF r.buat_indeks THEN
            EXECUTE format('DROP INDEX IF EXISTS %I', 'idx_' || r.tabel || '_' || r.kolom);
        END IF;
    END LOOP;

    -- 2. Kembalikan tipe dan nilai lama.
    FOR r IN SELECT * FROM rencana_fk_pengguna LOOP
        EXECUTE format('ALTER TABLE %I ALTER COLUMN %I TYPE varchar(255) USING garda_id_lama_pengguna(%I)',
                       r.tabel, r.kolom, r.kolom);
    END LOOP;
    ALTER TABLE pengguna ALTER COLUMN id DROP DEFAULT;
    ALTER TABLE pengguna ALTER COLUMN id TYPE varchar(255) USING garda_id_lama_pengguna(id);

    -- 3. Rujukan lunak kembali ke ID lama.
    ALTER TABLE audit_log DISABLE TRIGGER trg_audit_log_append_only;
    UPDATE audit_log a
       SET entitas_id = l.id_lama
      FROM pengguna_id_lama l
     WHERE a.entitas = 'pengguna' AND a.entitas_id = l.id::text;
    ALTER TABLE audit_log ENABLE TRIGGER trg_audit_log_append_only;

    -- 4. FK dengan aturan semula (NO ACTION = bawaan PostgreSQL).
    FOR r IN SELECT * FROM rencana_fk_pengguna LOOP
        EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES pengguna(id) ON DELETE %s',
                       r.tabel, r.nama, r.kolom, r.hapus_lama);
    END LOOP;
    ALTER TABLE pengguna
        ADD CONSTRAINT pengguna_kabupaten_kode_fkey FOREIGN KEY (kabupaten_kode) REFERENCES kabupaten(kode);
    ALTER TABLE entri_harga
        ADD CONSTRAINT chk_entri_harga_pemisahan_tugas
        CHECK (divalidasi_oleh IS NULL OR divalidasi_oleh IS DISTINCT FROM petugas_id) NOT VALID;

    DROP TABLE pengguna_id_lama;
    DROP FUNCTION garda_id_lama_pengguna(uuid);
END
$mig$;
-- +goose StatementEnd
