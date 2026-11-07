# 04 — Master data (komoditas, wilayah, pasar)

**Prioritas:** P0 (PRD §1.2) · **Status:** UI ✅ · Backend 🟡 · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [master-page.tsx](../../apps/web/src/components/master/master-page.tsx) di `/master/[entitas]` (komoditas, wilayah, pasar), versioning dan persetujuan tampil di UI.
- **API Go:** `GET /v1/master/{komoditas,wilayah,pasar}` (izin `master.lihat`) dan `POST /v1/master/komoditas/bulk` (rute: `master.ajukan` atau `master.setujui`; izin per aksi di handler, lihat 04-B). Tes: unit usecase dan integrasi (dockertest).
- **Skema:** `provinsi`, `kabupaten`, `pasar` (+ `radius_meter` untuk geofence, `aktif`), `komoditas` (+ `versi`, `status`, `pilot`, `shelf_life_hari`) **cocok** dengan entity. Tabel `perubahan_master` (`versi`, `perubahan` JSONB, `status`, `diajukan_oleh_id`, `disetujui_oleh_id`) sudah ada tetapi **tidak dirujuk kode Go**; sejak 000111 PK-nya BIGINT identity (ID lama di `kode`), lihat [Kunci primer](#kunci-primer-master-000111).
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
   - [ ] `perubahan_master`: tambah `alasan`, `alasan_penolakan`, `disetujui_pada`. PK sudah `BIGINT` identity (000111) dan indeks antrean parsial `idx_perubahan_master_menunggu (dibuat_pada) WHERE status = 'MENUNGGU'` sudah ada, jadi `index (status)` tidak perlu lagi. FK `diajukan_oleh_id`, `disetujui_oleh_id` → `pengguna` `RESTRICT` (000110). (Referensi ke entitas bersifat polimorfik: `entitas` + `entitas_id` tanpa FK; validasi di aplikasi.)
   - [ ] `kecamatan` dengan PK **KODE**: `kode varchar(7) PRIMARY KEY` (kode BPS kecamatan, 7 digit, mis. `1171010`), `nama`, `kabupaten_kode varchar(4) NOT NULL REFERENCES kabupaten(kode) ON DELETE RESTRICT` dengan indeks `idx_kecamatan_kabupaten_kode`; lalu `pasar.kecamatan_kode varchar(7) NULL REFERENCES kecamatan(kode) ON DELETE RESTRICT` dengan indeks `idx_pasar_kecamatan_kode`, sesuai hierarki PRD. Daftarkan `kecamatan` sebagai `PKKode` di `kategoriPK` (`kunci_test.go`) dan tambahkan barisnya di matriks [SKEMA-DATABASE](../SKEMA-DATABASE.md).
2. **SEED**: impor kecamatan se-Aceh dari BPS lewat skrip (catat sumber dan lisensi), bukan ditulis tangan.
3. **API**
   - [x] Aksi massal komoditas `POST /v1/master/komoditas/bulk` (`ajukan_perubahan`, `setujui`, `arsipkan`) dengan izin per aksi: `setujui` butuh `master.setujui` (STRATEGIS_BPS, STRATEGIS_FMIPA), `ajukan_perubahan`/`arsipkan` butuh `master.ajukan` (ADMIN_SISTEM, MANAJER_DATA, MANAJER_PROGRAM). Pengaju tidak dapat menyetujui usulannya sendiri. Transisi dinilai per kode: AKTIF → MENUNGGU_PERSETUJUAN (ajukan); MENUNGGU_PERSETUJUAN → AKTIF dan `versi` + 1 (setujui); AKTIF atau MENUNGGU_PERSETUJUAN → DIARSIPKAN (arsipkan, tanpa pemulihan). Kode tak dikenal atau transisi tak sah menjadi item gagal di `rincian` (`kode`, `berhasil`, `pesan`) tanpa menggagalkan item lain; pembaruan memakai `WHERE status IN (...)` sehingga balapan juga jadi item gagal. Audit `BULK_<aksi>` mencatat kode yang berubah dan yang gagal.
   - [ ] Hubungkan entity ke `perubahan_master` (riwayat versi, alasan, penolakan); endpoint tolak. Entity `PerubahanMaster` memakai `ID idx.ID` (`gorm:"column:id;primaryKey;autoIncrement"`), `Kode *string`, `DiajukanOlehID string` (`type:uuid`), `DisetujuiOlehID *string` (`type:uuid`, NULL bila belum). Rute `:id` adalah **BIGINT auto-increment** (`idx.ParamID`, ID salah bentuk dijawab `404`; ID lama `PM-...` tersimpan di `kode` tetapi tidak pernah tampil di UI, jadi tidak ada alias): `GET /v1/master/perubahan` (antrean, `status=MENUNGGU` memakai indeks parsial), `GET /v1/master/perubahan/:id`, `POST /v1/master/perubahan/:id/setujui`, `POST /v1/master/perubahan/:id/tolak`. ID di JSON string desimal.
   - **Wilayah dan pasar: tidak ada endpoint bulk.** Tabel `kabupaten` tidak punya kolom `status`/`versi` dan `pasar` hanya punya `aktif` (boolean), jadi siklus ajukan/setujui/arsip tidak berlaku tanpa migrasi. Migrasi tidak ditambahkan di sini; aksi ajukan/setujui/arsipkan dihapus dari UI untuk kedua entitas itu (tinggal Ekspor CSV). Bila siklus persetujuan wilayah/pasar diperlukan, tambahkan kolom `status` (enum `status_master`) dan `versi` pada `kabupaten` dan `pasar` (atau pakai `perubahan_master`), lalu perluas `domain.TransisiUntuk` dan repo agar generik per tabel.
   - [ ] Validasi referensial: komoditas atau pasar yang sudah punya data harga hanya dinonaktifkan, tidak dihapus. Basis data sudah menegakkannya: FK dari `entri_harga`, `tugas_lapangan`, `batas_harga`, `pasar`, dan `kabupaten` ke tabel kode berlaku `ON DELETE RESTRICT`, jadi `DELETE` master yang masih dirujuk gagal dengan `23503`; endpoint hapus (bila ada) menjawab `409` dengan saran menonaktifkan (`aktif = false` untuk pasar, status `DIARSIPKAN` untuk komoditas). Rute detail master memakai `:kode` (bukan `:id`), mis. `/v1/master/komoditas/:kode`, `/v1/master/wilayah/:kode`, `/v1/master/pasar/:kode` (nilainya kolom `pasar.id`, mis. `PSR-1171-01`), `/v1/master/kecamatan/:kode`; kode tak dikenal dijawab `404`.
4. **FE**
   - [x] Aksi ajukan/setujui/arsipkan di `/master/komoditas` memanggil API (`api.master.bulkKomoditas`, hook `useBulkMasterKomoditas`), menampilkan kode yang ditolak, memuat ulang daftar; "Setujui" hanya untuk `master.setujui`, "Ajukan perubahan"/"Arsipkan" hanya untuk `master.ajukan`.
   - [ ] Dialog ajukan perubahan (isi perubahan), antrean persetujuan, riwayat versi dan diff.
5. **TES**
   - [x] Unit usecase (transisi per aksi, item gagal, kode ganda, balapan, audit) dan integrasi (izin per aksi untuk ADMIN_SISTEM, MANAJER_DATA, STRATEGIS_BPS, STRATEGIS_FMIPA, peran lihat-saja; ajukan → setujui menaikkan versi; arsipkan dari AKTIF dan dari MENUNGGU; transisi tak sah; audit).
   - [x] e2e `apps/web/e2e/master.spec.ts`: manajer.data mengajukan (bertahan setelah muat ulang, tanpa tombol Setujui), strategis.bps menyetujui (versi + 1), wilayah/pasar tanpa aksi status, 403/400/item gagal lewat API. Arsip tidak diuji di e2e (tidak ada pemulihan).

---

## Kunci primer: master (000111)

Migrasi `000111_pk_harga_lapangan_master.sql`; kontrak lengkap di [SKEMA-DATABASE](../SKEMA-DATABASE.md).

- **KODE (tetap, kode unik sebagai PK)**: `provinsi(kode varchar(2))`, `kabupaten(kode varchar(4))` (kode BPS), `komoditas(kode)`, `pasar(id varchar)` (mis. `PSR-1171-01`; kolom bernama `id` tetapi isinya kode), dan tabel `kecamatan(kode varchar(7))` yang direncanakan. Kode adalah identitasnya dan dipakai langsung sebagai nilai FK serta di URL; tidak ada surrogate key.
- **FK ke master semuanya `ON DELETE RESTRICT` dan terindeks**: `kabupaten.provinsi_kode` → `provinsi`, `pasar.kabupaten_kode` → `kabupaten` (`idx_pasar_kabupaten`), `kecamatan.kabupaten_kode` → `kabupaten` (direncanakan), `entri_harga.pasar_id` dan `tugas_lapangan.pasar_id` → `pasar`, `entri_harga.komoditas_kode` dan `batas_harga.komoditas_kode` → `komoditas`, `batas_harga.kabupaten_kode` → `kabupaten` (`idx_batas_harga_kabupaten_kode`).
- **BIGINT**: `perubahan_master.id` (antrean persetujuan, `kode` = ID lama) dan `batas_harga.id` (`kode` = `BH-<komoditas>`; kunci alami `uq_batas_harga_berlaku` tetap menjaga satu batas per komoditas, kabupaten, dan tanggal berlaku). Detailnya di tugas 04-B dan [06](06-harga-validasi.md).
- **Seeder** `master.reference` meng-upsert menurut kode (`ON CONFLICT (kode)` untuk provinsi, kabupaten, komoditas; `(id)` untuk pasar, yang memang kodenya) dan tidak pernah menghapus, jadi aman terhadap FK `RESTRICT`.

---

## 04-C — Pelengkap master *(Fase F, prioritas rendah)*

- [ ] `satuan`, `pedagang` (sekarang `entri_harga.kode_pedagang` string bebas), dan keputusan `kualitas` (enum atau tabel).
- [ ] `komoditas`: `berlaku_dari`/`berlaku_sampai` atau tabel riwayat versi agar harga lama merujuk definisi yang berlaku saat itu.
- [ ] Paginasi dan pencarian sisi server untuk data besar (kecamatan se-Aceh; `kecamatan` berkunci KODE `kode varchar(7)`, urut menurut `kode`).
