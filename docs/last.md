
Berikut adalah draf **Prompt Spesifikasi & Instruksi Kerja Komprehensif** yang siap digunakan untuk mengeksekusi refaktorisasi arsitektur seeder dari kode Go di `internal/<modul>/seeder` menjadi berkas SQL deklaratif di `db/seeds/`, serta memperbarui PRD dan seluruh dokumentasi TODOs:

***

```markdown
# PROMPT INSTRUKSI KERJA:

REFAKTORISASI SEEDER KE DIRECT SQL (db/seeds/) & PEMBARUAN STANDAR ARSITEKTUR

## 1. Konteks & Latar Belakang Masalah
Saat ini, proyek `garda-inflasi` memiliki:
1. Enam belas (16) folder `seeder/` yang tersebar di dalam paket modul domain `apps/api/internal/<modul>/seeder/` (`master`, `auth`, `cold-storage`, `ingestion`, `sumber-data`, `harga`, `lapangan`, `dashboard`, `ews`, `intervensi`, `spasial`, `policy-brief`, `kpi`, `prediksi`, `eksekutif`, `audit`).
2. Berkas pendaftaran pusat `apps/api/internal/seed/register.go` mengimpor ke-16 paket seeder tersebut dan mengikat inisialisasi basis data dengan model ORM GORM serta runtime kompilasi Go.
3. Arsitektur ini mengotori Clean Architecture (lapisan internal domain tercampur dengan mock generator & fixture statis), memperbesar ukuran biner, memperlambat proses inisialisasi, dan menghalangi *quick bootstrapping* database murni via `psql` tanpa runtime Go.

## 2. Tujuan Pekerjaan
1. **Pola Baru (SQL-First Seeder):** Memindahkan seluruh inisialisasi data benih (seed) menjadi berkas SQL deklaratif murni (`.sql`) yang terorganisir di dalam folder `db/seeds/`.
2. **Pembersihan Total Lapisan Internal:** Menghapus seluruh 16 folder `apps/api/internal/<modul>/seeder/` beserta ketergantungan import-nya di `apps/api/internal/seed/`.
3. **Refaktorisasi Seeder Runner (`apps/api/internal/seed`):** Mengubah runner `cmd/seed` agar mengeksekusi berkas-berkas `.sql` dari `db/seeds/` berdasarkan tingkatan (*tier*), tetap mempertahankan guard keamanan produksi (`APP_ENV=production`), dan mendukung eksekusi idempoten.
4. **Pembaruan Infrastruktur & Docker:** Menyesuaikan `docker/api.Dockerfile`, `compose.yaml`, dan `Makefile` agar menyalin dan membaca `db/seeds/`.
5. **Standardisasi PRD & Todos:** Memperbarui `docs/prd-monorepo.md`, `docs/prd-prp.md`, `docs/todos/00-cara-kerja.md`, `docs/todos/19-migration-seeder.md`, dan berkas todos modul berikutnya agar secara konsisten mengadopsi pola seeder SQL di `db/seeds/`.

---

## 3. Spesifikasi Arsitektur Direktori `db/seeds/`

Struktur folder diatur secara terisolasi per tingkatan (*tier*) dengan penomoran urut eksekusi yang menjamin dependensi Foreign Key terpenuhi:

```text
db/seeds/
├── reference/                  # Tier 1: Data referensi master & sistem (Aman untuk Produksi)
│   ├── 01_master.sql           # Satuan, Provinsi Aceh, 23 Kab/Kota, Komoditas Pokok, Pasar Percontohan
│   ├── 02_kecamatan.sql        # Master seluruh kecamatan di Aceh (migrasi dari kecamatan_aceh.csv)
│   ├── 03_auth_rbac.sql        # 14 Peran baku, 35 izin baku (domain.aksi), penugasan role_permissions
│   ├── 04_sumber_data.sql      # Katalog sumber eksternal (BPS, BI, BMKG, Bapanas, OWM)
│   ├── 05_ingestion_mapping.sql# Pemetaan alias komoditas/wilayah sumber ke master
│   ├── 06_harga_pasar.sql      # Rujukan pedagang pasar percontohan
│   ├── 07_ews_aturan.sql       # Ambang batas & aturan deteksi dini awal
│   ├── 08_intervensi_tahap.sql # Definisi tahap intervensi pengendalian inflasi
│   ├── 09_cold_storage.sql     # Fasilitas cold storage resmi
│   ├── 10_spasial_geometri.sql # Koordinat dan centroid kabupaten/kota
│   ├── 11_prediksi_model.sql   # Registri model prediksi harga awal
│   ├── 12_policy_brief_tpl.sql # Templat standar kajian kebijakan
│   └── 13_kpi_definisi.sql     # Definisi indikator makro, batas toleransi, dan target IHK
├── admin/                      # Tier 2: Akun Administrator Awal (Bootstrap Produksi)
│   └── 01_admin_user.sql       # Superadmin awal (menggunakan hash bcrypt aman atau variabel SQL)
└── demo/                       # Tier 3: Data Simulasi & Paritas Mock (Ditolak di Produksi)
    ├── 01_auth_users.sql       # 25 Akun pengguna simulasi per peran (password: Garda2024!)
    ├── 02_master_pedagang.sql  # Data pedagang pasar simulasi
    ├── 03_harga_harian.sql     # Entri harga pangan harian & histori per kabupaten
    ├── 04_cuaca_harian.sql     # Data cuaca & anomali iklim simulasi
    ├── 05_ingestion_demo.sql   # Sampel log data mentah (bronze) & riwayat eksekusi
    ├── 06_ews_alerts.sql       # Insiden & alert peringatan dini simulasi
    ├── 07_intervensi_kartu.sql # Kartu intervensi lintas status (kanban)
    ├── 08_lapangan_tugas.sql   # Penugasan enumerator/pencacah harga lapangan
    ├── 09_spasial_neraca.sql   # Simulasi neraca pangan surplus/defisit spasial
    ├── 10_prediksi_harga.sql   # Simulasi proyeksi harga pangan & faktor XAI
    ├── 11_cold_storage_ops.sql # Simulasi kapasitas terpakai, lot komoditas, dan log suhu
    ├── 12_policy_brief.sql     # Draf brief kebijakan inflasi simulasi
    ├── 13_kpi_rekam.sql        # Rekaman realisasi KPI bulanan
    ├── 14_eksekutif_data.sql   # Ringkasan fiskal & komoditas eksekutif
    ├── 15_audit_trail.sql      # Jejak log audit aktivitas simulasi
    └── 16_refresh_views.sql    # Refresh materialized view (gold_harga_harian, dll.)
