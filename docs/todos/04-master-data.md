# 04 — Master data (komoditas, wilayah, pasar)

**Prioritas:** P0 (PRD §1.2) · **Status:** UI ✅ · Backend 🟡 · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [master-page.tsx](../../apps/web/src/components/master/master-page.tsx) di `/master/[entitas]` (komoditas, wilayah, pasar), versioning dan persetujuan tampil di UI.
- **API Go:** `GET /v1/master/{komoditas,wilayah,pasar}` (izin `master.lihat`) dan `POST /v1/master/komoditas/bulk` (izin `master.ajukan`). Ada `usecase_test.go`.
- **Skema:** `provinsi`, `kabupaten`, `pasar` (+ `radius_meter` untuk geofence, `aktif`), `komoditas` (+ `versi`, `status`, `pilot`, `shelf_life_hari`) **cocok** dengan entity. Tabel `perubahan_master` (`versi`, `perubahan` JSONB, `status`, `diajukan_oleh_id`, `disetujui_oleh_id`) sudah ada tetapi **tidak dirujuk kode Go**.
- **Data referensi sudah ada, tetapi di dalam migrasi 000006** (provinsi Aceh, 23 kabupaten/kota, pasar utama, komoditas utama; `ON CONFLICT DO NOTHING`).
- **Halaman membaca dari API** (`api.master.{komoditas,wilayah,pasar}`). Aksi ajukan/setujui/arsipkan masih simulasi lokal di halaman; penyimpanannya adalah 04-B.

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
   - [ ] Hubungkan entity ke `perubahan_master`; endpoint ajukan/setujui/tolak; izin `master.setujui` (ada di web, belum dipakai di route).
   - [ ] Bulk/ajukan untuk **wilayah** dan **pasar** (UI menampilkannya; API baru punya komoditas).
   - [ ] Validasi referensial: komoditas atau pasar yang sudah punya data harga hanya dinonaktifkan, tidak dihapus.
4. **FE**: dialog ajukan perubahan, antrean persetujuan, riwayat versi dan diff.
5. **TES**: alur ajukan → setujui, termasuk penolakan oleh peran tanpa izin; audit tercatat.

---

## 04-C — Pelengkap master *(Fase F, prioritas rendah)*

- [ ] `satuan`, `pedagang` (sekarang `entri_harga.kode_pedagang` string bebas), dan keputusan `kualitas` (enum atau tabel).
- [ ] `komoditas`: `berlaku_dari`/`berlaku_sampai` atau tabel riwayat versi agar harga lama merujuk definisi yang berlaku saat itu.
- [ ] Paginasi dan pencarian sisi server untuk data besar (kecamatan se-Aceh).
