# 04 — Master data (komoditas, wilayah, pasar)

**Prioritas:** P0 (PRD §1.2) · **Status:** UI ✅ · Backend 🟡 · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [master-page.tsx](../../apps/web/src/components/master/master-page.tsx) di `/master/[entitas]` (komoditas, wilayah, pasar), versioning dan persetujuan tampil di UI.
- **API Go:** `GET /v1/master/{komoditas,wilayah,pasar}` (izin `master.lihat`) dan `POST /v1/master/komoditas/bulk` (rute: `master.ajukan` atau `master.setujui`; izin per aksi di handler, lihat 04-B). Tes: unit usecase dan integrasi (dockertest).
- **Skema:** `provinsi`, `kabupaten`, `pasar` (+ `radius_meter` untuk geofence, `aktif`), `komoditas` (+ `versi`, `status`, `pilot`, `shelf_life_hari`) **cocok** dengan entity. Tabel `perubahan_master` (`versi`, `perubahan` JSONB, `status`, `diajukan_oleh_id`, `disetujui_oleh_id`) sudah ada tetapi **tidak dirujuk kode Go**.
- **Data referensi sudah ada, tetapi di dalam migrasi 000006** (provinsi Aceh, 23 kabupaten/kota, pasar utama, komoditas utama; `ON CONFLICT DO NOTHING`).
- **Halaman membaca dari API** (`api.master.{komoditas,wilayah,pasar}`). Aksi ajukan/setujui/arsipkan komoditas memanggil `POST /v1/master/komoditas/bulk` lalu memuat ulang daftar (tidak ada lagi simulasi di state React); wilayah dan pasar hanya punya ekspor CSV.

---

## 04-A — Master dari API + seed referensi *(Fase P, urutan #4)*

**Tujuan:** halaman master tampil dari database; data referensi lahir dari seeder, bukan migrasi.
**Bergantung pada:** 03-A.

1. **MIG** *(kecil)*
   - [x] Constraint: `CHECK (lat BETWEEN -90 AND 90)`, `CHECK (lng BETWEEN -180 AND 180)`, `harga_acuan > 0`, `shelf_life_hari > 0`, `radius_meter > 0`.
2. **SEED** (lapis `reference`, upsert `DO UPDATE`)
   - [x] Pindahkan dari 000006: provinsi, 23 kabupaten/kota, pasar, komoditas. Isinya disamakan dengan `lib/data/reference.ts` (kode komoditas `BRS-MED`, `CBM-KRT`, …; id pasar `PSR-1171-01`, …), 3 komoditas pilot (beras medium, cabai merah keriting, bawang merah). Empat komoditas yang hanya ada di 000006 (tepung terigu, ikan kembung, jagung, kedelai) tidak dibawa karena tak dipakai di mana pun.
   - [x] Seeder cold-storage tidak lagi menyisipkan komoditas; tomat (`TMT`) dan kentang (`KTG`) ada di seed ini, dan `coldstorage.reference` bergantung pada `master.reference`.
   - [x] Kode wilayah diselaraskan dengan kode BPS (kode di 000006 salah untuk 1101–1118 dan 1173–1174). Koordinat tetap perkiraan (ibu kota/pasar) dan perlu diganti dengan data geospasial resmi.
3. **API**
   - [x] Pastikan bentuk respons `GET /v1/master/{komoditas,wilayah,pasar}` cocok dengan `MasterKomoditasRow` dkk. di `lib/data/types.ts` (tambah mapper bila perlu); dukung paginasi dan pencarian.
4. **FE**
   - [x] `master-page.tsx` memakai hook `useMasterKomoditas`, `useMasterWilayah`, `useMasterPasar` (tambah `api.master.wilayah` / `api.master.pasar` dan mock-nya di [endpoints.ts](../../apps/web/src/lib/api/endpoints.ts)).
   - [x] `lib/data/reference.ts` tetap dipakai komponen lain (ikon, form lapangan). Tambahkan **tes yang membandingkan kode komoditas dan kabupaten di `reference.ts` dengan seed**, agar keduanya tidak menyimpang.
5. **TES**: tes handler dan repo (saat ini hanya usecase); `GET` mengembalikan seed lengkap.
6. **DoD** — checklist [00 §4](00-cara-kerja.md). Setelah ini F1 boleh memindahkan sisa data dari 000006.

---

## 04-B — Versioning dan persetujuan perubahan *(Fase F)*

**Tujuan:** perubahan master data melewati ajukan → tinjau → setujui, dengan versi dan riwayat (PRD §1.2).