```

### Konvensi Berkas SQL
1. **Idempoten:** Setiap pernyataan wajib aman dijalankan berulang kali tanpa membuat duplikasi baris atau galat *unique constraint*:
   - Gunakan `INSERT INTO ... ON CONFLICT (...) DO NOTHING;` atau `DO UPDATE SET ...`.
2. **Kemandirian (Self-contained):** Nilai ID/UUID yang dijadikan referensi relasi (FK) antar-berkas harus deterministik (misalnya menggunakan UUID v5 atau penamaan UUID stabil `idx.UUIDStabil`).
3. **Format Tanggal Dinamis:** Untuk data demo berbasis waktu, gunakan ekspresi SQL dinamis seperti `CURRENT_DATE - INTERVAL 'N days'` agar data demo selalu relevan saat dieksekusi kapan saja.

---

## 4. Rincian Pemetaan & Migrasi 16 Modul (`internal/<modul>/seeder` → `db/seeds/`)

| # | Modul Domain | Berkas Go Lama | Berkas SQL Target Baru | Deskripsi Data yang Dimigrasi |
|---|--------------|----------------|------------------------|-------------------------------|
| 1 | `master` | `internal/master/seeder/data.go`, `kecamatan.go`, `demo.go` | `db/seeds/reference/01_master.sql`, `02_kecamatan.sql`, `db/seeds/demo/02_master_pedagang.sql` | Master satuan, provinsi Aceh, 23 kabupaten/kota, komoditas pangan, pasar, kecamatan se-Aceh, pedagang demo. |
| 2 | `auth` | `internal/auth/seeder/rbac.go`, `admin.go`, `demo.go` | `db/seeds/reference/03_auth_rbac.sql`, `db/seeds/admin/01_admin_user.sql`, `db/seeds/demo/01_auth_users.sql` | 14 peran sistem, 35 izin sistem, pemetaan peran-izin, akun superadmin awal, serta 25 akun demo berbagai peran. |
| 3 | `sumber-data` | `internal/sumber-data/seeder/data.go` | `db/seeds/reference/04_sumber_data.sql` | Katalog sumber API & scraper resmi (BPS, BI, BMKG, Bapanas, OpenWeather) beserta URL & atribusi. |
| 4 | `ingestion` | `internal/ingestion/seeder/data.go`, `demo.go` | `db/seeds/reference/05_ingestion_mapping.sql`, `db/seeds/demo/05_ingestion_demo.sql` | Pemetaan alias nama daerah & komoditas dari format sumber luar ke kode master, plus sampel data bronze demo. |
| 5 | `harga` | `internal/harga/seeder/reference.go`, `seeder.go` | `db/seeds/reference/06_harga_pasar.sql`, `db/seeds/demo/03_harga_harian.sql` | Pedagang pasar percontohan rujukan dan data transaksi entri harga harian demo 30 hari ke belakang. |
| 6 | `ews` | `internal/ews/seeder/data.go`, `seeder.go` | `db/seeds/reference/07_ews_aturan.sql`, `db/seeds/demo/06_ews_alerts.sql` | Aturan matriks peringatan dini ambang batas volatilitas dan tiket/riwayat peringatan demo. |
| 7 | `intervensi` | `internal/intervensi/seeder/data.go`, `seeder.go` | `db/seeds/reference/08_intervensi_tahap.sql`, `db/seeds/demo/07_intervensi_kartu.sql` | Definisi alur tahapan intervensi TPID dan kartu tindakan operasi pasar/subsidi demo. |
| 8 | `cold-storage` | `internal/cold-storage/seeder/seeder.go`, `operasi.go` | `db/seeds/reference/09_cold_storage.sql`, `db/seeds/demo/11_cold_storage_ops.sql` | Data master fasilitas cold storage BUMD/swasta di Aceh dan lot inventaris serta log suhu simulasi. |
| 9 | `lapangan` | `internal/lapangan/seeder/seeder.go` | `db/seeds/demo/08_lapangan_tugas.sql` | Penugasan enumerator pencacahan pasar mingguan demo. |
| 10 | `spasial` | `internal/spasial/seeder/data.go`, `geometri.go` | `db/seeds/reference/10_spasial_geometri.sql`, `db/seeds/demo/09_spasial_neraca.sql` | Titik koordinat spasial kabupaten/kota dan simulasi surplus/defisit neraca pangan antar-wilayah. |
| 11 | `prediksi` | `internal/prediksi/seeder/seeder.go` | `db/seeds/reference/11_prediksi_model.sql`, `db/seeds/demo/10_prediksi_harga.sql` | Metadata model machine learning (LSTM/RandomForest) dan data proyeksi harga 14 hari ke depan. |
| 12 | `policy-brief`| `internal/policy-brief/seeder/seeder.go` | `db/seeds/reference/12_policy_brief_tpl.sql`, `db/seeds/demo/12_policy_brief.sql` | Templat laporan TPID dan dokumen rancangan rekomendasi kebijakan inflasi demo. |
| 13 | `kpi` | `internal/kpi/seeder/data.go`, `seeder.go` | `db/seeds/reference/13_kpi_definisi.sql`, `db/seeds/demo/13_kpi_rekam.sql` | Target indikator IHK, andil inflasi pangan, ketersediaan pasokan, dan capaian bulanan demo. |
| 14 | `eksekutif` | `internal/eksekutif/seeder/data.go`, `seeder.go` | `db/seeds/demo/14_eksekutif_data.sql` | Metrik inflasi bulanan eksekutif dan ruang fiskal daerah demo. |
| 15 | `audit` | `internal/audit/seeder/seeder.go` | `db/seeds/demo/15_audit_trail.sql` | Log pencatatan aktivitas awal untuk pengujian audit trail. |
| 16 | `dashboard` | `internal/dashboard/seeder/seeder.go` | `db/seeds/demo/16_refresh_views.sql` | Perintah penyegaran materialized view dan ringkasan dasbor. |

---

## 5. Refaktorisasi Runner CLI (`apps/api/internal/seed`)

1. **Struktur Biner `cmd/seed`:**
   - Pertahankan entry point sederhana `apps/api/cmd/seed/main.go` yang memanggil `seed.Run(...)`.
2. **Implementasi `apps/api/internal/seed/runner.go`:**
   - Hapus berkas `register.go` yang mengimpor 16 paket modul.
   - Buat fungsi pembaca berkas SQL:
     - Cari folder seeds dari environment `SEEDS_DIR` (dengan fallback default ke `../../db/seeds` saat dijalankan via `go run`).
     - Alternatif/pelengkap: Dukung `//go:embed` atau eksekusi filesystem langsung.
   - Eksekusi terurut berdasarkan argumen:
     - `reference`: Mengeksekusi seluruh berkas di `db/seeds/reference/*.sql` secara alfabetis/terurut.
     - `admin`: Mengeksekusi seluruh berkas di `db/seeds/admin/*.sql`.
     - `demo`: Mengeksekusi seluruh berkas di `db/seeds/demo/*.sql`.
     - `all`: Mengeksekusi `reference` → `admin` → `demo`.
   - **Guard Keamanan Wajib:**
     - Jika argumen memuat `demo` atau `all`, periksa `APP_ENV`. Jika `APP_ENV=production`, eksekusi **wajib langsung ditolak** dengan kode galat sebelum membuka koneksi database (`ErrDemoInProduction`).
   - Setiap berkas dieksekusi di dalam transaksi basis data (`BEGIN ... COMMIT`) menggunakan koneksi raw `*sql.DB` atau `gorm.DB.Exec(...)` dengan pencatatan log nama file dan statusnya.

