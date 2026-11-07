# 19 — Migration & seeder (fondasi data)

**Prioritas:** P0 — memblokir hampir semua fitur lain. Kerjakan sebelum menyambungkan halaman web ke backend.

Berkas ini berisi temuan audit skema, keputusan, konvensi, strategi seeder, dan rincian **tugas fondasi (F0 dan F1)**. **Urutan kerja dan aturan per task ada di [00-cara-kerja.md](00-cara-kerja.md).** Migration dan seeder spesifik per fitur sekarang menjadi langkah `MIG` dan `SEED` di dalam slice pada berkas 01–16 (tidak lagi dikerjakan terpusat di depan).

> **Cara temuan ini didapat:** analisis statis atas `db/migrations/*.sql`, seluruh `entity.go`, SQL mentah di repository, `cmd/seed*`, Makefile, Dockerfile, dan skrip deploy, ditambah `go test`. Perbandingan kolom entity vs migrasi dilakukan dengan skrip (bukan dibaca manual). **API belum dijalankan terhadap database hasil migrasi.** Langkah M0 di bawah memastikannya secara otomatis.

## 1. Temuan audit

### 1.1 Entity Go tidak cocok dengan migrasi (kritis)

Hanya `auth`, `audit`, `cold-storage`, dan `master` yang kolomnya cocok dengan migrasi (dua di antaranya, `auth` dan `cold-storage`, punya integration test). Modul lain ditulis terhadap skema yang berbeda, sehingga pada database hasil migrasi query-nya akan gagal.

**Tabel yang dipakai Go tetapi tidak ada di migrasi:**

| Modul      | Entity → tabel yang dicari                                                                                                   | Tabel di migrasi yang paling mendekati |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- |
| intervensi | `KartuIntervensi` → `kartu_intervensi`                                                                                       | `intervensi` (+ `intervensi_pj`)       |
| prediksi   | `ProyeksiXai` → `proyeksi_xai`                                                                                               | `prediksi_harga`                       |
| siklus     | `SiklusEkonomi` → `siklus_ekonomi` (tipe `siklus_ekonomi`)                                                                   | tidak ada tabel; enum `siklus` ada     |
| spasial    | `Wilayah` → `wilayah`                                                                                                        | `kabupaten` (punya lat/lng)            |
| dashboard  | SQL mentah: `kpi_operasional`, `heatmap_sel`, `ringkasan_eksekutif`, `komoditas_sumbangan`, `daerah_inflasi`, `tren_inflasi` | tidak ada                              |

**Kolom/tipe yang tidak cocok pada tabel yang ada:**

| Modul        | Tabel            | Kolom di entity yang tidak ada di migrasi                                                                                                           |
| ------------ | ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| ews          | `ews_alert`      | `level` (tipe `level_ews` juga tidak ada), `skor`, `deskripsi`, `wilayah_kode`, `direkomendasikan_pada`, `ditindaklanjuti_pada`                     |
| harga        | `entri_harga`    | `dibuat_pada`, `diupdate_pada`, `periode`                                                                                                           |
| lapangan     | `tugas_lapangan` | `dibuat_pada`, `lokasi`, `pic`                                                                                                                      |
| policy-brief | `policy_brief`   | `dibuat_pada`, `diupdate_pada` (migrasi: `diperbarui`); `disusun_oleh_id` NOT NULL tidak diisi; default status `DRAFT` padahal enum bernilai `DRAF` |
| sumber-data  | `sumber_data`    | `dibuat_pada`, `frekuensi_update`, `tipe`, `url`                                                                                                    |

Selain itu, `id` di entity `ews`, `intervensi`, `policy-brief`, `lapangan`, dan `sumber-data` bertipe `int64` auto-increment, sedangkan di migrasi bertipe `VARCHAR` (mis. `EWS-…`, `PB-2026-10`). Data seed dan mock web memakai id string.