1. **MIG**
   - [ ] `perubahan_master`: tambah `alasan`, `alasan_penolakan`, `disetujui_pada`; index `(status)`. (Referensi ke entitas bersifat polimorfik; validasi di aplikasi.)
   - [ ] `kecamatan` (kode BPS 7 digit, `kabupaten_kode` FK) dan `pasar.kecamatan_kode`, sesuai hierarki PRD.
2. **SEED**: impor kecamatan se-Aceh dari BPS lewat skrip (catat sumber dan lisensi), bukan ditulis tangan.
3. **API**
   - [x] Aksi massal komoditas `POST /v1/master/komoditas/bulk` (`ajukan_perubahan`, `setujui`, `arsipkan`) dengan izin per aksi: `setujui` butuh `master.setujui` (STRATEGIS_BPS, STRATEGIS_FMIPA), `ajukan_perubahan`/`arsipkan` butuh `master.ajukan` (ADMIN_SISTEM, MANAJER_DATA, MANAJER_PROGRAM). Pengaju tidak dapat menyetujui usulannya sendiri. Transisi dinilai per kode: AKTIF → MENUNGGU_PERSETUJUAN (ajukan); MENUNGGU_PERSETUJUAN → AKTIF dan `versi` + 1 (setujui); AKTIF atau MENUNGGU_PERSETUJUAN → DIARSIPKAN (arsipkan, tanpa pemulihan). Kode tak dikenal atau transisi tak sah menjadi item gagal di `rincian` (`kode`, `berhasil`, `pesan`) tanpa menggagalkan item lain; pembaruan memakai `WHERE status IN (...)` sehingga balapan juga jadi item gagal. Audit `BULK_<aksi>` mencatat kode yang berubah dan yang gagal.
   - [ ] Hubungkan entity ke `perubahan_master` (riwayat versi, alasan, penolakan); endpoint tolak.
   - **Wilayah dan pasar: tidak ada endpoint bulk.** Tabel `kabupaten` tidak punya kolom `status`/`versi` dan `pasar` hanya punya `aktif` (boolean), jadi siklus ajukan/setujui/arsip tidak berlaku tanpa migrasi. Migrasi tidak ditambahkan di sini; aksi ajukan/setujui/arsipkan dihapus dari UI untuk kedua entitas itu (tinggal Ekspor CSV). Bila siklus persetujuan wilayah/pasar diperlukan, tambahkan kolom `status` (enum `status_master`) dan `versi` pada `kabupaten` dan `pasar` (atau pakai `perubahan_master`), lalu perluas `domain.TransisiUntuk` dan repo agar generik per tabel.
   - [ ] Validasi referensial: komoditas atau pasar yang sudah punya data harga hanya dinonaktifkan, tidak dihapus.
4. **FE**
   - [x] Aksi ajukan/setujui/arsipkan di `/master/komoditas` memanggil API (`api.master.bulkKomoditas`, hook `useBulkMasterKomoditas`), menampilkan kode yang ditolak, memuat ulang daftar; "Setujui" hanya untuk `master.setujui`, "Ajukan perubahan"/"Arsipkan" hanya untuk `master.ajukan`.
   - [ ] Dialog ajukan perubahan (isi perubahan), antrean persetujuan, riwayat versi dan diff.
5. **TES**
   - [x] Unit usecase (transisi per aksi, item gagal, kode ganda, balapan, audit) dan integrasi (izin per aksi untuk ADMIN_SISTEM, MANAJER_DATA, STRATEGIS_BPS, STRATEGIS_FMIPA, peran lihat-saja; ajukan → setujui menaikkan versi; arsipkan dari AKTIF dan dari MENUNGGU; transisi tak sah; audit).
   - [x] e2e `apps/web/e2e/master.spec.ts`: manajer.data mengajukan (bertahan setelah muat ulang, tanpa tombol Setujui), strategis.bps menyetujui (versi + 1), wilayah/pasar tanpa aksi status, 403/400/item gagal lewat API. Arsip tidak diuji di e2e (tidak ada pemulihan).

---

## 04-C — Pelengkap master *(Fase F, prioritas rendah)*

- [ ] `satuan`, `pedagang` (sekarang `entri_harga.kode_pedagang` string bebas), dan keputusan `kualitas` (enum atau tabel).
- [ ] `komoditas`: `berlaku_dari`/`berlaku_sampai` atau tabel riwayat versi agar harga lama merujuk definisi yang berlaku saat itu.
- [ ] Paginasi dan pencarian sisi server untuk data besar (kecamatan se-Aceh).