---

## 6. Pembaruan Dockerfile, Compose, & Makefile

1. **`docker/api.Dockerfile`**:
   Tambahkan penyalinan berkas seed ke tahap runtime:
   ```dockerfile
   # Salin migrations dan seeds ke stage runtime
   COPY db/migrations /migrations
   ENV MIGRATIONS_DIR=/migrations

   COPY db/seeds /seeds
   ENV SEEDS_DIR=/seeds
   ```
2. **`compose.yaml`**:
   Pastikan container `seed` (`garda-seed`) memiliki variabel lingkungan `SEEDS_DIR=/seeds` dan volume terkait jika di-mount di mode pengembangan.
3. **`Makefile`**:
   - Pastikan target `db/seed/reference`, `db/seed/admin`, dan `db/seed/demo` tetap berfungsi tanpa perubahan sintaks pemanggilan:
     ```makefile
     db/seed/reference: ## Seed data referensi langsung via SQL
     	cd apps/api && SEEDS_DIR=../../db/seeds go run ./cmd/seed reference

     db/seed/admin: ## Seed akun admin awal via SQL
     	cd apps/api && SEEDS_DIR=../../db/seeds go run ./cmd/seed admin

     db/seed/demo: ## Seed data simulasi demo via SQL (ditolak di production)
     	cd apps/api && SEEDS_DIR=../../db/seeds go run ./cmd/seed demo
     ```
   - Perbarui variabel `INTEGRATION_PACKAGES` di `Makefile` dengan menghapus path seeder yang telah dieliminasi:
     - Hapus: `./internal/harga/seeder/...`, `./internal/lapangan/seeder/...`, `./internal/dashboard/seeder/...`, `./internal/ingestion/seeder/...`, `./internal/master/seeder/...`, `./internal/audit/seeder/...`.

