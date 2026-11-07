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
| `entri_harga` | BIGINT | Log transaksional time-series (±32 ribu baris demo, tumbuh tiap hari). ID baru menaik menurut `dicatat_pada`, lalu `dibuat_pada`, lalu ID lama. ID lama (`HRG-D00-00-01-K`, atau UUID dari aplikasi lama) pindah ke `kode UNIQUE`: alias tautan `/v1/harga/:id`. Baris baru: `kode` NULL. Idempotensi sinkron offline tetap `UNIQUE (petugas_id, idempotency_key)`. | `pasar_id` → `pasar` RESTRICT; `komoditas_kode` → `komoditas` RESTRICT; `petugas_id`, `divalidasi_oleh`, `dipublikasi_oleh`, `ditolak_oleh` → `pengguna` RESTRICT (000110) | PK, `uq_entri_harga_kode`, `uq_entri_harga_petugas_kunci`, BRIN `idx_entri_harga_dicatat_brin (dicatat_pada)`, `idx_entri_harga_dicatat (dicatat_pada DESC, id)`, `idx_entri_harga_komoditas_pasar`, `idx_entri_harga_status (status, dicatat_pada)`, `idx_entri_harga_bentrok`, `idx_entri_harga_petugas_dicatat (petugas_id, dicatat_pada DESC)`, parsial antrean validasi `idx_entri_harga_antrean` dan `idx_entri_harga_antrean_skor (skor_anomali DESC, dicatat_pada DESC, id) WHERE status = 'DIAJUKAN'` |
| `riwayat_status_entri` | BIGINT (BIGSERIAL) | Log perpindahan status entri; PK tidak berubah. Hanya `entri_id` menjadi bigint. | `entri_id` → `entri_harga` CASCADE; `oleh` → `pengguna` RESTRICT (000110) | `idx_riwayat_status_entri (entri_id, waktu, id)`, `idx_riwayat_status_entri_oleh` |
| `tugas_lapangan` | BIGINT | Tugas survei operasional per petugas. ID baru menurut `dibuat_pada`; ID lama `TGS-<slug>-<n>` pindah ke `kode UNIQUE` (alias `/v1/lapangan/tugas/:id` dan antrean PWA lama). Tugas baru: `kode` NULL. | `pasar_id` → `pasar` RESTRICT; `petugas_id`, `dibuat_oleh_id` → `pengguna` RESTRICT (000110) | PK, `uq_tugas_lapangan_kode`, `idx_tugas_lapangan_petugas (petugas_id, status)`, `idx_tugas_lapangan_pasar_tenggat`, parsial tugas aktif `idx_tugas_lapangan_aktif (petugas_id, tenggat) WHERE status <> 'SELESAI'` |
| `tugas_entri` | KOMPOSIT `(tugas_id, entri_id)` | Tabel penghubung tugas dan entri; satu baris per pasangan. Kedua kolom menjadi bigint, PK tidak berubah. | `tugas_id` → `tugas_lapangan` CASCADE; `entri_id` → `entri_harga` CASCADE | PK (awalan `tugas_id`), `idx_tugas_entri_entri` |
| `batas_harga` | BIGINT | Rentang harga wajar per komoditas (dan kabupaten bila diisi), berlaku dari tanggal tertentu; operasional, diubah admin. Tidak punya kolom waktu pembuatan, jadi ID baru menurut `berlaku_dari`, komoditas, kabupaten, lalu ID lama. ID lama `BH-<komoditas>` pindah ke `kode UNIQUE`. | `komoditas_kode` → `komoditas` RESTRICT; `kabupaten_kode` → `kabupaten` RESTRICT | PK, `uq_batas_harga_kode`, kunci alami `uq_batas_harga_berlaku (komoditas_kode, COALESCE(kabupaten_kode, ''), berlaku_dari)`, `idx_batas_harga_kabupaten_kode` |
| `perubahan_master` | BIGINT | Antrean perubahan master data yang menunggu persetujuan (belum dipakai kode Go, direncanakan di todo 04). ID baru menurut `dibuat_pada`; ID lama pindah ke `kode UNIQUE`. `entitas_id` polimorfik (kode komoditas atau lainnya), tanpa FK. | `diajukan_oleh_id`, `disetujui_oleh_id` → `pengguna` RESTRICT (000110) | PK, `uq_perubahan_master_kode`, `idx_perubahan_master_entitas (entitas, entitas_id)`, parsial antrean persetujuan `idx_perubahan_master_menunggu (dibuat_pada) WHERE status = 'MENUNGGU'` |
| `pasar` | KODE (`id` varchar, mis. `PSR-1171-01`) | Referensi master berkode; kode itu dipakai langsung sebagai nilai FK (entri, tugas) dan oleh materialized view `agregat_harga_harian`. Tipe tidak berubah, hanya aturan FK. | `kabupaten_kode` → `kabupaten` RESTRICT | PK, `idx_pasar_kabupaten` |
| `kabupaten` | KODE (`kode` varchar(4), kode BPS) | Referensi nasional; dirujuk puluhan tabel. Hanya aturan FK ke `provinsi` yang diperjelas. | `provinsi_kode` → `provinsi` RESTRICT | PK, `idx_kabupaten_provinsi` |

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
| _diisi oleh migrasi 000114_ | | | | |