**Akibat:** begitu `NEXT_PUBLIC_API_BASE_URL` diisi, halaman dashboard, EWS, intervensi, prediksi, policy brief, sumber data, tugas lapangan, peta, dan siklus kemungkinan besar error. Ini menjelaskan mengapa modul-modul itu hanya punya unit test dengan repository tiruan.

### 1.2 Tabel tanpa seed, jadi kosong walau skema benar

Setelah migrasi + `cmd/seed`, tabel berikut **kosong**: `entri_harga`, `cuaca_harian`, `sumber_data`, `aturan_ews`, `ews_alert`, `model_registry`, `prediksi_harga`, `ruang_fiskal`, `policy_brief`, `intervensi`, `intervensi_pj`, `tugas_lapangan`, `perubahan_master`, `audit_log`.

Yang ada datanya: `provinsi`, `kabupaten`, `pasar`, `komoditas`, `pengguna`, `role_permissions` (dari migrasi 000006), serta cold storage (dari `cmd/seed`). [ARCHITECTURE.md](../ARCHITECTURE.md) menyebut mock web "identik dengan seeder", tetapi seeder Postgres untuk tabel-tabel di atas tidak ada.

### 1.3 Tabel tanpa pemakai

Ada di migrasi tetapi tidak dirujuk kode Go: `aturan_ews`, `cuaca_harian`, `intervensi_pj`, `perubahan_master`, `prediksi_harga`, `ruang_fiskal`, `password_history`, dan empat tabel RBAC. Beberapa di antaranya sebenarnya adalah jawaban untuk fitur yang masih berstatus todo (versioning master data, ambang EWS, ruang fiskal), jadi tinggal dihubungkan.

### 1.4 Tabel RBAC di database tidak dipakai

`roles` (15 baris), `permissions` (29 baris berformat `harga:read`), `role_permissions`, dan `user_permissions` diisi oleh migrasi 000001 dan 000006, tetapi **tidak dibaca kode apa pun**. Izin yang berlaku berasal dari peta `rolePermissions` di [auth/usecase/usecase.go](../../apps/api/internal/auth/usecase/usecase.go) yang dikunci pada enum `peran` (14 nilai) dan memakai 35 izin berformat `domain.aksi`. Nama izin di DB tidak ada satupun yang sama dengan nama yang dipakai kode.

Akibat lain yang terverifikasi dari kode: route yang meminta `lapangan.lihat`, `lapangan.baca`, `lapangan.validasi`, dan `prediksi.baca` **tidak diberikan ke peran manapun** di peta itu. Semua route `/v1/lapangan/*` dan sebagian route `analytics` (prediksi, validasi, aktivitas lapangan, registri model) selalu menjawab 403.

### 1.5 Migrasi 000006 bercampur antara seed referensi dan seed demo

- Menanam **25 akun demo** dengan password yang sama (`Garda2024!`, hash bcrypt tertulis di SQL), termasuk akun admin dan akun MFA dengan secret serta kode cadangan polos. Karena ini migrasi, akun-akun itu ikut terpasang di produksi.
- Berisi `DELETE FROM role_permissions;` yang menghapus semua penugasan izin setiap kali migrasi dijalankan (mis. pada database baru atau rebuild), termasuk bila admin pernah mengubahnya.
- Data referensi (provinsi, 23 kabupaten/kota, pasar, komoditas) ikut di sini, padahal ini bagian yang memang aman untuk produksi.

### 1.6 Migrasi di deploy dilewati diam-diam

[remote-deploy.sh](../../scripts/deploy/remote-deploy.sh) langkah 6 menjalankan `goose` di dalam image API, tetapi [api.Dockerfile](../../docker/api.Dockerfile) hanya menyalin binary `api` ke image alpine (tanpa goose dan tanpa direktori `/migrations`). Perintahnya `command -v goose && goose up || echo "…migrasi dilewati"` selalu jatuh ke `echo`, dan langkah tetap dilaporkan "Migrasi selesai (atau dilewati dengan aman)". Pola `A && B || C` juga menelan kegagalan `goose` yang sebenarnya. Berdasarkan kode, database produksi tidak akan punya skema kecuali migrasi dijalankan manual. (Belum diuji di server.)

