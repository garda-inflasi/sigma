# 02 — Pengguna, peran, dan RBAC

**Prioritas:** P0 · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes 🟡 · 02-C (Opsi B) ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19 §1.4](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [pengguna-page.tsx](../../apps/web/src/components/pengguna/pengguna-page.tsx) (daftar akun, aksi massal, matriks peran × izin) di `/pengguna`.
- **BFF:** `app/api/pengguna` (list, create, `[id]`, `[id]/roles`, `[id]/reset-password`, `bulk`) dan `app/api/rbac/{roles,permissions}`.
- **API Go:** `POST/GET /v1/users`, `GET/PATCH/DELETE /v1/users/:id`, `POST /v1/users/bulk`, `POST /v1/users/:id/reset-password`, `PUT /v1/users/:id/roles`, `GET /v1/roles`, `GET /v1/permissions` (semua butuh `pengguna.kelola`).
- **Catatan 02-C:** poin di bawah tentang peta `rolePermissions` dan tabel RBAC yang tidak dibaca adalah keadaan SEBELUM 02-A/02-C. Sejak 02-C (Opsi B) izin yang berlaku dibaca dari basis data; peta Go menjadi matriks bawaan. Lihat [02-C](#02-c--izin-langsung-per-pengguna-dan-matriks-yang-dapat-diedit-opsi-b).
- **Aturan di web:** 14 peran dan 35 izin di [roles.ts](../../apps/web/src/lib/rbac/roles.ts) / [permissions.ts](../../apps/web/src/lib/rbac/permissions.ts), dipakai sidebar, palet perintah, middleware, aksi massal.
- **Temuan penting (terverifikasi dari kode):**
  - Izin yang berlaku di backend berasal dari peta `rolePermissions` di [auth/usecase/usecase.go](../../apps/api/internal/auth/usecase/usecase.go), dikunci pada enum `peran` (14 nilai) dengan 35 izin `domain.aksi`.
  - Tabel `roles` (15 baris), `permissions` (29 baris format `harga:read`), `role_permissions`, `user_permissions` diisi migrasi 000001/000006 tetapi **tidak dibaca kode apa pun**. Nama izinnya tidak ada yang sama dengan yang dipakai kode; `roles` berisi "Superadmin" yang tidak ada di enum `peran`.
  - Route yang meminta `lapangan.lihat`, `lapangan.baca`, `lapangan.validasi`, `prediksi.baca` **tidak dimiliki peran manapun**, jadi selalu 403 (termasuk semua `/v1/lapangan/*`).
- **Detail pengguna (✅, terhubung):** dialog "Ubah pengguna" memuat `GET /v1/users/:id` lewat BFF (`penggunaApi.get`, hook `usePenggunaDetail`, tanpa cache agar formulir selalu segar) sebelum formulir tampil, dan menampilkan rincian akun (ID, cakupan, status, MFA, terakhir masuk, dibuat). Bila gagal dimuat, formulir memakai baris daftar. Perbaikan backend: `GetUser` tidak lagi menjawab 404 untuk galat basis data (hanya "tidak ada" yang 404, selain itu 500); tes usecase dan integrasi `get_user_test.go` (200 tanpa rahasia, 404, 403 tanpa `pengguna.kelola`, 401).
- **Undang, impor, dan ekspor (02-D, ✅):** `POST /v1/users/undang`, `POST /v1/users/:id/undang-ulang`, publik `POST /v1/auth/aktivasi` (halaman `/aktivasi`), `POST /v1/users/impor`, `GET /v1/users/ekspor`; rancangannya di [02-D](#02-d--undang-pengguna-lewat-email-impor-dan-ekspor-csv).

### Daftar pengguna sisi server (✅)

- **Kontrak** `GET /v1/users` (`pengguna.kelola`): `search` (nama, email, peran sebagai id atau dengan spasi, lembaga; metakarakter `%`/`_`/`\` dicari apa adanya, maks 100 karakter), filter `peran`, `status`, `lapis`, `mfa=true|false`, `terkunci=true|false`, urutan `sort` ∈ `nama|email|peran|lembaga|status|terakhir_masuk|dibuat_pada` + `arah=asc|desc`, `page`, `limit` (1-100, bawaan 20). Mengembalikan `{items,total,page,limit}`. Nilai tak dikenal, boolean selain `true`/`false`, atau `limit` > 100 dijawab **400**; parameter kosong sama dengan tidak ada.
- **Urutan total.** Kolom urut lalu `id` pada arah yang sama, jadi paginasi tidak menggandakan/melewatkan baris walau nilainya kembar dan `desc` adalah kebalikan persis `asc`. `nama`/`email`/`lembaga` tak peka huruf besar-kecil; `peran` dan `status` mengikuti urutan enum basis data (peran: hierarki lapis, bukan abjad id); `terakhir_masuk` menaruh akun yang belum pernah masuk selalu paling akhir. Tanpa `sort`: `dibuat_pada` menurun (perilaku lama).
- **Filter `terkunci`** memakai tabel lockout ([01-F](01-auth.md)): `kunci_login.email_hash` dihitung di SQL dari `email` (SHA-256 dari huruf kecil tanpa spasi tepi, sama dengan `usecase.HashEmailKunci`) dan `terkunci_sampai > sekarang`; paginasi tetap terjadi di basis data.
- **Satu pintu validasi** untuk daftar dan ekspor CSV pengguna (`GET /v1/users/ekspor`): `dto.FilterPenggunaRequest` (parameter kueri yang disematkan) → `domain.DaftarPenggunaQuery.Normal()` → `usecase.ListUsers` (peran divalidasi terhadap katalog). Galatnya `*domain.FilterPenggunaError` → `jawabGalatFilterPengguna` di handler.
- **Tanpa migrasi.** Tabel `pengguna` kecil dan pencarian `LIKE '%…%'` tak bisa memakai indeks btree; `kunci_login` dicari lewat kunci utama. Migrasi 000109 tidak dipakai.
- **Web.** `/pengguna` digerakkan server: kotak cari dengan jeda ketik 300 ms (Enter mengirim seketika), kontrol filter ([filter-pengguna.tsx](../../apps/web/src/components/pengguna/filter-pengguna.tsx)), kepala kolom yang bisa diurutkan (klik: naik, turun, kembali ke bawaan), dan paginasi dengan total dari server. Keadaan hidup di **URL** (`?search=&peran=&status=&lapis=&mfa=&terkunci=&sort=&arah=&page=&limit=`; nilai bawaan tidak ditulis; nilai asing diabaikan) lewat `window.history.pushState`/`replaceState` yang terintegrasi dengan `useSearchParams` Next, jadi muat ulang dan tombol kembali memulihkan tabel; halaman di luar jangkauan dikoreksi ke halaman terakhir. Logika murni di [lib/pengguna/tabel.ts](../../apps/web/src/lib/pengguna/tabel.ts) (ber-tes); `DataTable` mendapat prop opsional `server` (mode server, kontrak di [server-mode.ts](../../apps/web/src/components/data-table/server-mode.ts)) dan `filterBar`, tanpa mengubah perilaku halaman lain.
- **"Pilih semua" di mode server** berarti **hanya baris di halaman yang tampil**. Ringkasan di atas tabel menyebut berapa pengguna di halaman lain yang tidak ikut, dan pilihan dibersihkan begitu halaman, urutan, pencarian, atau filter berubah, sehingga aksi massal tidak pernah mengenai baris yang tidak terlihat. (Aksi massal lintas halaman butuh endpoint berbasis filter; belum ada.)

### Reset MFA oleh admin (✅)

- `POST /v1/users/:id/reset-mfa` (`pengguna.kelola`), audit `RESET_MFA` (sebelum/sesudah `mfa_aktif`; metadata jumlah kode cadangan dan sesi). Dalam **satu transaksi** (baris pengguna dikunci): `mfa_aktif=false`, `mfa_secret=NULL`, `mfa_langkah_terakhir=0`, semua `mfa_kode_cadangan` dihapus, dan, bila MFA memang aktif, semua sesi dicabut ([01-E](01-auth.md)): access token lama langsung ditolak.
- **Idempoten:** pengguna tanpa MFA dijawab 200 (`mfa_aktif_sebelumnya:false`, `sesi_dicabut:0`), baris tidak ditulis ulang, sesinya tidak disentuh; rahasia setup yang menggantung ikut dibersihkan. Setiap pemanggilan tercatat.
- **Akun sendiri ditolak (400).** Pemilik akun memakai `POST /v1/auth/mfa/disable`, yang meminta kata sandi dan menghormati MFA wajib; tanpa larangan ini sesi admin yang dibajak bisa melucuti MFA-nya sendiri tanpa kata sandi. Tanda `mfa_wajib` dan peran wajib MFA tidak dilepas: bila MFA diwajibkan lingkungan, login berikutnya mendapat `mfa_setup_diwajibkan` (hanya endpoint setup yang terbuka).
- **UI:** aksi baris "Reset MFA" (hanya untuk akun ber-MFA selain akun sendiri) membuka dialog konfirmasi ([reset-mfa-dialog.tsx](../../apps/web/src/components/pengguna/reset-mfa-dialog.tsx)) yang menjelaskan akibatnya; sesudah berhasil kolom MFA diperbarui. Status MFA per pengguna ada di kolom MFA, filter MFA, dan rincian akun.

### Riwayat peran (✅)

- `GET /v1/users/:id/riwayat-peran?limit=` (`pengguna.kelola`, bawaan 50, maks 200) → `{items:[{id,waktu,aksi,dari,ke,aktor:{id,nama}}], terpotong}` terbaru dulu, **dibaca dari `audit_log`** (append-only, indeks `(entitas, entitas_id, waktu DESC)` dari 000011): baris `BERHASIL` pada entitas `pengguna` dengan id ini yang mencatat `sesudah.peran` dan berbeda dari `sebelum.peran` (kosong = peran awal). Aksi tidak dibatasi pada daftar tertentu, jadi jalur baru yang mencatat snapshot dengan peran (undangan, impor CSV) ikut tanpa mengubah kueri. `aktor.id` null bila akun pelaku sudah dihapus (nama dari snapshot tetap terbaca); `id` entri = id baris audit (`GET /v1/audit/:id`). Akun seed tanpa audit pembuatan punya riwayat kosong.
- **Jalur yang mengubah peran dan mencatat sebelum/sesudah:** `CREATE_USER` (sesudah), `ASSIGN_ROLE` (`PUT /roles`), `UPDATE_USER` (`PATCH`). Perbaikan: audit dua jalur terakhir kini **selalu** ditulis begitu peran tersimpan, juga bila pencabutan sesi sesudahnya gagal (ditandai `metadata.pencabutan_sesi_gagal`); sebelumnya perubahan yang nyata hilang dari jejak.
- **UI:** bagian "Riwayat peran" di dialog "Ubah pengguna" ([riwayat-peran.tsx](../../apps/web/src/components/pengguna/riwayat-peran.tsx), pemetaan murni di [lib/pengguna/riwayat-peran.ts](../../apps/web/src/lib/pengguna/riwayat-peran.ts)) dengan tautan "Lihat di audit trail" ke `/audit?entitas=pengguna&entitas_id=<id>`. Halaman `/audit` kini membaca filter awal dari kueri URL (`entitas`, `entitas_id`, `aksi`, `aktor`, `hasil`, `q`, `dari`, `sampai`, `correlation_id`) dan menampilkan lencana "Entitas …" yang bisa dilepas.

### Tes

Go: unit (`domain`, `dto`, `usecase`, `delivery/http`) dan integrasi dockertest (`integration/daftar_pengguna_test.go`, `reset_mfa_test.go`, `riwayat_peran_test.go`). Web: `lib/pengguna/*.test.ts`, `lib/api/pengguna-daftar.test.ts`, `lib/api/audit-url.test.ts`, `lib/bff/api-client-pengguna.test.ts`, `components/data-table/server-mode.test.ts`. E2E: `e2e/pengguna-daftar.spec.ts`.

---

## 02-A — Satu sumber izin + perbaiki route yang selalu 403 _(Fase P, urutan #2)_

**Tujuan:** tidak ada izin yatim, tidak ada route yang mustahil diakses, dan tabel RBAC tidak lagi menyesatkan.
**Bergantung pada:** 01-A. **Sentuh juga:** F1 menghapus penugasan izin dari 000006.

**Keputusan D3 (default yang dipakai di slice ini: Opsi A).**

- **Opsi A (default):** peta di kode adalah sumber tunggal; tabel RBAC dibuang. Termurah dan sesuai kondisi nyata.
- **Opsi B:** tabel DB menjadi sumber. Pilih ini hanya bila "izin langsung per pengguna" dan matriks yang bisa diedit di UI memang dibutuhkan; langkahnya ada di 02-C.

> **Pembaruan (02-C): Opsi B kini berlaku.** Product owner memilih Opsi B dengan meminta 02-C. Migrasi `000107_02c_rbac_db.sql` membuat ulang `roles`, `permissions`, `role_permissions`, dan `user_permissions` (yang dibuang 000010 pada Opsi A), dan izin yang ditegakkan API dibaca dari basis data. Peta `rolePermissions` di [rbac.go](../../apps/api/internal/auth/usecase/rbac.go) tinggal matriks **bawaan** (seed dan pembanding "pulihkan bawaan"); katalog izin tetap didefinisikan di kode ([rbac_katalog.go](../../apps/api/internal/auth/usecase/rbac_katalog.go)). Butir 02-A di atas (konsistensi katalog, route yatim, `permissions.json`) tetap berlaku untuk katalog dan matriks bawaan.

1. **MIG** _(Opsi A)_
   - [x] Migration yang `DROP` `user_permissions`, `role_permissions`, `permissions`, `roles`. `Down` membuat ulang tabel kosong. Ini tidak menyentuh enum `peran`.
2. **SEED**
   - [x] Tidak ada seed RBAC (peta di kode). Catat di F1 bahwa `INSERT` roles/permissions (000001) dan `role_permissions` + `DELETE FROM role_permissions` (000006) dihapus.
3. **API**
   - [x] Perbaiki nama izin pada route yang yatim, mis. `lapangan.lihat`/`lapangan.baca` → izin yang memang dimiliki peran lapangan (`tugas.lihat_sendiri`, `dashboard.operasional`), `lapangan.validasi` → `harga.validasi`, `prediksi.baca` → `prediksi.lihat`. Tentukan bersama D4 (modul `analytics`).
   - [x] Satu berkas kontrak `packages/contracts/permissions.json`, dihasilkan dari peta Go (`go test ./internal/auth/usecase -run TestPermissionsContractFile -update-permissions`). Konstanta web tidak dihasilkan, tetapi dijaga sama oleh tes `permissions.test.ts`.
   - [x] **Tes konsistensi:** setiap izin dalam `RequirePermission(...)` ada di daftar; setiap izin di daftar dimiliki ≥ 1 peran; daftar web sama dengan daftar Go.
4. **FE**
   - [x] Matriks akses di `/pengguna` memakai `GET /v1/roles` dan `GET /v1/permissions` (BFF sudah ada); pastikan sama dengan `lib/rbac`.
5. **TES**
   - [x] Integrasi: tiap peran → rute yang boleh/tidak boleh, dibandingkan dengan matriks di [ARCHITECTURE.md](../ARCHITECTURE.md).
6. **DoD** — checklist [00 §4](00-cara-kerja.md). Khusus: `GET /v1/lapangan/tugas/saya` tidak lagi 403 untuk peran lapangan.

---

## 02-B — Pemisahan tugas dan cakupan baris _(Fase F)_

**Tujuan:** aturan RBAC yang dijanjikan dokumen ditegakkan di backend, bukan hanya UI.

- [ ] **Row scope:** Koordinator dan Agregator hanya kabupatennya, Mahasiswa hanya entrinya sendiri. Terapkan di `harga`, `lapangan`, `dashboard`, `ews` (kolom `pengguna.kabupaten_kode` sudah ada). Dikerjakan bersama slice tiap modul.
- [ ] **Pemisahan tugas:** validator tidak boleh memvalidasi entrinya sendiri; penginput ≠ validator ≠ publisher; Admin Sistem tanpa hak persetujuan bisnis (kaitan: CHECK di 06-B).
- [ ] **Persetujuan berjenjang** intervensi ditegakkan di `UpdateTahap` (kaitan: 14-B).
- [ ] Tes integrasi RBAC per peran untuk tiap aturan di atas.

---

## 02-C — Izin langsung per pengguna dan matriks yang dapat diedit _(Opsi B, berlaku)_

**Tujuan:** matriks peran × izin dapat diubah admin dari UI tanpa rilis, dan admin dapat memberi izin tambahan per pengguna dengan masa berlaku dan alasan, dengan perubahan yang berlaku cepat di API dan di web.
**Migration:** `000107_02c_rbac_db.sql`. **Seeder:** `auth.reference` (lapis `reference`).

1. **MIG**
   - [x] `roles` (kunci = enum `peran`, 14 baris; `label`, `lapis`, `urutan`, `versi` untuk kunci optimistis, `diperbarui_pada/oleh`), `permissions` (`nama` domain.aksi, `deskripsi`, `kelompok`; CHECK format), `role_permissions` (PK `(role_kunci, permission_nama)`, FK `ON DELETE CASCADE`), `user_permissions` (PK `(pengguna_id, permission_nama)`, `berlaku_sampai` NULL = tanpa batas, `alasan` NOT NULL dan tidak kosong, `diberikan_oleh` `ON DELETE SET NULL`, `diberikan_pada`; FK pengguna `ON DELETE CASCADE`). Pencarian per request memakai awalan PK keduanya. Down menjatuhkan keempatnya. Model schematest ditambahkan.
2. **SEED** (lapis `reference`, aman di produksi)
   - [x] [`SeedRBAC`](../../apps/api/internal/auth/seeder/rbac.go): 14 peran, 35 izin, dan matriks bawaan, **tanpa `DELETE`**. `roles` dan `permissions` adalah katalog di kode (label, lapis, deskripsi, kelompok disegarkan dari kode; `versi` dan `diperbarui_*` milik admin tidak disentuh). `role_permissions` hanya diisi untuk peran yang **baru** dibuat dan izin yang **baru** masuk katalog (`ON CONFLICT DO NOTHING`), jadi izin yang dicabut atau ditambah admin tidak dikembalikan atau dihapus oleh seed ulang, sedangkan izin baru di rilis berikutnya otomatis menempel ke peran bawaannya satu kali. `SeedDemo` memanggilnya juga (akun demo tidak berguna tanpa matriks), sehingga tes modul lain yang hanya memanggil `SeedDemo` tetap jalan. **Stack yang sudah berjalan: jalankan `seed reference` sesudah migrasi 000107** (tabel baru kosong; tanpa seed semua pengguna mendapat izin kosong, fail-closed).
3. **API** (semua `pengguna.kelola`, tercatat di audit dengan sebelum/sesudah, OpenAPI disinkronkan)
   - [x] `GET /v1/roles` (diperluas: `label`, `lapis`, `permissions` dari DB, `bawaan`, `versi`) dan `GET /v1/permissions` (diperluas: `deskripsi`, `kelompok`); perluasan hanya menambah kolom.
   - [x] `PUT /v1/roles/{id}/permissions` `{permissions, versi}`: mengganti seluruh izin satu peran dalam satu transaksi. `409 ROLE_VERSION_CONFLICT` (data berisi peran saat ini) bila `versi` tertinggal; `400 UNKNOWN_PERMISSION` bila nama di luar katalog; himpunan yang sama tidak menulis apa pun (versi tetap, tanpa audit). Audit `UPDATE_ROLE_PERMISSIONS`.
   - [x] `GET/POST /v1/users/{id}/permissions`, `DELETE /v1/users/{id}/permissions/{izin}`: daftar (izin peran, langsung aktif dan kedaluwarsa, efektif), beri (`izin`, `berlaku_sampai` opsional di masa depan, `alasan` wajib maks. 500 karakter; `diberikan_oleh` dari token), cabut. Audit `GRANT_USER_PERMISSION` / `REVOKE_USER_PERMISSION`.
   - [x] **Keputusan: tidak idempoten, ditolak.** Memberi izin yang sudah dimiliki lewat peran: `409 PERMISSION_ALREADY_VIA_ROLE` (pemberian itu tak berarti dan akan menjadi izin "hantu" bila peran berubah). Izin langsung yang masih berlaku: `409 DIRECT_PERMISSION_EXISTS` (cabut dulu bila ingin mengubah masa berlaku atau alasan, supaya audit mencatat kedua langkah). Baris yang sudah kedaluwarsa diganti oleh pemberian baru (audit memuat baris yang digantikan). Pengguna tak ada: 404; masa berlaku lampau, alasan kosong: 400.
   - [x] **Penjaga pemegang terakhir `pengguna.kelola`** (`409 LAST_PERMISSION_HOLDER`): sistem tidak boleh kehilangan pengguna **aktif** yang memegang `pengguna.kelola` lewat **perannya** (izin langsung tidak dihitung: bisa kedaluwarsa atau dicabut tanpa melewati penjaga). Berlaku untuk edit matriks, `PUT /users/:id/roles`, `PATCH /users/:id` (peran/status), `DELETE /users/:id`, dan aksi massal `nonaktifkan` (ditolak seluruhnya). Ditegakkan di lapisan repositori di dalam transaksi: kunci penasihat `pg_advisory_xact_lock` menyerialkan perubahan (dua admin yang serentak menurunkan dua pemegang terakhir: tepat satu lolos), hitungan sebelum dan sesudah dibandingkan, dan basis data tanpa pemegang sama sekali tidak menghalangi perubahan lain.
4. **Penegakan dan batas waktu** (inti 02-C)
   - **Sumber izin efektif** = `role_permissions` peran akun saat ini ∪ `user_permissions` yang `berlaku_sampai` NULL atau di masa depan. Akun tak ada/tidak `AKTIF` tidak punya izin efektif (401).
   - **API menegakkan dari basis data, bukan dari klaim token.** `token.Manager.SetPermissionResolver` dipasang oleh `internal/auth/routes.Register` pada `Manager` yang dibagi semua modul (cara yang sama dengan `SetRevocationChecker`, 01-E), jadi tanda tangan `JWTAuth(tokenMgr)` tidak berubah dan setiap router yang memasang modul auth (cmd/api, harness smoketest, tes integrasi modul) otomatis menegakkannya; router tanpa modul auth memakai klaim token seperti dulu. `JWTAuth` mengisi `permissions` di konteks dari resolver.
   - **Cache dan batas keterlambatan.** Resolver menyimpan izin efektif per pengguna di memori proses, umur maksimum `IzinCacheTTL` (**15 detik**, `usecase.DefaultIzinCacheTTL`; 0 = bawaan, negatif = tanpa cache). Batas: (a) perubahan lewat proses API ini (edit matriks, beri/cabut izin langsung, ganti peran, nonaktif, hapus, aksi massal) membuang cache terkait seketika, jadi berlaku pada request berikutnya; (b) perubahan dari proses lain (replika lain, SQL langsung) berlaku paling lambat 15 detik kemudian; (c) izin langsung yang **kedaluwarsa berhenti tepat waktu**: entri cache tidak hidup melewati `berlaku_sampai` terdekat (sisa waktu dihitung jam basis data, bukan jam aplikasi); (d) pembacaan yang dimulai sebelum pembuangan tidak menyimpan hasil basinya (generasi). **Gagal-tutup:** bila izin tak dapat dipastikan (galat basis data) request dijawab **503**, entri basi tidak disajikan. Koneksi SSE EWS yang sudah terbuka diperiksa saat dibuka dan berakhir paling lambat saat access token kedaluwarsa (`TokenLifetimeContext`, 01-E).
   - **Token tetap membawa gabungan izin saat terbit** (untuk UI web), dihitung lewat cache yang sama dengan `JWTAuth` supaya token baru tidak langsung dianggap basi.
   - **Web menyusul cepat tanpa versi izin terpisah:** `JWTAuth` membandingkan himpunan `permissions` di token dengan izin efektif; bila berbeda ia menjawab **`401 PERMISSIONS_CHANGED`** ("Izin akun berubah, sesi perlu disegarkan"; pesan sengaja tidak memuat kata sandi/password agar klien tidak mengira salah kredensial). Refresh senyap 01-E menukar refresh token dan menerbitkan token dengan izin segar secara transparan, lalu permintaan diulang. Dipilih dibanding "versi izin di token" karena tanpa kolom/penghitung tambahan dan otomatis benar untuk kedaluwarsa. Kompromi: dengan banyak replika yang cache-nya belum seragam, token segar dari satu replika bisa ditolak replika lain paling lama satu TTL (sekali refresh tambahan; tidak pernah salah izin). Token lama yang terbit sebelum fitur ini sama dengan izin bawaan, jadi tidak ditolak.
   - **Batas total yang berlaku:** izin yang dicabut/kedaluwarsa/dihapus dari matriks berhenti bekerja di API dalam **≤ 15 detik** (seketika bila perubahan lewat proses yang sama), bukan 15 menit; UI web menyusul pada panggilan API berikutnya (ditambah polling data yang sudah ada) atau navigasi berikutnya.
5. **FE**
   - [x] Sesi web membawa klaim `permissions` (`SessionUser.permissions`). `can(subjek, izin)` menerima peran (matriks bawaan; mode mock/dev) **atau** pengguna sesi (izin efektif token; klaim kosong `[]` berarti benar-benar tanpa izin). Beralih ke izin efektif sesi: `useCan`/`Can`, `canAccessRoute`, sidebar, palet perintah, topbar, shell lapangan, kanban dan panel persetujuan intervensi, aliran EWS, bar aksi massal, dan halaman server `/operasional`. Penjaga edge sudah memakai klaim.
   - [x] Refresh senyap yang **mengubah** izin (`izin_berubah` pada `POST /api/auth/refresh`) membuat `PenyegarIzin` (di `Providers`) memanggil `router.refresh()`; tab lain disusulkan lewat peristiwa `storage`. Navigasi halaman yang ditolak edge menukar refresh token **sekali** sebelum `/akses-ditolak` (penanda `garda_pulih` 20 detik memutus loop), jadi halaman yang baru diizinkan terbuka tanpa masuk ulang.
   - [x] `/pengguna` › Matriks akses (komponen baru [matriks-akses.tsx](../../apps/web/src/components/pengguna/matriks-akses.tsx)): sel dapat diklik menjadi **draf** (cincin kuning, penghitung "N peran belum disimpan", batalkan per peran/semua, simpan per peran dengan `versi`), titik kuning pada izin yang berbeda dari bawaan dan tombol "Pulihkan bawaan", draf bertahan saat pindah tab dan memperingatkan sebelum halaman ditutup, konflik versi membuang draf dan memuat ulang, penolakan pemegang terakhir mempertahankan draf. Tampilan baca-saja lama tetap untuk mode mock/tanpa izin.
   - [x] Dialog Ubah pengguna › tab **Izin langsung** ([izin-langsung.tsx](../../apps/web/src/components/pengguna/izin-langsung.tsx)): daftar (sumber "Langsung", status, masa berlaku, alasan, pemberi dan waktu), izin dari peran terpisah, formulir pemberian (`useFormBff` + `FormBff` sesuai [FORM.md](../FORM.md); pemilih izin hanya yang belum dimiliki, tanggal berlaku opsional sampai akhir hari, alasan wajib) dan pencabutan. Berdiri di samping formulir akun (bukan di dalamnya: `<form>` tidak boleh bersarang); formulir akun tetap terpasang saat tab izin dibuka.
6. **TES**
   - [x] Go unit: cache (TTL, pembuangan, kedaluwarsa, generasi, gagal-tutup), `JWTAuth` dengan resolver (izin dari resolver, `PERMISSIONS_CHANGED`, 401/503), union di token, matriks (versi, audit, tanpa perubahan), izin langsung (validasi, penggantian yang kedaluwarsa, audit), penjaga pada perubahan akun, katalog. Integrasi (dockertest, `auth/integration/izin_test.go`): edit matriks berlaku seketika lalu token basi disegarkan, perubahan SQL berlaku ≤ TTL, izin langsung berlaku/kedaluwarsa tepat waktu/dicabut, akun nonaktif lewat SQL ditolak, fail-closed 503, 12 edit matriks serentak → satu pemenang, penjaga pemegang terakhir (termasuk balapan 8 putaran), 403/401 untuk semua endpoint baru, audit sebelum/sesudah, seed idempoten dan tidak menimpa perubahan admin (`seeder/rbac_test.go`).
   - [x] Web (node:test): `can` dengan klaim, `resolveSession` memetakan klaim, deteksi perubahan izin token, store draf, pemetaan dan helper matriks/masa berlaku, penjaga edge. E2E [rbac.spec.ts](../../apps/web/e2e/rbac.spec.ts).

**Batasan yang diketahui:** (1) penerima notifikasi EWS dipilih dari matriks **bawaan** saat proses dimulai (`RolesWithPermission`); mengubah `ews.akui` di matriks belum mengubah penerima. (2) `GET /v1/auth/me` tidak memuat izin; web membacanya dari klaim token. (3) Baris izin langsung yang kedaluwarsa dibiarkan sebagai riwayat sampai dicabut (belum ada job pemangkas). (4) Peran tetap 14 nilai enum; matriks dapat diedit, peran tidak dapat ditambah dari UI.

---

## 02-D — Undang pengguna lewat email, impor dan ekspor CSV _(backlog lanjutan, selesai)_

**Tujuan:** admin tidak lagi mengarang dan menyampaikan kata sandi sementara; pemilik email menetapkan kata sandi pertamanya sendiri. Impor massal dari CSV dan ekspor yang tercatat di audit menggantikan ekspor sisi klien yang tak teraudit. Migration `000108_02d_undangan_pengguna.sql`.

### Model undangan

- **Tabel sendiri `undangan_pengguna`, bukan `password_reset_tokens` + kolom jenis.** Siklus hidupnya berbeda (72 jam vs 30 menit; dicabut saat kirim ulang; hanya untuk akun `MENUNGGU`), dan `ResetPassword`/`ConsumeResetToken` membaca tabel reset tanpa penyaring jenis: satu filter yang terlewat membuat token undangan bisa dipakai di jalur atur ulang. Dipisah, jalur atur ulang tidak mungkin mengaktifkan akun (diuji di integrasi).
- Kolom: `token_hash` (SHA-256 dari token acak 256-bit; token mentah hanya ada di email), `email` (alamat tujuan saat undangan dikirim), `diundang_oleh` (FK `ON DELETE SET NULL`, tidak menghalangi penghapusan admin), `expires_at`, `used_at`, `dicabut_pada`. Indeks unik parsial `uq_undangan_pengguna_aktif`: paling banyak satu undangan hidup per akun.
- **TTL 72 jam** (`usecase.Config.UndanganTTL`, bawaan `UndanganTTLBawaan`); email menyebut masa berlakunya.
- **Aktivasi atomik** (`repo.Aktifkan`, satu transaksi): `UPDATE undangan … SET used_at WHERE token_hash AND belum dipakai/dicabut/kedaluwarsa AND akun MENUNGGU AND lower(email akun) = lower(undangan.email) RETURNING`, lalu akun diisi kata sandi, `status = AKTIF`, `wajib_ganti_password = false`. Permintaan serentak: tepat satu berhasil (teruji unit dan integrasi). Karena syarat "email masih sama" ada di pernyataan itu, **mengubah email akun MENUNGGU otomatis menghanguskan tautan yang terlanjur dikirim ke alamat lama** (pemilik alamat lama tidak bisa mengambil alih akun); admin mengirim ulang ke alamat baru.
- **Urutan pemeriksaan:** token (ada, belum dipakai/dicabut, belum kedaluwarsa) → kebijakan kata sandi (yang sama dengan atur ulang, plus maksimal 72 karakter karena batas bcrypt) → transaksi aktivasi. Kata sandi lemah **tidak** menghanguskan tautan.
- **Tanpa auto-login:** aktivasi hanya menetapkan kata sandi; halaman menautkan ke `/masuk`. Alasannya: lockout, MFA (peran wajib MFA), dan penerbitan sesi tinggal di satu tempat (`Login`), dan pengguna langsung membuktikan kata sandinya bekerja.

### Status `MENUNGGU` dan anti-enumerasi

- Akun `MENUNGGU` tidak punya kata sandi. Di `Login` ia lewat cabang "tanpa kata sandi" yang sudah ada: bcrypt palsu, `401 INVALID_CREDENTIALS`, dan hitungan lockout per email yang **sama persis** dengan email tak terdaftar (teruji: urutan jawaban 401 ×5 lalu 423 identik, dan badan jawaban identik). `ForgotPassword` tidak mengirim apa pun untuknya (status bukan `AKTIF`). Tidak ada jawaban publik yang membedakan akun menunggu dari yang tidak ada. Membedakan "kedaluwarsa" dari "tidak valid" di `/v1/auth/aktivasi` hanya memberi tahu pemegang token yang sah (256 bit, tidak bisa ditebak).
- **Status hanya dikelola alur undangan:** `PATCH /v1/users/:id` menolak mengubah status akun menunggu (409) dan menolak memasang `MENUNGGU` pada akun lain (400); `reset-password` oleh admin untuk akun menunggu ditolak (409, gunakan kirim ulang undangan); aksi massal `aktifkan`/`nonaktifkan` melewatinya. Membatalkan undangan = menghapus akun (undangannya ikut terhapus lewat `ON DELETE CASCADE`).
- SSO Google tetap menolak akun yang bukan `AKTIF` (jadi `MENUNGGU` ditolak); aktivasi tidak dilakukan lewat SSO.

### API

| Endpoint | Izin | Isi |
|---|---|---|
| `POST /v1/users/undang` | `pengguna.kelola` | Buat akun `MENUNGGU` + undangan dalam satu transaksi, kirim email. `id` dibuat server (`USR-` + 16 heksadesimal), email dirapikan ke huruf kecil. 201 `{user, berlaku_sampai, email_terkirim}`; 400 (semua galat isian digabung), 409 email terdaftar. |
| `POST /v1/users/:id/undang-ulang` | `pengguna.kelola` | Cabut semua undangan hidup, buat yang baru (kunci baris akun), kirim ke email saat ini. 404 / 409 (bukan `MENUNGGU`). |
| `POST /v1/auth/aktivasi` | publik, batas laju per IP satu ember dengan login | `{token, password}`; 200 / 400 (kebijakan) / 401 (tidak valid, terpakai, dicabut, kedaluwarsa) / 429. |
| `POST /v1/users/impor` | `pengguna.kelola` | Lihat di bawah. |
| `GET /v1/users/ekspor` | `pengguna.kelola` | Lihat di bawah. |

- **Email.** `pkg/email` (`AktivasiLink`, `SendUndangan`; nama dari admin di-escape HTML). Dikirim **sinkron dengan batas 10 detik** untuk undangan tunggal dan kirim ulang (admin langsung tahu bila gagal: `email_terkirim: false`, akun dan undangan tetap ada). `SMTP_HOST` kosong = `email_terkirim: false`. Karena SMTP bisa tidak tersedia, "buat dengan kata sandi sementara" (`POST /v1/users`) **tetap ada sebagai opsi kedua** di dialog undangan (untuk lingkungan tanpa email atau pengguna tanpa akses email); `/pengguna` memakai undangan sebagai cara utama.
- **Pembersihan.** Langkah `undangan_pengguna` di job `auth_pembersihan` menghapus undangan yang kedaluwarsa, terpakai, atau dicabut lebih dari `RetensiResetToken` (bawaan 7 hari) lalu; undangan hidup tidak pernah dihapus. Akun `MENUNGGU` yang undangannya habis tetap ada (kirim ulang).

### Impor CSV

- Server yang mengurai dan memeriksa (`usecase/impor_pengguna.go`); klien hanya mengirim teks (`{csv, dry_run}`, `dry_run` wajib eksplisit). Header `nama,email,peran,lembaga,kabupaten_kode` (`kabupaten_kode` boleh tanpa kolom; urutan bebas; kolom lain ditolak supaya salah ketik tidak menghilangkan data), UTF-8 dengan atau tanpa BOM, koma atau titik koma (dari header), CRLF/LF, sel berkutip boleh memuat pemisah dan baris baru. Berkas bukan UTF-8 (termasuk karakter pengganti U+FFFD dan byte NUL) ditolak dengan petunjuk "CSV UTF-8". Batas **500 baris data dan 512 KB** (baris kosong tidak dihitung); lebih dari itu ditolak seluruhnya (400 / 413).
- Per baris: nama/lembaga wajib dan ≤ 255, email valid (bentuk sempit yang aman untuk header email dan CSV), peran dikenal, kabupaten 4 digit dan ada di master wilayah, email unik dalam berkas (baris pertama tetap sah, salinannya "Email sama dengan baris N") dan terhadap basis data (tak peka huruf besar-kecil). Semua galat satu baris dilaporkan sekaligus; nomor baris = nomor di spreadsheet (header 1).
- **Transaksi:** mode komit memeriksa ulang dengan data terkini, lalu menulis **semua baris sah dalam satu transaksi** (akun + undangan; semua atau tidak sama sekali). Baris tidak sah tidak pernah dibuat dan dilaporkan per baris tanpa menggagalkan yang lain. Bila email sebuah baris keburu didaftarkan pihak lain di antara pemeriksaan dan penulisan, seluruh penulisan dibatalkan (409, teruji dengan komit serentak: tepat satu menang, tak ada akun setengah jadi).
- **Email massal** dikirim sesudah komit **di latar belakang**, paling banyak 4 serempak, masing-masing dibatasi 10 detik (ratusan email serempak lewat SMTP akan menahan permintaan berpuluh detik). Kegagalan tercatat di log dan audit (`KIRIM_UNDANGAN`, `GAGAL`); akunnya tetap ada dan admin memakai "Kirim ulang undangan". Kompromi yang disadari: proses yang mati di tengah pengiriman kehilangan sisa email (akun tetap `MENUNGGU`, kirim ulang); `UseCase.TungguPengirimanEmail` tersedia untuk penutupan layanan dan tes.
- **Injeksi rumus:** nama, email, dan lembaga yang diawali `=`, `+`, `-`, atau `@` ditolak di impor.
- **Audit:** `UNDANG_PENGGUNA` per akun (`metadata.sumber = impor`, nomor baris) dan satu `IMPOR_PENGGUNA` (`total`, `dibuat`, `ditolak`). Mode uji tidak menulis dan tidak diaudit.

### Ekspor CSV

- Menggantikan ekspor sisi klien (yang membangun CSV dari baris tabel tanpa audit dan tanpa penetral rumus). Sumber tunggal filter: `terapkanFilterPengguna` ([filter_pengguna.go](../../apps/api/internal/auth/repository/postgres/filter_pengguna.go)) dipakai bersama `GET /v1/users` dan ekspor; parameter di `dto.EksporPenggunaRequest` (`search`, `ids`). **Menambah kriteria daftar (peran, status, dst.) berarti menambahkannya di dua tempat itu.**
- Kolom daftar putih tetap `id,nama,email,peran,lapis,lembaga,kabupaten_kode,status,mfa_aktif,terakhir_masuk,dibuat_pada`; tidak pernah kata sandi, rahasia MFA, kode cadangan, atau token (teruji). Titik koma + BOM UTF-8 (seperti ekspor audit); sel diawali `= + - @ tab CR` diberi awalan apostrof (sama dengan ekspor audit); sel berisi pemisah/kutip/baris baru dikutip.
- **Audit sebelum serah:** `EKSPOR_DATA` (`entitas = pengguna`, `entitas_id = ekspor`, metadata `baris`, `terpotong`, `search`, `jumlah_id`) ditulis sebelum berkas dikirim; bila audit gagal, jawabannya 500 tanpa data (teruji dengan tabel audit yang dirusak). Maksimal 20.000 baris (`X-Export-Truncated`), `ids` maksimal 500; `ids` yang hanya berisi pemisah ditolak (bukan berubah menjadi semua pengguna). Header `Cache-Control: no-store`.
- Web: unduhan lewat BFF `/api/pengguna/ekspor` (berkas diteruskan apa adanya, sama dengan pola `unduhEksporAudit`). Toolbar punya "Ekspor semua (CSV)"; aksi massal "Ekspor CSV" mengirim id terpilih ke server.

### Web

- **Halaman publik `/aktivasi`** (`app/(auth)/aktivasi`): form kata sandi baru + konfirmasi dengan aturan kebijakan langsung (`useFormBff` + `FormBff`), pesan galat tautan hangus mengarahkan ke admin. `/aktivasi` dan `/api/auth/aktivasi` masuk `PUBLIC_ROUTES` edge guard dan daftar tanpa-pemulihan sesi. BFF `POST /api/auth/aktivasi` dibatasi 5/menit/IP (jendela yang sama dengan login) selain batas Go.
- **`/pengguna`:** toolbar "Undang pengguna" (dialog nama, email, peran, lembaga, kabupaten/kota bila peran berlingkup wilayah), "Impor CSV" (pilih berkas → pratinjau dengan galat per baris → "Impor N pengguna valid" → ringkasan; templat CSV berisi header saja agar templat yang tak diisi tidak membuat akun contoh), "Ekspor semua (CSV)". Kolom status menampilkan "Menunggu aktivasi"; menu baris akun menunggu menggantikan "Reset kata sandi" dengan "Kirim ulang undangan". Dialog undangan menawarkan "Buat dengan kata sandi sementara" (dialog lama). Logika baru di berkas baru (`aksi-undangan.tsx`, `undang-dialog.tsx`, `impor-csv-dialog.tsx`, `lib/pengguna/*`, `lib/api/pengguna-undangan.ts`, `lib/query/undangan-hooks.ts`); `pengguna-page.tsx` hanya disambungkan.

### Tes

- **Go unit** (`usecase/undangan_test.go`, `impor_pengguna_test.go`, `ekspor_pengguna_test.go`, `pkg/email/undangan_test.go`): undang (akun MENUNGGU tanpa kata sandi, hanya hash token, email terkirim ke SMTP palsu `pkg/email/emailtest`, audit tanpa token), semua kelas galat isian, email kembar, SMTP gagal/mati/macet, kirim ulang mencabut tautan lama, aktivasi (sekali pakai, serentak, kedaluwarsa tepat di batas, kata sandi lemah tidak menghabiskan tautan, akun/ email berubah, galat penyimpanan), login/lupa kata sandi akun menunggu identik dengan email tak terdaftar, PATCH/reset; impor (BOM, titik koma, CRLF, sel berkutip, nomor baris, semua galat berkas dan baris, 500/501 baris, batas ukuran, dry run tanpa tulis, komit, bentrok, konkurensi email); ekspor (format, tanpa rahasia, injeksi rumus, filter, terpotong, audit gagal).
- **Go integrasi** (dockertest: `integration/undangan_test.go`, `impor_ekspor_test.go`, `undangan_pembersihan_test.go`): alur penuh via HTTP, 403/401, validasi/409, kirim ulang, kedaluwarsa, serentak, ganti email, jalur lain untuk akun menunggu, batas laju, penulisan atomik, komit serentak, ekspor (isi, header, filter sama dengan daftar, tanpa rahasia, injeksi, audit gagal), pembersihan tepat di batas.
- **Web (node:test):** skema/templat/pemetaan (`lib/pengguna/*.test.ts`), klien API dan unduhan, route BFF (`pengguna-undangan-route.test.ts`, `aktivasi-route.test.ts`: sesi, CSRF, validasi, penerusan tanpa isian liar, header unduhan, batas laju), skema/galat aktivasi, daftar rute publik.
- **E2E** [undangan.spec.ts](../../apps/web/e2e/undangan.spec.ts): undang dari UI → email Mailpit → `/aktivasi` → masuk form → beranda; tautan bekas ditolak; akun menunggu belum bisa masuk; kirim ulang (email baru, tautan lama mati); impor CSV (satu sah dua salah → pratinjau → konfirmasi → "Menunggu aktivasi"); ekspor semua dan terpilih, tercatat di audit. Pemakaian anggaran BFF: aktivasi 3 + masuk 1.
- Catatan: stack demo harus dibangun ulang (`make docker/demo/up`) untuk migration 000108 dan kode baru; email undangan membutuhkan `APP_PUBLIC_URL` dan SMTP (Mailpit di demo).

---

## Backlog lanjutan

- [x] Buka-kunci akun dari UI (lihat [01-F](01-auth.md)); undang pengguna lewat email (set password pertama kali); impor/ekspor CSV pengguna. _(selesai: lihat [02-D](#02-d--undang-pengguna-lewat-email-impor-dan-ekspor-csv))_ Ekspor memakai filter daftar yang sama (pencarian, peran, status, lapis, MFA, kunci login) dan validasi yang sama (400 untuk nilai asing).
- [x] Filter, sorting, dan paginasi sisi server untuk daftar pengguna. Lihat "Daftar pengguna sisi server" di bagian "Yang sudah ada".
- [x] Tampilkan status MFA per pengguna dan tombol reset MFA oleh admin. Lihat "Reset MFA oleh admin" di bagian "Yang sudah ada".
- [x] Riwayat perubahan peran per pengguna (terhubung ke [03](03-audit.md)). Lihat "Riwayat peran" di bagian "Yang sudah ada".
- [ ] Naikkan coverage handler/usecase pengguna; tes komponen `pengguna-page`.
