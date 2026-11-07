# Skema database: kunci primer, kunci asing, dan indeks

Dokumen ini adalah kontrak skema PostgreSQL Garda Inflasi: cara memilih kunci primer (PK), aturan kunci asing
(FK), strategi indeks, dan cara kode Go, API, BFF, serta seeder memperlakukan ID. Kamus data per tabel ada di
bagian [Matriks tabel](#matriks-tabel).

Sumber kebenaran yang dijalankan mesin:

- **Matriks kategori PK**: `kategoriPK` di [`apps/api/internal/platform/schematest/kunci_test.go`](../apps/api/internal/platform/schematest/kunci_test.go). Tes `TestInvarianKunci` memeriksa invarian di bawah terhadap skema hasil migrasi.
- **Bentuk ID di Go dan JSON**: paket [`apps/api/pkg/idx`](../apps/api/pkg/idx/idx.go).
- **UUID deterministik untuk seed dan migrasi data**: `idx.UUIDStabil` di Go dan `garda_uuid_stabil` di SQL (migrasi `000109_pk_fondasi.sql`), yang diuji sama persis oleh `TestUUIDStabil_SamaDenganSQL`.

## 1. Kategori kunci primer

| Kategori     | Bentuk kolom                                                                              | Dipakai untuk                                                                                                                                                                                                                                                                                                         | Alasan                                                                                                                                                                     |
| ------------ | ----------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **UUID**     | `id uuid PRIMARY KEY DEFAULT gen_random_uuid()`                                           | Entitas bisnis utama dan identitas yang terlihat klien: `pengguna`, `intervensi`, `policy_brief`, `sumber_data`, `cold_storage`                                                                                                                                                                                       | Tidak bisa ditebak atau dienumerasi dari URL. Bisa dibuat di mana saja tanpa koordinasi. Tidak membocorkan volume atau urutan pembuatan.                                   |
| **BIGINT**   | `id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY` (tabel lama boleh tetap `BIGSERIAL`) | Tabel transaksional, time-series, log, dan operasional internal                                                                                                                                                                                                                                                       | Ringkas (8 byte) dan cepat untuk join. Indeks B-tree tumbuh di ujung kanan karena nilainya menaik, jadi insert murah. `ALWAYS` membuat basis data satu-satunya pemberi ID. |
| **KODE**     | Satu kolom kode alami (`varchar`, enum, atau `integer` tahun)                             | Referensi nasional dan master berkode: `provinsi` dan `kabupaten` (kode BPS), `komoditas`, `pasar`, `permissions`, `roles`, `kpi_definisi`, `alur_persetujuan`, `target_inflasi`, `wilayah_geometri` (1:1 dengan kabupaten)                                                                                           | Kode itu sendiri identitasnya, stabil lintas sistem (BPS, PIHPS), dan dipakai langsung sebagai nilai FK yang mudah dibaca.                                                 |
| **HASH**     | Satu kolom hash atau token id                                                             | Tabel keamanan: `jwt_blacklist` (jti), `kunci_login` (hash email), `sso_login_state` (hash state)                                                                                                                                                                                                                     | Kuncinya memang nilai yang dicari. Tidak ada entitas bisnis.                                                                                                               |
| **KOMPOSIT** | Gabungan kolom FK atau induk + urutan                                                     | Tabel penghubung (`role_permissions`, `user_permissions`, `intervensi_pj`, `tugas_entri`, `preferensi_notifikasi`, `pemetaan_sumber`) dan entitas lemah (`alur_persetujuan_langkah`, `intervensi_persetujuan_langkah`), serta definisi atau matriks referensi berkode (`tahap_intervensi_def`, `jarak_antar_wilayah`) | Satu baris per pasangan. Kunci gabungan mencegah duplikat tanpa indeks tambahan.                                                                                           |

Tabel time-series dan agregat yang dulu hanya punya kunci alami gabungan, misalnya `andil_inflasi(periode,
wilayah_kode, komoditas_kode)`, kini memakai PK BIGINT. Kunci alaminya tetap dijaga sebagai `UNIQUE` dan
dipakai sebagai target `ON CONFLICT` untuk upsert.

### Kolom `kode`: kode tampilan dan ID lama

Tabel yang PK-nya diubah dari `VARCHAR` menyimpan ID lamanya di kolom `kode varchar(64)` beserta indeks `UNIQUE`.
Kolom ini punya tiga fungsi:

1. **Kode tampilan** untuk manusia, misalnya `INT-1000`, `PB-2026-10`, `EWS-0901`, `CS-01`, `SRC-BAPANAS`, `TGS-…`.
   Baris baru tetap diberi kode bila modulnya memang menampilkan kode.
2. **Alias tautan lama**: rute entitas yang punya kode menerima ID maupun kode (`idx.ParamUUIDAtauKode` untuk
   UUID, `idx.ParamIDAtauKode` untuk BIGINT), jadi URL lama seperti `/policy-brief/PB-2026-10` atau `/ews/EWS-0901`
   tetap berlaku. Kode tidak boleh berupa angka murni, karena angka selalu dibaca sebagai ID BIGINT.
3. **Kunci idempoten seeder** (`ON CONFLICT (kode)`) dan pemulihan ID lama saat migrasi di-rollback.

Baris yang dibuat aplikasi tanpa makna kode boleh berisi `kode NULL`, karena indeks unik mengizinkan banyak NULL.

## 2. Aturan kunci asing

- **Tipe FK sama persis dengan tipe kolom induknya.** FK ke tabel UUID bertipe `uuid` dan FK ke tabel BIGINT bertipe `bigint`. Teks bebas tidak boleh menunjuk surrogate key.
- **Setiap FK punya aturan `ON DELETE` eksplisit.** `NO ACTION` tidak dipakai.

  | Hubungan                                                                                                | Aturan     | Contoh                                                       |
  | ------------------------------------------------------------------------------------------------------- | ---------- | ------------------------------------------------------------ |
  | Anak milik agregat (riwayat, langkah, lampiran, stok milik fasilitas, sesi milik pengguna)              | `CASCADE`  | `intervensi_riwayat.intervensi_id`, `refresh_tokens.user_id` |
  | Jejak pelaku opsional (kolom nullable "oleh siapa") yang tidak boleh menghalangi penghapusan akun       | `SET NULL` | `audit_log.aktor_id`, `ews_riwayat.oleh`                     |
  | Pelaku atau pemilik yang menjadi bukti asal data (penginput, validator, penerbit, penyusun)             | `RESTRICT` | `entri_harga.petugas_id`, `entri_harga.divalidasi_oleh`      |
  | Rujukan ke master atau referensi (`kabupaten`, `komoditas`, `pasar`, `sumber_data`) dan antar-transaksi | `RESTRICT` | `entri_harga.komoditas_kode`, `transaksi_stok.lot_id`        |

  Akun yang masih dirujuk sebagai bukti asal data tidak bisa dihapus. API menjawab `409` dan menyarankan
  menonaktifkan akun.

- **Setiap FK punya indeks** yang kolom awalnya sama dengan kolom FK. Indeks ini mencegah table scan saat join
  dan saat baris induk dihapus atau diubah. Indeks yang sudah diawali kolom FK, misalnya indeks gabungan
  `(kabupaten_kode, tanggal)`, sudah mencukupi.

## 3. Strategi indeks

- **Penamaan**: `idx_<tabel>_<kolom…>` untuk indeks biasa dan `uq_<tabel>_<kolom…>` untuk unik. Indeks parsial diberi akhiran keadaan, misalnya `idx_entri_harga_menunggu`.
- **Indeks FK**: wajib, lihat bagian 2.
- **Gabungan filter + urutan**: mengikuti pola kueri nyata di repositori. Kolom kesetaraan diletakkan dulu, lalu kolom urutan dengan arah yang sama dengan `ORDER BY`. Contoh: `(komoditas_kode, pasar_id, dicatat_pada DESC)`, `(status, dibuat_pada DESC)`.
- **Parsial untuk antrean**: baris yang menunggu tindakan, misalnya entri `DIAJUKAN`, tugas `DITUGASKAN`, alert `AKTIF`, persetujuan `MENUNGGU`, perubahan master `MENUNGGU`. Indeksnya kecil dan selalu panas.
- **BRIN untuk time-series append-only bervolume besar**: `entri_harga(dicatat_pada)`, `telemetri_suhu(waktu)`, `audit_log(waktu)`, `data_mentah(diambil_pada)`, `harga_eksternal(tanggal)`. Ukurannya sangat kecil dan efektif karena baris masuk kira-kira urut waktu. B-tree untuk kueri titik dan urutan halaman tetap dipertahankan.

## 4. Kode Go

| Kategori     | Tipe Go                        | Tag GORM                                                          |
| ------------ | ------------------------------ | ----------------------------------------------------------------- |
| UUID         | `string` (kanonik huruf kecil) | `gorm:"column:id;type:uuid;primaryKey;default:gen_random_uuid()"` |
| BIGINT       | `idx.ID`                       | `gorm:"column:id;primaryKey;autoIncrement"`                       |
| FK ke UUID   | `string` / `*string`           | `gorm:"column:x_id;type:uuid"`                                    |
| FK ke BIGINT | `idx.ID` / `*idx.ID`           | `gorm:"column:x_id"`                                              |

- **Kolom UUID nullable** dipetakan ke `*string`, dan "tidak ada pelaku" ditulis sebagai `NULL`, bukan string kosong. PostgreSQL menolak `''` untuk kolom `uuid`.
- **Repositori** membaca, mengubah, dan menghapus berdasarkan PK (`FindByID`, `Update`, `Delete`). Daftar yang menampilkan relasi memakai `JOIN` atau `Preload` yang terindeks, bukan kueri per baris (N+1).
- **Handler** membaca `:id` lewat `idx.ParamID`, `idx.ParamUUID`, `idx.ParamIDAtauKode`, atau `idx.ParamUUIDAtauKode`. Repositori mencari baris rujukan alias dengan `db.Where(rujukan.Kondisi("id", "kode"))`. ID yang salah bentuk dijawab `404`, sama dengan ID yang tidak ada, sehingga teks bebas tidak pernah sampai ke kolom `uuid`. Pelanggaran FK saat menghapus (`23503`) dijawab `409`.
- **Seed `GENERATED ALWAYS`**: kolom identity tidak boleh diisi. Tes dan seeder tidak menulis `id` BIGINT sendiri, kecuali memakai `OVERRIDING SYSTEM VALUE` di fixture SQL yang memang membutuhkannya.

## 5. API, BFF, dan web

- **Semua ID di JSON adalah string**: UUID kanonik, atau angka desimal untuk BIGINT (`"42"`, mengikuti pemetaan JSON proto3 untuk int64). Klien memperlakukan ID sebagai teks tak bermakna, sehingga tipe `id: string` di web tidak berubah dan angka di atas 2^53 tetap utuh. Saat membaca, API juga menerima angka JSON.
- **Rute** memakai bentuk `/v1/<sumber>/:id`. Entitas UUID yang punya kode tampilan juga menerima kode di posisi yang sama sebagai alias.
- **Kode tampilan** dikirim di field `kode`. UI menampilkan `kode`, bukan `id`.
- **Pemetaan BFF** meneruskan ID apa adanya. BFF tidak mengurai atau mengubah format ID.

## 6. Seeder

- **Idempoten** terhadap kunci alami yang unik, seperti `email` untuk pengguna, `kode` untuk master dan entitas berkode, atau kunci alami tabel agregat. Seeder memakai `ON CONFLICT (<kunci alami>) DO UPDATE` atau `DO NOTHING`, tidak pernah `ON CONFLICT (id)` untuk identity.
- **UUID baris seed** memakai `idx.UUIDStabil("<tabel>", "<kode lama>")`, misalnya `idx.UUIDStabil("pengguna", "superadmin")`. Basis data baru dan basis data yang dimigrasi memakai UUID yang sama, sehingga rujukan antar-seed langsung memakai nilainya.
- **BIGINT baris seed** diberi basis data. Seeder anak mencari ID induknya lewat `kode`, misalnya `(SELECT id FROM entri_harga WHERE kode = ?)`.
- **Menjalankan seeder dua kali** (`make db/seed/reference`, `make db/seed/demo`) tidak menambah baris.

## 7. Migrasi data dari ID lama

1. **VARCHAR → UUID**:
   - `id_baru = garda_uuid_stabil('<tabel>', id_lama)`.
   - Semua kolom FK anak diperbarui lewat pemetaan yang sama di dalam satu transaksi.
   - ID lama disimpan di `kode` untuk entitas berkode.
   - Untuk `pengguna`, ID lama disimpan di tabel pemetaan `pengguna_id_lama` supaya Down bisa memulihkannya.
   - Rujukan lunak (kolom teks tanpa FK) yang berisi ID lama ikut dipetakan bila ia identitas yang dicari aplikasi, mis.
     `audit_log.entitas_id` pada baris `entitas = 'pengguna'` (riwayat peran dan filter audit). Snapshot JSON historis
     (`audit_log.sebelum/sesudah/metadata`) dibiarkan sebagai sejarah.
   - Nilai turunan yang tidak bisa dihitung ulang (hash kode cadangan MFA dari `"<id>:<kode>"`) tidak diubah: aplikasi
     mencocokkannya dengan ID lama dari `pengguna_id_lama`.
2. **VARCHAR → BIGINT**:
   - ID baru diberikan berurutan menurut waktu pembuatan, lalu ID lama sebagai pemutus seri.
   - Kolom dijadikan `GENERATED ALWAYS AS IDENTITY` dan urutannya diset ke `max(id) + 1`.
   - ID lama disimpan di `kode` (UNIQUE), dan FK anak diperbarui lewat kode itu.
3. **Kunci alami gabungan → BIGINT + UNIQUE**: kolom `id` identity ditambahkan sebagai PK, dan kunci gabungan lama menjadi constraint `UNIQUE`.
4. **Down** mengembalikan tipe dan nilai lama, memakai `kode` atau tabel pemetaan. `TestMigrationsApplyUpDownUp` memastikan siklus naik-turun-naik berjalan.
5. **Tes migrasi data**: setiap migrasi konversi punya tes integrasi. Tes itu menjalankan migrasi sampai versi sebelumnya, mengisi baris berbentuk data lama, menjalankan migrasi, lalu memeriksa ID baru, kode lama, FK anak, dan rollback-nya.

## Matriks tabel

Matriks dikelompokkan per migrasi konversi (`000110`–`000115`). Daftar lengkap FK dan indeks selalu bisa
diturunkan dari katalog (`\d+ <tabel>` atau kueri di `kunci_test.go`).

### 000110: pengguna dan keamanan akun

| Tabel                                       | PK  | Alasan | FK utama | Indeks utama |
| ------------------------------------------- | --- | ------ | -------- | ------------ |
| `pengguna` | UUID | Identitas akun yang terlihat klien (path `/v1/users/:id`, token `sub`). ID lama berupa slug (`superadmin`, `mahasiswa_9`, `USR-…`) dipetakan stabil: `garda_uuid_stabil('pengguna', id_lama)` = `seedkit.IDPengguna(slug)`. | `kabupaten_kode` → `kabupaten` RESTRICT | `pengguna_pkey`, `uq email`, `idx_pengguna_kabupaten_kode`, `idx_pengguna_peran`, `ux_pengguna_sso (sso_provider, sso_subject)` |
| `pengguna_id_lama` | KODE | 1:1 dengan `pengguna`: pemetaan ID lama untuk Down 000110. Aplikasi memakainya hanya agar kode cadangan MFA yang di-hash dengan ID lama tetap berlaku. | `id` → `pengguna` CASCADE | `pengguna_id_lama_pkey`, `UNIQUE (id_lama)` |
| `user_permissions` | KOMPOSIT `(pengguna_id, permission_nama)` | Satu izin langsung per pasangan pengguna dan izin. | `pengguna_id` → `pengguna` CASCADE; `permission_nama` → `permissions` CASCADE; `diberikan_oleh` → `pengguna` SET NULL | PK (awalan `pengguna_id`), `idx_user_permissions_permission`, `idx_user_permissions_diberikan_oleh`, `idx_user_permissions_berlaku` (parsial) |
| `undangan_pengguna` | BIGINT | Operasional: satu baris per undangan, token hanya hash. | `user_id` → `pengguna` CASCADE; `diundang_oleh` → `pengguna` SET NULL | `idx_undangan_pengguna_user`, `uq_undangan_pengguna_aktif` (parsial), `UNIQUE (token_hash)`, `idx_undangan_pengguna_diundang_oleh` |
| `roles` | KODE (`kunci` enum `peran`) | Katalog 14 peran; identitasnya enum itu sendiri. | `diperbarui_oleh` → `pengguna` SET NULL | PK, `idx_roles_diperbarui_oleh` |
| `refresh_tokens` | BIGINT | Sesi milik pengguna; baris ikut hilang bersama akun. | `user_id` → `pengguna` CASCADE | `idx_rt_user`, `idx_rt_user_revoked (user_id, revoked_at)`, `UNIQUE (token_hash)`, `idx_rt_family` |
| `login_attempts` | BIGINT | Log percobaan masuk; `user_id` NULL untuk email tak dikenal, dan baris bertahan walau akunnya dihapus. | `user_id` → `pengguna` SET NULL | `idx_la_user_time (user_id, attempted_at)` |
| `mfa_kode_cadangan` | BIGINT | Satu baris per kode cadangan (hash), milik akun. | `user_id` → `pengguna` CASCADE | `UNIQUE (user_id, kode_hash)` (awalan `user_id`) |
| `password_reset_tokens` | BIGINT | Token sekali pakai milik akun. | `user_id` → `pengguna` CASCADE | `idx_prt_user`, `UNIQUE (token_hash)` |
| `kunci_login` | HASH `email_hash` | Lockout per email: kuncinya SHA-256 email, bukan ID pengguna. | tanpa FK | PK, `idx_kunci_login_diperbarui` |
| `jwt_blacklist` | HASH `jti` | Daftar cabut token: kuncinya `jti` token. | tanpa FK | PK, `idx_jwt_blacklist_expires` |
| `sso_login_state` | HASH `state_hash` | Keadaan satu alur SSO (≤ 10 menit), kuncinya hash `state`. | tanpa FK | PK, `idx_sso_login_state_expires` |
| `preferensi_notifikasi` | KOMPOSIT `(pengguna_id, kanal)` | Satu baris per pengguna dan kanal. | `pengguna_id` → `pengguna` CASCADE | PK (awalan `pengguna_id`) |
| `langganan_push` | BIGINT | Satu langganan push per endpoint, milik akun. | `pengguna_id` → `pengguna` CASCADE | `idx_langganan_push_pengguna`, `UNIQUE (endpoint)` |

### 000111: harga, lapangan, dan master wilayah

| Tabel | PK | Alasan | FK utama | Indeks utama |
| ----- | -- | ------ | -------- | ------------ |
| _diisi oleh migrasi 000111_ | | | | |

### 000112: EWS dan intervensi

| Tabel | PK | Alasan | FK utama | Indeks utama |
| ----- | -- | ------ | -------- | ------------ |
| _diisi oleh migrasi 000112_ | | | | |

### 000113: cold storage dan spasial

| Tabel | PK | Alasan | FK utama | Indeks utama |
| ----- | -- | ------ | -------- | ------------ |
| _diisi oleh migrasi 000113_ | | | | |

### 000114: prediksi dan policy brief

| Tabel | PK | Alasan | FK utama | Indeks utama |
| ----- | -- | ------ | -------- | ------------ |
| `policy_brief` | UUID | Entitas bisnis yang terlihat klien (path `/v1/policy-brief/:id`). ID lama `PB-YYYY-MM` dipetakan stabil: `garda_uuid_stabil('policy_brief', id_lama)` = `idx.UUIDStabil("policy_brief", kode)`. `kode` = `PB-YYYY-MM` (NOT NULL, `UNIQUE`, `CHECK` berbentuk `PB-YYYY-MM`) menjadi kode tampilan, alias URL, dan kunci idempotensi `Generate` serta seeder: satu brief per periode. | `disusun_oleh_id` → `pengguna` RESTRICT (000110) | `policy_brief_pkey`, `uq_policy_brief_kode`, `idx_policy_brief_disusun_oleh_id`. Daftar diurutkan menurut `kode DESC`. |
| `policy_brief_versi` | BIGINT `id` (versi_id) + `UNIQUE (brief_id, versi)` | Entitas lemah (PK komposit lama `(brief_id, versi)`) yang kini dirujuk telaah dan revisi lewat `versi_id`. Nomor `versi` tetap urut per brief dan dipakai target keunikan. Urutan `id` mengikuti `waktu`. | `brief_id` → `policy_brief` CASCADE; `oleh` → `pengguna` RESTRICT (000110) | `policy_brief_versi_pkey`, `uq_policy_brief_versi_alami (brief_id, versi)` (melayani FK `brief_id`), `idx_policy_brief_versi_oleh` |
| `brief_job` | BIGINT | Operasional: satu baris per percobaan penyusunan. Urutan menurut `mulai`; `kode` = ID lama (UUID), kosong untuk job baru. | `brief_id` → `policy_brief` CASCADE (sebelumnya NO ACTION) | `idx_brief_job_brief (brief_id, mulai DESC)`, `idx_brief_job_berjalan (mulai) WHERE status = 'BERJALAN'` (parsial), `uq_brief_job_kode` |
| `model_registry` | BIGINT | Registri model: transaksional/operasional, dirujuk proyeksi dan riwayat promosi. Urutan menurut `dilatih_pada`; `kode` = ID lama `MDL-<komoditas>-<versi>`, tetap menjadi kode tampilan dan alias rujukan (baris baru hasil pelatihan juga diberi kode itu). `UNIQUE (komoditas_kode, versi)` dan `uq_model_registry_produksi` (satu PRODUKSI per komoditas) tetap. | `komoditas_kode` → `komoditas` RESTRICT; `dipromosikan_oleh` → `pengguna` RESTRICT (000110) | `model_registry_pkey`, `uq_model_registry_kode`, `model_registry_komoditas_kode_versi_key` (melayani FK komoditas), `uq_model_registry_produksi` (parsial), `idx_model_registry_dipromosikan_oleh` |
| `prediksi_harga` | BIGINT | Data prediksi: banyak baris per pelatihan, dibaca per komoditas/kabupaten. Urutan menurut `dibuat_pada`; `kode` = ID lama `PRD-…`/`PRM-…` (kunci idempoten seeder demo), kosong untuk proyeksi hasil pipeline ML yang di-upsert menurut kunci alami `uq_prediksi_harga_proyeksi`. `model_id` menggantikan FK komposit varchar; `model_versi` tetap sebagai informasi yang ditulis konsisten dengan `model_id`. | `model_id` → `model_registry` RESTRICT; `komoditas_kode` → `komoditas` RESTRICT; `kabupaten_kode` → `kabupaten` RESTRICT | `prediksi_harga_pkey`, `uq_prediksi_harga_kode`, `uq_prediksi_harga_proyeksi (komoditas_kode, kabupaten_kode, tanggal_target, horizon_hari, model_versi)`, `idx_prediksi_harga_model_id`, `idx_prediksi_harga_terbaru (komoditas_kode, kabupaten_kode, horizon_hari, tanggal_target DESC)`, `idx_prediksi_harga_kabupaten_horizon (kabupaten_kode, horizon_hari, komoditas_kode, tanggal_target DESC)` |
| `prediksi_evaluasi` | BIGINT `id` + `UNIQUE (prediksi_id)` | Evaluasi 1:1 dengan proyeksi (galat nyata setelah tanggal target). PK lama `prediksi_id` menjadi `UNIQUE` yang tetap menjadi target `ON CONFLICT (prediksi_id)`. | `prediksi_id` → `prediksi_harga` CASCADE | `prediksi_evaluasi_pkey`, `uq_prediksi_evaluasi_prediksi` |
| `model_promosi` | BIGINT | Riwayat tak berubah (promosi dan pengarsipan model). Urutan menurut `waktu`; `kode` = ID lama (UUID), kosong untuk riwayat baru. | `model_id` → `model_registry` RESTRICT; `model_digantikan_id` → `model_registry` RESTRICT; `komoditas_kode` → `komoditas` RESTRICT; `oleh` → `pengguna` RESTRICT (000110) | `idx_model_promosi_model (model_id, waktu DESC)`, `idx_model_promosi_komoditas (komoditas_kode, waktu DESC)`, `idx_model_promosi_model_digantikan_id`, `idx_model_promosi_oleh`, `uq_model_promosi_kode` |
| `skenario_simulasi` | BIGINT | Hasil simulasi what-if per pengguna. Urutan menurut `dibuat_pada`; `kode` = ID lama (UUID, alias rujukan), kosong untuk skenario baru. | `komoditas_kode` → `komoditas` RESTRICT; `kabupaten_kode` → `kabupaten` RESTRICT; `dibuat_oleh` → `pengguna` RESTRICT (000110) | `idx_skenario_simulasi_pembuat (dibuat_oleh, dibuat_pada DESC)`, `idx_skenario_simulasi_komoditas_kode`, `idx_skenario_simulasi_kabupaten_kode`, `uq_skenario_simulasi_kode` |

### 000115: sumber data, ingestion, eksekutif, siklus, KPI, dashboard, dan audit

| Tabel | PK | Alasan | FK utama | Indeks utama |
| ----- | -- | ------ | -------- | ------------ |
| _diisi oleh migrasi 000115_ | | | | |