### 1.7 Temuan kecil lainnya

- **Penyebab 4 tes integrasi cold-storage yang gagal:** tes memakai domain email `@garda-inflasi.id`, sedangkan seed memakai `@gardainflasi.id` (domain diubah di commit `fix: naming seeder data`). Bukan karena seed pindah ke `cmd/seed`, seperti dugaan awal di [README](README.md). `env.example`, `packages/contracts/openapi.yaml`, dan beberapa tes lain memakai domain bertanda hubung.
- Test harness memakai fungsi `splitGoose` buatan sendiri, bukan library goose. Perilaku bisa berbeda dari `goose up` sungguhan (mis. `StatementBegin`, `NO TRANSACTION`).
- `docker/postgres/init.sql` hanya membuat ekstensi, dan hanya dipasang di `compose.infra.yaml`.
- Seeder MongoDB (`cmd/seed-analytics`) hanya `InsertMany` tanpa hapus/upsert, jadi dijalankan dua kali kemungkinan menggandakan data (verifikasi). Isinya (`seedModelRegistry`, dan lainnya) tumpang tindih dengan tabel Postgres.
- Tiga struct `AuditLog` (auth, audit, cold-storage) memetakan tabel `audit_log` yang sama.
- Migrasi memakai format goose dengan Up dan Down dalam satu berkas (bagus, semuanya punya Down). Makefile punya `db/migrate`, `db/migrate/down`, `db/seed`.

## 2. Keputusan yang harus diambil dulu

| #   | Keputusan                                              | Rekomendasi                                                                                                                                                                                                                            |
| --- | ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | Sumber kebenaran skema: migrasi atau entity?           | **Migrasi.** Skemanya lebih kaya, cocok dengan tipe di web (`id` string, `kabupaten_kode`, enum), dan jadi pola yang sudah terbukti di `cold-storage`. Entity mengikuti.                                                               |
| D2  | Rebaseline atau hanya migrasi maju?                    | **Migrasi maju (tanpa rebaseline).** Cocok dengan cara kerja per task: tiap slice menambah migration baru. Satu-satunya berkas lama yang disentuh adalah 000006 (murni data) pada task `F1`, setelah isinya dipindah ke seeder. Rebaseline baru dipertimbangkan bila terbukti ada konflik skema yang tak bisa diselesaikan dengan migrasi maju. |
| D3  | RBAC: peta di kode (A) atau tabel di DB (B)?           | Lihat [02](02-pengguna-rbac.md). **B** bila izin per pengguna dan UI matriks yang bisa diubah memang dibutuhkan; **A** (hapus tabel mati) bila tidak.                                                                                  |
| D4  | Data analitik: Postgres saja, atau Postgres + MongoDB? | **Diputuskan: Postgres saja** (modul `analytics`, `cmd/seed-analytics`, `pkg/database/mongo.go`, dan layanan mongodb dihapus). Semula: satu sumber per jenis data. Rekomendasi: Postgres untuk semua data transaksional dan agregat; MongoDB hanya bila ada alasan teknis yang jelas (sekarang `analytics` tidak dipanggil web).                                              |
| D5  | Domain email akun demo                                 | Pilih satu (`@gardainflasi.id` sudah di seed) dan seragamkan test, `env.example`, OpenAPI, dokumentasi.                                                                                                                                |
| D6  | Penyimpanan time-series (harga, cuaca, suhu, audit)    | Partisi deklaratif Postgres per bulan (tanpa ekstensi tambahan) dulu; TimescaleDB bila volume memaksa.                                                                                                                                 |

## 3. Konvensi migrasi