---

## 7. Pembaruan Dokumentasi (PRD & TODOs)

Wajib memperbarui dokumen panduan agar tim dan subagen pada modul berikutnya tidak kembali membuat folder seeder di internal:

1. **`docs/todos/00-cara-kerja.md`**:
   - Ubah definisi langkah **SEED** pada siklus slice:
     - **Lama:** *Letakkan di `apps/api/internal/<modul>/seeder/` (pola cold-storage), daftarkan ke registry seeder (task F0-4)...*
     - **Baru:** *Tambahkan langsung berkas seeder `.sql` di `db/seeds/reference/<nomor>_<fitur>.sql` (untuk data master/referensi) dan/atau `db/seeds/demo/<nomor>_<fitur>.sql` (untuk data simulasi/demo). Wajib idempoten (`ON CONFLICT DO NOTHING/UPDATE`). Dilarang membuat folder `seeder/` di dalam `apps/api/internal/<modul>/`.*
   - Perbarui checklist *Definition of Done* (DoD) pada butir seeder.
2. **`docs/todos/19-migration-seeder.md`**:
   - Perbarui Bagian 4 (*Strategi seeder: tiga lapis*) dan Bagian 5 (*F0-4 Seeder kit*).
   - Nyatakan secara eksplisit bahwa arsitektur seeder telah dimigrasi ke **SQL-First Seeder** di `db/seeds/` dengan pembagian direktori `reference/`, `admin/`, dan `demo/`.
3. **`docs/prd-monorepo.md` & `docs/prd-prp.md`**:
   - Selaraskan deskripsi struktur pohon direktori: hilangkan `internal/{modul}/seeder/` dan cantumkan `db/seeds/{reference,admin,demo}/` sebagai *single source of truth* data awal sistem.
4. **`docs/todos/README.md` & TODOs Modul 01 s.d. 16**:
   - Perbarui kolom atau keterangan seeder di ringkasan status bahwa seluruh data benih dikelola langsung via berkas `.sql` di folder `db/seeds/`.

---

## 8. Langkah-Langkah Eksekusi (Step-by-Step Execution Plan)

