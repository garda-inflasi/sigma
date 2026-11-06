# 02 — Pengguna, peran, dan RBAC

**Prioritas:** P0 · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes 🟡
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19 §1.4](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [pengguna-page.tsx](../../apps/web/src/components/pengguna/pengguna-page.tsx) (daftar akun, aksi massal, matriks peran × izin) di `/pengguna`.
- **BFF:** `app/api/pengguna` (list, create, `[id]`, `[id]/roles`, `[id]/reset-password`, `bulk`) dan `app/api/rbac/{roles,permissions}`.
- **API Go:** `POST/GET /v1/users`, `GET/PATCH/DELETE /v1/users/:id`, `POST /v1/users/bulk`, `POST /v1/users/:id/reset-password`, `PUT /v1/users/:id/roles`, `GET /v1/roles`, `GET /v1/permissions` (semua butuh `pengguna.kelola`).
- **Aturan di web:** 14 peran dan 35 izin di [roles.ts](../../apps/web/src/lib/rbac/roles.ts) / [permissions.ts](../../apps/web/src/lib/rbac/permissions.ts), dipakai sidebar, palet perintah, middleware, aksi massal.
- **Temuan penting (terverifikasi dari kode):**
  - Izin yang berlaku di backend berasal dari peta `rolePermissions` di [auth/usecase/usecase.go](../../apps/api/internal/auth/usecase/usecase.go), dikunci pada enum `peran` (14 nilai) dengan 35 izin `domain.aksi`.
  - Tabel `roles` (15 baris), `permissions` (29 baris format `harga:read`), `role_permissions`, `user_permissions` diisi migrasi 000001/000006 tetapi **tidak dibaca kode apa pun**. Nama izinnya tidak ada yang sama dengan yang dipakai kode; `roles` berisi "Superadmin" yang tidak ada di enum `peran`.
  - Route yang meminta `lapangan.lihat`, `lapangan.baca`, `lapangan.validasi`, `prediksi.baca` **tidak dimiliki peran manapun**, jadi selalu 403 (termasuk semua `/v1/lapangan/*`).
- **Detail pengguna (✅, terhubung):** dialog "Ubah pengguna" memuat `GET /v1/users/:id` lewat BFF (`penggunaApi.get`, hook `usePenggunaDetail`, tanpa cache agar formulir selalu segar) sebelum formulir tampil, dan menampilkan rincian akun (ID, cakupan, status, MFA, terakhir masuk, dibuat). Bila gagal dimuat, formulir memakai baris daftar. Perbaikan backend: `GetUser` tidak lagi menjawab 404 untuk galat basis data (hanya "tidak ada" yang 404, selain itu 500); tes usecase dan integrasi `get_user_test.go` (200 tanpa rahasia, 404, 403 tanpa `pengguna.kelola`, 401).

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

## 02-C — Izin langsung per pengguna _(opsional; hanya bila Opsi B dipilih)_

1. **MIG**
   - [ ] Buat ulang `roles` (kunci = nilai enum `peran`, 14 baris), `permissions` (35 izin `domain.aksi`, `deskripsi`, `kelompok`), `role_permissions`, `user_permissions` (+ `berlaku_sampai`, `alasan`, `diberikan_oleh`, `diberikan_pada`).
2. **SEED** (lapis `reference`)
   - [ ] 14 peran, 35 izin, dan `role_permissions` sesuai matriks; `ON CONFLICT DO NOTHING` agar tidak menimpa perubahan admin. **Jangan** ada `DELETE`.
3. **API**
   - [ ] Token membawa gabungan izin peran + izin langsung, dimuat dari DB dengan cache; UI matriks bisa mengubah.
4. **FE**: UI penugasan izin langsung dengan masa berlaku dan alasan.
5. **TES**: izin langsung berlaku dan kedaluwarsa; perubahan tercatat di audit.

---

## Backlog lanjutan

- [ ] Buka-kunci akun dari UI; undang pengguna lewat email (set password pertama kali); impor/ekspor CSV pengguna.
- [x] Filter, sorting, dan paginasi sisi server untuk daftar pengguna. Lihat "Daftar pengguna sisi server" di bawah.
- [x] Tampilkan status MFA per pengguna dan tombol reset MFA oleh admin. Lihat "Reset MFA oleh admin".
- [x] Riwayat perubahan peran per pengguna (terhubung ke [03](03-audit.md)). Lihat "Riwayat peran".
- [ ] Naikkan coverage handler/usecase pengguna; tes komponen `pengguna-page`.