- [ ] Satu migrasi = satu perubahan logis. Selalu ada `Up` dan `Down` yang bisa dijalankan (goose, seperti sekarang).
- [ ] **Tidak ada data demo di migrasi.** Data referensi hanya boleh di migrasi bila tidak pernah berubah; yang lain masuk seeder.
- [ ] Idempoten dan aman diulang: `IF NOT EXISTS`, `ON CONFLICT`. **Dilarang** `DELETE`/`TRUNCATE` data di migrasi.
- [ ] Jangan mengubah migrasi yang sudah diterapkan di lingkungan bersama (kecuali `000006` pada task F1). Perubahan = berkas baru dengan nomor berikutnya.
- [ ] Pola aman untuk tabel besar: tambah kolom nullable → backfill bertahap → `SET NOT NULL`; `CREATE INDEX CONCURRENTLY` dengan `-- +goose NO TRANSACTION`.
- [ ] `ALTER TYPE … ADD VALUE` harus di migrasi terpisah (`NO TRANSACTION`) dan tidak boleh dipakai di migrasi yang sama.
- [ ] Nama: `NNNNNN_<fitur>_<perubahan>.sql`. Satu komentar di kepala berkas yang menyebut fitur/todo terkait.
- [ ] Constraint di level DB untuk invarian bisnis yang murah: `CHECK (harga > 0)`, `CHECK (divalidasi_oleh IS DISTINCT FROM petugas_id)`, `CHECK (ambang_waspada < ambang_siaga AND ambang_siaga < ambang_kritis)`.
- [ ] Email unik tak peka huruf besar-kecil (`UNIQUE (lower(email))` atau `citext`).
- [ ] Lint migrasi di CI (mis. `squawk`) untuk menangkap operasi yang mengunci tabel.

## 4. Strategi seeder: tiga lapis

Pisahkan menurut keamanan dan tujuan. Semua seeder idempoten (upsert) dan deterministik (PRNG dengan seed tetap; tanggal relatif terhadap parameter `now`, seperti seeder cold-storage).

| Lapis          | Isi                                                                                                                                                                                                                       | Aman di produksi?                                  | Perintah                      |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- | ----------------------------- |
| **referensi**  | provinsi, kabupaten/kota, kecamatan, pasar, komoditas, satuan, definisi tahap intervensi, ambang EWS awal, katalog sumber data, definisi dan target KPI, templat brief, versi kode etik, (opsional) daftar izin dan peran | **Ya**, upsert                                     | `make db/seed/reference` |
| **admin awal** | satu akun admin dari env (`SEED_ADMIN_EMAIL`, password dari env atau acak dan dicetak sekali), `wajib_ganti_password`, MFA wajib                                                                                          | **Ya**, hanya bila belum ada admin                 | `make db/seed/admin`     |
| **demo**       | 25 akun contoh, histori harga, EWS, intervensi, brief, tugas, prediksi, KPI, audit, cold storage (lot dan suhu)                                                                                                           | **Tidak.** Menolak jalan bila `APP_ENV=production` | `make db/seed/demo`      |
| **test**       | fixture per tes (tiap tes membuat pengguna dan datanya sendiri)                                                                                                                                                           | n/a                                                | di dalam tes                  |

- [ ] Ubah `cmd/seed` menjadi subperintah (`reference|admin|demo|all`) dengan guard `APP_ENV`. Ubah Makefile (`db/seed`, `db/seed/demo`) dan dokumentasi.
- [ ] Data demo diberi penanda (kolom `sumber = 'DEMO'`, atau kolom `is_demo`) agar bisa disaring atau dibersihkan, dan UI menampilkan banner "Data demo".
- [ ] **Angka yang tampak resmi** (inflasi BPS, neraca pangan, ruang fiskal DJPb) tidak pernah di-seed di produksi; hanya masuk lewat impor dari sumbernya ([05](05-sumber-data.md)).
- [ ] Seed demo meniru bentuk `lib/api/mock.ts` dan `lib/data/generators.ts` sehingga UI tampil sama dengan mode mock. Tambahkan tes yang membandingkan jumlah/ID kunci.
- [ ] `cmd/seed-analytics` (MongoDB): jadikan idempoten (upsert atau hapus-lalu-isi per koleksi) dan masukkan ke subperintah yang sama, atau hapus bila D4 memutuskan tanpa Mongo.