### 000115: sumber data, ingestion, eksekutif, siklus, KPI, dashboard, dan audit

| Tabel | PK | Alasan | FK utama | Indeks utama |
| ----- | -- | ------ | -------- | ------------ |
| `sumber_data` | UUID | Identitas sumber yang terlihat klien (path `/v1/sumber-data/:id`). ID lama (`SRC-BI`, `SRC-BPS`, …) dipetakan stabil: `garda_uuid_stabil('sumber_data', id_lama)` = `idx.UUIDStabil("sumber_data", kode)`, dan disimpan di `kode` (UNIQUE, NOT NULL): kode tampilan, alias rute, dan kunci konfigurasi konektor/CLI ingestion (konektor dikenali lewat kode, riwayat memakai UUID). | tanpa FK keluar | `sumber_data_pkey`, `uq_sumber_data_kode` |
| `eksekusi_sumber` | BIGINT (BIGSERIAL lama) | Satu baris per run ingestion; operasional bervolume tumbuh. | `sumber_id` → `sumber_data` CASCADE (riwayat milik sumber); `aktor_id` → `pengguna` SET NULL (000110) | `idx_eksekusi_sumber_riwayat (sumber_id, mulai DESC)`, `uq_eksekusi_sumber_berjalan` (parsial, satu run BERJALAN per sumber), `idx_eksekusi_sumber_aktor_id` |
| `data_mentah` | BIGINT (BIGSERIAL lama) | Bronze: payload JSONB asli per respons. | `sumber_id` → `sumber_data` CASCADE; `eksekusi_id` → `eksekusi_sumber` CASCADE | `uq_data_mentah_checksum (sumber_id, checksum)`, `idx_data_mentah_eksekusi`, `idx_data_mentah_diambil (sumber_id, diambil_pada DESC)`, `idx_data_mentah_diambil_brin` (BRIN) |
| `pemetaan_sumber` | KOMPOSIT `(sumber_id, jenis, alias)` | Konfigurasi milik sumber: satu baris per alias; kunci gabungan mencegah duplikat. | `sumber_id` → `sumber_data` CASCADE | PK (awalan `sumber_id`) |
| `harga_eksternal` | BIGINT + `UNIQUE (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe)` | Silver: time-series harga hasil ingestion. Kunci alami komposit dulu PK; kini UNIQUE sebagai target `ON CONFLICT` upsert. | `sumber_id` → `sumber_data` RESTRICT (bukti asal data: sumber yang punya harga dinonaktifkan, bukan dihapus); `komoditas_kode` → `komoditas` RESTRICT; `kabupaten_kode` → `kabupaten` RESTRICT; `data_mentah_id` dan `eksekusi_id` SET NULL (lineage boleh hilang bersama retensi bronze) | `uq_harga_eksternal_alami`, `idx_harga_eksternal_lookup (komoditas_kode, kabupaten_kode, tanggal)`, `idx_harga_eksternal_kabupaten_kode`, `idx_harga_eksternal_data_mentah_id`, `idx_harga_eksternal_eksekusi_id`, `idx_harga_eksternal_tanggal_brin` (BRIN) |
| `cuaca_harian` | BIGINT (dulu VARCHAR `CUA-<kab>-<yyyymmdd>-<sumber>`) | Silver cuaca. ID BIGINT berurut menurut `tanggal`, lalu ID lama; `kode` = ID lama (NULL untuk baris baru). Pembuatan ID di aplikasi dihapus. | `kabupaten_kode` → `kabupaten` RESTRICT; `eksekusi_id` dan `data_mentah_id` SET NULL | `UNIQUE (kabupaten_kode, tanggal, sumber)` (target upsert), `uq_cuaca_harian_kode`, `idx_cuaca_harian_tanggal`, `idx_cuaca_harian_eksekusi_id`, `idx_cuaca_harian_data_mentah_id` |
| `andil_inflasi` | BIGINT + `UNIQUE (periode, wilayah_kode, komoditas_kode)` | Agregat bulanan hasil impor; kunci alami jadi target upsert. `wilayah_kode` berisi kode provinsi atau kabupaten (CHECK panjang 2 atau 4), jadi tanpa FK. | `komoditas_kode` → `komoditas` RESTRICT | `uq_andil_inflasi_alami`, `idx_andil_inflasi_komoditas_kode` |
| `inflasi_bulanan` | BIGINT + `UNIQUE (wilayah_kode, periode)` | Agregat bulanan hasil impor (BPS) dan baris demo. | tanpa FK (wilayah provinsi atau kabupaten) | `uq_inflasi_bulanan_alami` |
| `neraca_pangan` | BIGINT + `UNIQUE (komoditas_kode, wilayah_kode, periode)` | Agregat bulanan neraca pangan. | `komoditas_kode` → `komoditas` RESTRICT | `uq_neraca_pangan_alami`, `idx_neraca_pangan_periode (wilayah_kode, periode DESC)` |
| `ruang_fiskal` | BIGINT (dulu VARCHAR `DEMO-RF-…`) | Agregat impor DJPb. ID BIGINT berurut menurut `diimpor_pada`, lalu ID lama; `kode` = ID lama. | tanpa FK | `UNIQUE (tahun, instrumen, per_tanggal)` (target upsert), `uq_ruang_fiskal_kode` |
| `siklus_ekonomi` | BIGINT (dulu VARCHAR `SE-11-DEMO`) | Riwayat siklus per wilayah. ID BIGINT berurut menurut `dibuat_pada`, lalu ID lama; `kode` = ID lama (kunci idempotensi seeder demo). Tidak ada rute `:id`. | `wilayah_kode` → `provinsi` RESTRICT | `uq_siklus_ekonomi_aktif` (parsial, satu siklus aktif per wilayah), `idx_siklus_ekonomi_wilayah`, `uq_siklus_ekonomi_kode` |
| `target_kpi` | BIGINT + `UNIQUE (indikator, periode_mulai)` | Target indikator dashboard operasional; kunci alami jadi target upsert seeder. | tanpa FK | `uq_target_kpi_alami` |
| `kpi_nilai` | BIGINT (BIGSERIAL lama) | Nilai KPI per periode dan versi (revisi = baris baru). | `kpi_kode` → `kpi_definisi` CASCADE; `wilayah_kode` → `kabupaten` RESTRICT (NULL = seluruh provinsi) | `uq_kpi_nilai (kpi_kode, COALESCE(wilayah_kode, ''), periode, versi)`, `idx_kpi_nilai_kpi_periode`, `idx_kpi_nilai_wilayah_kode` |
| `kpi_target` | BIGINT (BIGSERIAL lama) | Target KPI per periode mulai. | `kpi_kode` → `kpi_definisi` CASCADE; `wilayah_kode` → `kabupaten` RESTRICT (NULL = semua wilayah) | `uq_kpi_target`, `idx_kpi_target_wilayah_kode` |
| `audit_log` | BIGINT identity (dulu VARCHAR UUID teks dari aplikasi) | Log append-only bervolume besar. ID BIGINT berurut menurut `waktu`, lalu ID lama; `kode` = ID lama (alias `GET /v1/audit/:id` dan kunci idempotensi seeder demo; NULL untuk baris baru). Pemicu append-only dimatikan hanya selama konversi. Rujukan lunak `entitas_id` (teks, bukan FK) dipetakan ke ID baru untuk entitas `sumber_data`, `langganan_push`, dan `preferensi_notifikasi`. | `aktor_id` → `pengguna` SET NULL (000110) | `idx_audit_log_waktu_brin` (BRIN), `idx_audit_log_waktu_id (waktu DESC, id DESC)` (cursor), `idx_audit_log_entitas_waktu (entitas, entitas_id, waktu DESC)`, `idx_audit_log_aktor_waktu`, `idx_audit_log_aksi_waktu`, `uq_audit_log_kode` |