1. **Langkah 1: Ekstraksi Data SQL ke `db/seeds/`**
   - Buat subdirektori `db/seeds/reference/`, `db/seeds/admin/`, dan `db/seeds/demo/`.
   - Konversikan seluruh data Go fixture dari 16 modul lama menjadi berkas `.sql` berurutan sesuai daftar di Bagian 4.
   - Pindahkan data kecamatan dari CSV/kode Go menjadi `db/seeds/reference/02_kecamatan.sql`.
   - Pastikan setiap query SQL memiliki klausa `ON CONFLICT` idempoten.

2. **Langkah 2: Perbarui Seeder Runner Go (`apps/api/internal/seed`)**
   - Hapus `apps/api/internal/seed/register.go`.
   - Tulis ulang runner di `apps/api/internal/seed/runner.go` untuk membaca direktori `SEEDS_DIR` atau `../../db/seeds/`, memvalidasi environment, dan mengeksekusi file `.sql` secara berurutan dalam transaksi per file.
   - Perbarui `apps/api/internal/seed/seed_test.go` untuk memvalidasi pemanggilan CLI, penolakan tier demo pada production, dan penanganan galat sintaks SQL.

3. **Langkah 3: Pembersihan Folder Seeder Go**
   - Hapus ke-16 folder `seeder/` di `apps/api/internal/*`.
   - Jalankan `go -C apps/api test ./internal/...` untuk memastikan tidak ada kode aplikasi yang tertinggal mengimpor paket seeder yang dihapus.

4. **Langkah 4: Penyesuaian Docker, Compose, & Makefile**
   - Tambahkan `COPY db/seeds /seeds` di `docker/api.Dockerfile`.
   - Hapus path paket seeder yang telah dihapus dari variabel `INTEGRATION_PACKAGES` di `Makefile`.

5. **Langkah 5: Pembaruan Dokumen PRD & TODOs**
   - Perbarui `00-cara-kerja.md`, `19-migration-seeder.md`, `docs/prd-monorepo.md`, dan todos terkait.

6. **Langkah 6: Verifikasi & Uji Menyeluruh**
   - Jalankan pengujian idempotensi:
     ```bash
     make db/seed/reference
     make db/seed/reference  # Putaran kedua tidak boleh galat / baris tidak bertambah ganda
     make db/seed/demo
     make db/seed/demo       # Putaran kedua tidak boleh galat
     ```
   - Jalankan seluruh tes Go backend:
     ```bash
     go -C apps/api test ./... -v
     ```
   - Jalankan Playwright E2E untuk memverifikasi antarmuka tetap memuat data seeder dengan benar.

---

## 9. Kriteria Keberhasilan (Definition of Done)
- [ ] Folder `db/seeds/reference/`, `db/seeds/admin/`, dan `db/seeds/demo/` terisi lengkap dan mencakup seluruh data 16 modul lama.
- [ ] Tidak ada lagi satupun folder `seeder/` di dalam `apps/api/internal/`.
- [ ] Biner `seed` (`cmd/seed`) dapat menjalankan `reference`, `admin`, dan `demo` murni dengan mengeksekusi file-file `.sql`.
- [ ] Tier `demo` dan `all` terbukti menolak jalan jika `APP_ENV=production`.
- [ ] `make db/seed/reference` dan `make db/seed/demo` dapat dijalankan 2x berturut-turut tanpa error dan tanpa menggandakan data.
- [ ] Dokumen `00-cara-kerja.md`, `19-migration-seeder.md`, `docs/prd-monorepo.md`, dan TODOs modul terkait telah diperbarui dengan aturan seeder SQL yang baru.
- [ ] Semua tes unit Go (`go test ./...`) dan tes integrasi/E2E tetap berstatus hijau (PASS).
```

***

### Catatan Keunggulan Prompt Ini:
1. **Komprehensif & Menyeluruh:** Mencakup inventarisasi ke-16 modul yang ada di `apps/api/internal/`, tidak ada modul yang terlewat.
2. **Mematuhi Arsitektur Idempoten & Multi-Tier:** Membedakan dengan tegas data aman produksi (`reference` dan `admin`) dengan data simulasi (`demo`), serta menjaga guard `APP_ENV=production`.
3. **Pembersihan Bersih (Clean Code):** Mengeliminasi ketergantungan import sirkular/monolitik di `internal/seed/register.go` dan membersihkan `INTEGRATION_PACKAGES` di `Makefile`.
4. **Instruksi Dokumentasi Jelas:** Menjelaskan secara presisi bagian mana dari `00-cara-kerja.md`, `19-migration-seeder.md`, dan PRD yang harus diamandemen agar konsisten untuk modul-modul selanjutnya.