**Urutan dependensi seed demo:** referensi → pengguna → harga → (refresh agregat/MV) → EWS → intervensi → tugas → prediksi → policy brief → KPI → audit → cold storage.

## 5. Tugas fondasi

Urutan semua task (Fase P/F) ada di [00 §6](00-cara-kerja.md). Berikut rincian fondasi yang berlaku lintas fitur.

> **Perubahan dari versi awal berkas ini:** langkah "selaraskan semua modul dengan migrasi" (dulu M1) **tidak lagi dikerjakan terpusat di depan.** Setiap modul diselaraskan di slice `-A`-nya sendiri, dan dibuktikan oleh tes kontrak skema (F0-3) lewat daftar `belumSelaras` yang berkurang satu per slice.

### F0-1 — Dev proxy dan alur lokal
- [x] Tambah `rewrites` di [next.config.ts](../../apps/web/next.config.ts): `/api/v1/:path*` → `${GO_API_BASE_URL}/:path*` (hanya dev). Tanpa ini, `NEXT_PUBLIC_API_BASE_URL=/api` memberi 404 pada `make dev/web` karena Caddy tidak berjalan.
- [x] Makefile: `db/seed/reference`, `db/seed/demo`, `db/seed/admin` (sekarang hanya `db/seed`); perluas `test/api/integration` agar mencakup semua paket integrasi.
- [x] README akar: alur lokal `docker/infra/up → db/migrate → db/seed/* → dev/api → dev/web`.

### F0-2 — Domain email dan 4 tes cold-storage
- [x] Pilih satu domain (seed sudah `@gardainflasi.id`) dan seragamkan di [integration_test.go](../../apps/api/internal/cold-storage/integration/integration_test.go) (baris 258–263), `env.example`, `packages/contracts/openapi.yaml`, serta tes lain yang memakai `@garda-inflasi.id`.
- [x] `go test ./internal/cold-storage/...` hijau.

### F0-3 — Tes kontrak skema
- [x] Paket tes (dockertest) yang: (1) menerapkan semua migrasi memakai library `github.com/pressly/goose/v3` (ganti `splitGoose` buatan sendiri di tes), (2) mengambil daftar entity ber-`TableName()`, (3) membandingkan tabel, kolom, dan kompatibilitas tipe dengan `information_schema`.
- [x] **Daftar `belumSelaras`**: peta `modul → alasan` berisi modul yang saat ini terbukti tidak cocok (`ews`, `harga`, `intervensi`, `lapangan`, `policy-brief`, `prediksi`, `siklus`, `spasial`, `sumber-data`, plus tabel fiktif dashboard). Modul di daftar ini dilewati dengan peringatan; modul di luar daftar harus lulus. Menghapus entri dari daftar adalah bagian DoD tiap slice `-A`.
- [x] Tes gagal bila modul di `belumSelaras` ternyata sudah cocok (agar daftar tidak basi).
- [x] `up → down → up` pada DB kosong diuji (`TestMigrationsApplyUpDownUp`); berkas CI sendiri belum ada ([18 §F](18-platform-lintas-fitur.md)).

### F0-4 — Seeder kit
- [x] Paket `internal/seedkit`: antarmuka `Seeder { Name; Tier; DependsOn; Run(ctx, db, now) }`, registry, pengurut topologis, ringkasan hasil.
- [x] `cmd/seed` menerima subperintah `reference | admin | demo | all` dan menolak `demo` bila `APP_ENV=production`.
- [x] Pindahkan seeder cold-storage ke registry (fasilitas = `reference`, lot = `demo`; lihat [13](13-cold-storage.md)).
- [x] `cmd/seed-analytics` dihapus bersama modul `analytics` dan MongoDB (D4: Postgres saja).
- [x] Tes: menjalankan seeder dua kali tidak menambah baris; `demo` menolak produksi.

### F0-5 — Tes asap seed
- [x] Kerangka selesai (`internal/platform/smoketest`, kasus cold storage sebagai contoh). Daftar endpoint bertambah satu per slice `-A`: tiap slice menambah `smoke_<modul>_test.go`.

