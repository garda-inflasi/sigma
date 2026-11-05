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
- [ ] Filter, sorting, dan paginasi sisi server untuk daftar pengguna (sekarang `list({ search })`).
- [ ] Tampilkan status MFA per pengguna dan tombol reset MFA oleh admin.
- [ ] Riwayat perubahan peran per pengguna (terhubung ke [03](03-audit.md)).
- [ ] Naikkan coverage handler/usecase pengguna; tes komponen `pengguna-page`.
