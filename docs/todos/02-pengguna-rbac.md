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

## Backlog lanjutan

- [ ] Buka-kunci akun dari UI; undang pengguna lewat email (set password pertama kali); impor/ekspor CSV pengguna.
- [ ] Filter, sorting, dan paginasi sisi server untuk daftar pengguna (sekarang `list({ search })`).
- [ ] Tampilkan status MFA per pengguna dan tombol reset MFA oleh admin.
- [ ] Riwayat perubahan peran per pengguna (terhubung ke [03](03-audit.md)).
- [ ] Naikkan coverage handler/usecase pengguna; tes komponen `pengguna-page`.