### F0-6 — Migrasi nyata saat deploy (gerbang produksi)
- [x] Sediakan service/image `migrate` (binary goose + `db/migrations`, atau `cmd/migrate` dengan migrasi di-`embed`) di `compose.prod.yaml`, dijalankan sebelum API.
- [x] Hapus pola `A && B || echo` di [remote-deploy.sh](../../scripts/deploy/remote-deploy.sh) langkah 6; kegagalan migrasi harus menghentikan deploy. Cetak versi skema terakhir.
- [x] Deploy hanya menjalankan seeder `reference` dan `admin`, tidak pernah `demo`.

### F0-7 — Proteksi CSRF dan cookie
- [x] Middleware Go: untuk request ber-cookie (tanpa header `Authorization`) yang mengubah data, wajibkan `Origin`/`Referer` sama dengan origin aplikasi atau header `X-CSRF-Token` yang valid. Saat ini Go tidak memeriksa CSRF sama sekali.
- [x] [cookies.ts](../../apps/web/src/lib/bff/cookies.ts) memakai `const isDev = true` permanen, sehingga cookie token tidak pernah `Secure`. Ganti dengan deteksi lingkungan (atau konfigurasi TLS di Caddy) dan tambahkan tes.

### F1 — Netralkan seed di migrasi 000006 (gerbang produksi)
Dikerjakan **setelah** 01-A, 02-A, dan 04-A, karena isinya dipindah ke seeder oleh ketiganya:
- [x] Akun pengguna → seeder `demo` ([01-A](01-auth.md)); admin awal → seeder `admin`.
- [x] `role_permissions` dan `DELETE FROM role_permissions` → hilang sesuai keputusan D3 ([02-A](02-pengguna-rbac.md)).
- [x] provinsi, kabupaten, pasar, komoditas → seeder `reference` ([04-A](04-master-data.md)).
- [x] Ubah `000006_seed_data.sql` menjadi *no-op* (hanya komentar yang menunjuk ke seeder), supaya DB baru tidak lagi mendapat akun demo atau penghapusan izin. DB dev lama tidak terpengaruh; reset manual bila ingin bersih.
- [x] Jalankan ulang seluruh tes (auth, cold-storage, schematest, smoketest) yang sebelumnya bergantung pada data 000006: kini memakai seeder `master.reference`/`auth.demo` atau fixture sendiri. `TestMigrationsLeaveEveryTableEmpty` menjaga agar migrasi tidak menanam data lagi.

## 6. Tes yang harus ada

- [x] Tes kontrak skema entity ↔ migrasi (F0-3): `TestEntitiesMatchMigrations` (`internal/platform/schematest`), termasuk kewajiban tag `type:uuid` untuk kolom uuid.
- [x] Migrasi `up/down/up` pada DB kosong: `TestMigrationsApplyUpDownUp`. Migrasi konversi kunci primer 000110–000115 juga punya tes migrasi data sendiri (isi data lama → Up → periksa → Down → periksa → Up).
- [x] Seeder idempoten (dua kali → baris sama); `demo` menolak `APP_ENV=production`: tes `seeder` per modul (dua kali jalan, dan di atas DB hasil migrasi kunci primer), `seedkit` dan `cmd/seed` menolak `production`.
- [x] Seeder `reference` berjalan di atas DB berisi data produksi tanpa mengubah data non-referensi.
- [x] Tes asap seed (F0-5): `internal/platform/smoketest`.
- [x] Tidak ada akun ber-password default setelah `reference + admin`.

## 6a. Standardisasi kunci primer (7 Nov 2026)

Kontrak: [SKEMA-DATABASE.md](../SKEMA-DATABASE.md). Matriks kategori PK seluruh tabel ada di `kategoriPK`
(`internal/platform/schematest/kunci_test.go`) dan dijaga `TestInvarianKunci`. Tabel baru wajib terdaftar dan
selaras sejak migrasi pertamanya.

- [x] `000109` fondasi: `garda_uuid_stabil(ruang, kunci)` (UUID v3 deterministik, identik dengan `idx.UUIDStabil`).
- [x] `000110` `pengguna` → UUID beserta 41 kolom FK, tabel `pengguna_id_lama` untuk Down dan hash kode MFA lama.
- [x] `000111` harga, lapangan, master; `000112` EWS, intervensi; `000113` cold storage, spasial (termasuk membuat
      ulang `mv_kpi_operasional`); `000114` prediksi, policy brief; `000115` sumber data, ingestion, eksekutif, siklus,
      KPI, dashboard, audit.
- [x] Konvensi seeder:
  - Baris UUID memakai `idx.UUIDStabil("<tabel>", "<ID lama>")`, sama dengan hasil migrasi.
  - Baris BIGINT tidak mengisi `id`. Upsert memakai `ON CONFLICT (kode)` atau kunci alami, tidak pernah
    `ON CONFLICT (id)` untuk kolom identity.
  - Anak menemukan induknya lewat kode.
- [x] Gladi pada salinan data demo (versi 108):
  - Up 000109–000115 dalam ±6 detik tanpa mengubah jumlah baris.
  - Down ke 108 memulihkan isi tabel (md5 per tabel) dan struktur (kolom, indeks, constraint, trigger, view) persis.
  - Seed ulang dua kali tanpa duplikat.
- [ ] Sebelum produksi: jalankan kueri penjaga migrasi pada salinan data produksi. Kasus yang menghentikan migrasi:
      ID lebih dari 64 karakter, ID brief bukan `PB-YYYY-MM`, dan baris anak yatim. Jadwalkan jendela pemeliharaan,
      karena `ALTER COLUMN TYPE` memegang kunci `ACCESS EXCLUSIVE` pada `entri_harga`, `audit_log`, dan `data_mentah`.
- [ ] Setelah semua kode cadangan MFA lama dirotasi, `pengguna_id_lama` boleh dibuang (migrasi baru; Down 000110
      tidak lagi bisa memulihkan slug).

## 7. Operasional database (setelah Fase P, sebelum produksi)

- [x] Backup terjadwal (`pg_dump` atau WAL archiving) dan **uji restore** ke lingkungan lain; catat hasilnya di [DEPLOYMENT.md](../DEPLOYMENT.md). Selesai: `scripts/backup/{backup,restore,verify-restore}.sh`, target `make db/backup`, `db/restore`, `db/backup/verify`, service compose profile `backup`. `verify-restore` terbukti pada Postgres demo (goose 51, 42 tabel cocok, database sementara dihapus); hasil dan RPO/RTO ada di [DEPLOYMENT.md §10](../DEPLOYMENT.md). Belum: salinan backup ke luar VM dan WAL/PITR.
- [ ] Kebijakan rollback: kapan `goose down`, kapan migrasi koreksi ke depan. Larang `down` yang menghapus data produksi tanpa backup.
- [ ] Job pemeliharaan: partisi bulanan, refresh materialized view, pembersihan `jwt_blacklist`/`refresh_tokens` kedaluwarsa, retensi data mentah. Sudah: partisi bulanan dan retensi `audit_log` (job `audit_partisi`, [03-C](03-audit.md)), refresh materialized view (`dashboard_refresh`), pembersihan tabel auth (`auth_pembersihan`). Belum: retensi `data_mentah` (bronze ingestion).
- [x] Pengguna DB aplikasi berhak minimum (tanpa DDL, tanpa `UPDATE/DELETE` pada `audit_log`); pengguna terpisah untuk migrasi. Migrasi 000119 membuat peran grup `garda_aplikasi`; `cmd/migrate` membuat login `DB_APP_USER` (anggotanya), dan compose menghubungkan api dan worker dengan login itu, sedangkan migrate dan seed tetap memakai pemilik skema. Rincian di [03-C](03-audit.md) dan [DEPLOYMENT §10b](../DEPLOYMENT.md).
