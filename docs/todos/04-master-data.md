# 04 — Master data (komoditas, wilayah, pasar)

**Prioritas:** P0 (PRD §1.2) · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
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
   - [x] `perubahan_master` (000120): tambah `alasan`, `alasan_penolakan`, `disetujui_pada`, serta `jenis` (`TAMBAH`/`UBAH`/`TINJAU`), `sebelum` (nilai bidang saat diajukan, untuk diff), `ditolak_oleh_id` (FK `pengguna` RESTRICT + indeks), dan `ditolak_pada`.
     - **Kunci dan indeks:** PK `BIGINT` identity (000111) dan indeks antrean parsial `idx_perubahan_master_menunggu` dipakai apa adanya.
     - **Satu usulan menunggu per data:** indeks unik parsial `uq_perubahan_master_menunggu_entitas (entitas, entitas_id) WHERE status = 'MENUNGGU'`.
     - **`versi`:** versi data yang berlaku bila usulan disetujui.
     - **CHECK:** penolakan wajib beralasan; `perubahan` berupa objek. Keduanya `NOT VALID` untuk baris lama.
     - **Rujukan entitas** tetap polimorfik (`entitas` + `entitas_id` tanpa FK), karena usulan TAMBAH merujuk kode yang belum ada.
   - [x] `kecamatan` dengan PK **KODE** (000120).
     - **Kolom:** `kode varchar(7)` (kode BPS 7 digit), `nama`, `kabupaten_kode` → `kabupaten` RESTRICT (`idx_kecamatan_kabupaten_kode`), dan `kode_dagri` (padanan Kemendagri).
     - **CHECK:** kode 7 digit dan berawalan kode kabupatennya.
     - **Pasar:** `pasar.kecamatan_kode` → `kecamatan` RESTRICT (`idx_pasar_kecamatan_kode`, CHECK satu kabupaten) dan `pasar.versi` (naik setiap usulan pasar disetujui).
     - **Pendaftaran:** `PKKode` di `kategoriPK`; baris di [SKEMA-DATABASE](../SKEMA-DATABASE.md).
2. **SEED**
   - [x] Impor kecamatan se-Aceh dari BPS lewat skrip (catat sumber dan lisensi), bukan ditulis tangan.
     - **Skrip:** `gardactl master impor-kecamatan` (`make db/impor/kecamatan`) mengambil API bridging wilayah BPS (`sig.bps.go.id`, `periode_merge=2024_1.2022`) per kabupaten dan memvalidasinya.
     - **Hasil:** `internal/master/seeder/kecamatan_aceh.csv`, 290 kecamatan untuk 23 kabupaten/kota, dengan kepala sumber, tanggal ambil, dan atribusi. Sumber dan syarat penggunaan BPS dicatat di `seeder/SUMBER.md`. Beberapa `nama_dagri` tampak salah ketik di sumber dan dibiarkan apa adanya (dicatat di SUMBER.md).
     - **Seeder `master.reference`:** meng-upsert `satuan` lalu `kecamatan` (sesudah kabupaten). Toleran skema lama, karena tes migrasi bertahap menjalankannya sebelum 000120/000121.
     - **Seeder demo `master.demo`:** mengisi `pedagang` dari pasangan (pasar, kode pedagang) entri harga demo.
3. **API**
   - [x] Aksi massal komoditas `POST /v1/master/komoditas/bulk` (`ajukan_perubahan`, `setujui`, `arsipkan`) dengan izin per aksi: `setujui` butuh `master.setujui` (STRATEGIS_BPS, STRATEGIS_FMIPA), `ajukan_perubahan`/`arsipkan` butuh `master.ajukan` (ADMIN_SISTEM, MANAJER_DATA, MANAJER_PROGRAM). Pengaju tidak dapat menyetujui usulannya sendiri. Transisi dinilai per kode: AKTIF → MENUNGGU_PERSETUJUAN (ajukan); MENUNGGU_PERSETUJUAN → AKTIF dan `versi` + 1 (setujui); AKTIF atau MENUNGGU_PERSETUJUAN → DIARSIPKAN (arsipkan, tanpa pemulihan). Kode tak dikenal atau transisi tak sah menjadi item gagal di `rincian` (`kode`, `berhasil`, `pesan`) tanpa menggagalkan item lain; pembaruan memakai `WHERE status IN (...)` sehingga balapan juga jadi item gagal. Audit `BULK_<aksi>` mencatat kode yang berubah dan yang gagal.
   - [x] Hubungkan entity ke `perubahan_master` (riwayat versi, alasan, penolakan); endpoint tolak. Entity `PerubahanMaster` (`ID idx.ID`); rute `:id` BIGINT lewat `idx.ParamID` (salah bentuk → 404, tanpa alias kode); ID di JSON string desimal.
     - **Endpoint:**
       - `GET /v1/master/perubahan`: antrean `status=MENUNGGU`, terlama lebih dulu; status lain terbaru lebih dulu; `X-Total-Count`.
       - `GET /v1/master/perubahan/:id`: detail dengan diff per bidang.
       - `POST /v1/master/perubahan`: ajukan, izin `master.ajukan`.
       - `POST /v1/master/perubahan/:id/setujui` dan `/tolak`: izin `master.setujui`; tolak wajib beralasan.
       - `POST /v1/master/perubahan/:id/batalkan`: hanya pengaju.
     - **Isi usulan:**
       - Komoditas: `nama`, `kategori`, `satuan`, `harga_acuan`, `shelf_life_hari`, `pilot`.
       - Pasar: `nama`, `lat`, `lng`, `radius_meter`, `aktif`, `kecamatan_kode`; TAMBAH juga `kabupaten_kode`.
       - Usulan divalidasi per bidang (`domain.ValidasiUsulan`). Bidang yang tidak berubah dibuang (400 `TANPA_PERUBAHAN` bila tak tersisa).
     - **Kunci baris dalam transaksi** menjaga aturan:
       - satu usulan menunggu per data (409 `USULAN_MENUNGGU`);
       - pengaju tidak memutuskan usulannya sendiri (403 `PEMISAHAN_TUGAS`);
       - data yang berubah sejak diajukan tidak diterapkan (409 `USULAN_KEDALUWARSA`);
       - komoditas arsip tidak bisa diubah (409 `STATUS_TIDAK_SESUAI`);
       - kode TAMBAH unik (409 `KODE_SUDAH_ADA`).
     - **Persetujuan:**
       - Isi usulan diterapkan dan `versi` naik.
       - Riwayat versi komoditas (`komoditas_versi`) menautkan usulannya lewat GUC transaksi `garda.perubahan_master_id`.
       - Setiap langkah dicatat di audit trail: `AJUKAN_`, `SETUJUI_`, `TOLAK_`, `BATALKAN_PERUBAHAN_MASTER`. Persetujuan mencatat keadaan sebelum/sesudah.
     - **Aksi massal ikut antrean:**
       - `ajukan_perubahan` membuat usulan `TINJAU`.
       - `setujui` menerapkan usulan yang menunggu dengan aturan pemisahan tugas yang sama. Status menunggu lama tanpa baris usulan tetap disetujui seperti dulu.
       - `arsipkan` menolak usulan yang menunggu ("Komoditas diarsipkan").
     - **Seeder:** `master.reference` tidak lagi menimpa komoditas/pasar yang sudah diubah lewat persetujuan (`versi > 1`).
     - **Tes:**
       - integrasi `internal/master/integration/perubahan_test.go`;
       - unit `internal/master/domain/perubahan_test.go`.
   - **Wilayah dan pasar: tidak ada endpoint bulk.** Tabel `kabupaten` tidak punya kolom `status`/`versi` dan `pasar` hanya punya `aktif` (boolean), jadi siklus ajukan/setujui/arsip tidak berlaku tanpa migrasi. Migrasi tidak ditambahkan di sini; aksi ajukan/setujui/arsipkan dihapus dari UI untuk kedua entitas itu (tinggal Ekspor CSV). Bila siklus persetujuan wilayah/pasar diperlukan, tambahkan kolom `status` (enum `status_master`) dan `versi` pada `kabupaten` dan `pasar` (atau pakai `perubahan_master`), lalu perluas `domain.TransisiUntuk` dan repo agar generik per tabel.
   - [x] Validasi referensial: komoditas atau pasar yang sudah punya data harga hanya dinonaktifkan, tidak dihapus.
     - **Basis data:** FK `RESTRICT` menegakkannya (23503).
     - **`DELETE /v1/master/komoditas/:kode` dan `/pasar/:kode`** (izin `master.setujui`):
       - 204 bila tidak dirujuk (riwayat versi ikut CASCADE) dan dicatat `HAPUS_MASTER`;
       - 409 `MASTER_DIRUJUK` dengan saran mengarsipkan komoditas atau menonaktifkan pasar lewat usulan;
       - 409 `MASTER_ADA_USULAN` bila ada usulan menunggu;
       - 404 untuk kode tak dikenal.
     - **Rute detail ber-`:kode`:** `/v1/master/komoditas/:kode` (+ `riwayat_versi`, `perubahan_menunggu`), `/wilayah/:kode` (+ jumlah kecamatan/pasar), `/pasar/:kode` (+ kecamatan, pedagang, `perubahan_menunggu`), `/kecamatan/:kode`. Kode tak dikenal atau salah bentuk dijawab 404.
4. **FE**
   - [x] Aksi ajukan/setujui/arsipkan di `/master/komoditas` memanggil API (`api.master.bulkKomoditas`, hook `useBulkMasterKomoditas`), menampilkan kode yang ditolak, memuat ulang daftar; "Setujui" hanya untuk `master.setujui`, "Ajukan perubahan"/"Arsipkan" hanya untuk `master.ajukan`.
   - [x] Dialog ajukan perubahan (isi perubahan), antrean persetujuan, riwayat versi dan diff.
     - **Dialog usulan** (`components/master/usulan-dialog.tsx`): UBAH/TAMBAH komoditas dan pasar. Hanya bidang yang berubah dikirim; validasi klien mengikuti aturan server; satuan dan kecamatan dari API.
     - **Antrean `/master/perubahan`:** tab Menunggu/Riwayat, filter entitas, paginasi server, detail dengan tabel diff, aksi Setujui/Tolak/Batalkan sesuai izin dan kepemilikan. Lencana jumlah menunggu tampil di navigasi untuk `master.setujui`.
     - **Sheet detail komoditas/pasar:** riwayat versi, kartu usulan menunggu, Hapus dengan pesan 409.
     - **Tab baru:** `/master/kecamatan`, `/master/pedagang` (pencarian dan paginasi sisi server lewat `X-Total-Count`) dan `/master/satuan`.
     - **Mode mock:** backend palsu di memori (`master-mock.ts`) dengan alur yang sama.
5. **TES**
   - [x] Unit usecase (transisi per aksi, item gagal, kode ganda, balapan, audit) dan integrasi (izin per aksi untuk ADMIN_SISTEM, MANAJER_DATA, STRATEGIS_BPS, STRATEGIS_FMIPA, peran lihat-saja; ajukan → setujui menaikkan versi; arsipkan dari AKTIF dan dari MENUNGGU; transisi tak sah; audit).
   - [x] e2e `apps/web/e2e/master.spec.ts`: manajer.data mengajukan (bertahan setelah muat ulang, tanpa tombol Setujui), strategis.bps menyetujui (versi + 1), wilayah/pasar tanpa aksi status, 403/400/item gagal lewat API. Arsip tidak diuji di e2e (tidak ada pemulihan).

---

## Kunci primer: master (000111)

Migrasi `000111_pk_harga_lapangan_master.sql`; kontrak lengkap di [SKEMA-DATABASE](../SKEMA-DATABASE.md).

- **KODE (tetap, kode unik sebagai PK)**: `provinsi(kode varchar(2))`, `kabupaten(kode varchar(4))` (kode BPS), `komoditas(kode)`, `pasar(id varchar)` (mis. `PSR-1171-01`; kolom bernama `id` tetapi isinya kode), dan tabel `kecamatan(kode varchar(7))` yang direncanakan. Kode adalah identitasnya dan dipakai langsung sebagai nilai FK serta di URL; tidak ada surrogate key.
- **FK ke master semuanya `ON DELETE RESTRICT` dan terindeks**: `kabupaten.provinsi_kode` → `provinsi`, `pasar.kabupaten_kode` → `kabupaten` (`idx_pasar_kabupaten`), `kecamatan.kabupaten_kode` → `kabupaten` (direncanakan), `entri_harga.pasar_id` dan `tugas_lapangan.pasar_id` → `pasar`, `entri_harga.komoditas_kode` dan `batas_harga.komoditas_kode` → `komoditas`, `batas_harga.kabupaten_kode` → `kabupaten` (`idx_batas_harga_kabupaten_kode`).
- **BIGINT**: `perubahan_master.id` (antrean persetujuan, `kode` = ID lama) dan `batas_harga.id` (`kode` = `BH-<komoditas>`; kunci alami `uq_batas_harga_berlaku` tetap menjaga satu batas per komoditas, kabupaten, dan tanggal berlaku). Detailnya di tugas 04-B dan [06](06-harga-validasi.md).
- **Seeder** `master.reference` meng-upsert menurut kode (`ON CONFLICT (kode)` untuk provinsi, kabupaten, komoditas, dan pasar sejak 04-D; sebelum 000122 pasar memakai `(id)`, yang saat itu berisi kodenya) dan tidak pernah menghapus, jadi aman terhadap FK `RESTRICT`.

---

## 04-C — Pelengkap master *(Fase F, prioritas rendah)*

- [x] `satuan`, `pedagang` (sekarang `entri_harga.kode_pedagang` string bebas), dan keputusan `kualitas` (enum atau tabel). Migrasi 000121.
  - **`satuan`:** PK KODE; `komoditas.satuan` menjadi FK RESTRICT.
  - **`pedagang`:** PK BIGINT, `UNIQUE (pasar_id, kode)`. `entri_harga.kode_pedagang` tetap teks bebas agar entri luring tidak gagal; dicocokkan menurut pasar + kode.
  - **Kualitas:** **tetap enum** (PREMIUM/MEDIUM/ASALAN). Nilainya sedikit, stabil, dan tanpa atribut lain.
  - **API:** `GET /v1/master/satuan` dan `GET /v1/master/pedagang?pasar_id=`.
- [x] `komoditas`: `berlaku_dari`/`berlaku_sampai` atau tabel riwayat versi agar harga lama merujuk definisi yang berlaku saat itu. Dipilih tabel riwayat `komoditas_versi` (000121).
  - **Pengisian:** append-only, ditulis pemicu setiap isi komoditas berubah, dengan `berlaku_dari` dan `perubahan_id` usulan yang menghasilkannya.
  - **API:** `GET /v1/master/komoditas/:kode/versi?pada=` mengembalikan definisi yang berlaku pada waktu itu. Tanggal dibaca sebagai akhir hari WIB; harga yang lebih tua dari riwayat memakai versi paling awal.
- [x] Paginasi dan pencarian sisi server untuk data besar: `GET /v1/master/kecamatan?kabupaten_kode=&q=&page=&limit=` (urut `kode`, `X-Total-Count`), begitu pula pedagang; dipakai tab web tanpa memuat semua baris.

---

## 04-D — Standardisasi Kunci Primer Master Data & Penataan Kolom (000122)

**Tujuan:** Menerapkan kunci primer surrogate terpadu dengan auto-increment (`id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY`) untuk keenam tabel master data (`provinsi`, `satuan`, `kabupaten`, `kecamatan`, `komoditas`, `pasar`), serta menata ulang posisi kolom fisik tabel:
- Kolom `id` (PK) di sisi paling kiri.
- Kolom atribut utama bisnis (`kode`, `nama`, foreign keys, kategori, harga) di tengah.
- Kolom atribut sekunder / opsional (`lat`, `lng`, `radius_meter`, `aktif`, `versi`, `pilot`, `shelf_life_hari`, `kode_dagri`) di sisi kanan.

1. **MIG (000122_04d_pk_master_data.sql)**:
   - [x] Menambahkan kolom `id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY` pada tabel `provinsi`, `satuan`, `kabupaten`, `kecamatan`, `komoditas`, dan `pasar`.
   - [x] Mengubah kolom `pasar.id` lama (berisi teks kode seperti `PSR-1171-01`) menjadi `pasar.kode varchar(255) UNIQUE NOT NULL`.
   - [x] Memastikan seluruh foreign key relasi anak (`entri_harga`, `pedagang`, `tugas_lapangan`, dll.) merujuk ke kode unik master data dengan aturan `ON DELETE RESTRICT` (atau `CASCADE` untuk `komoditas_versi`, `wilayah_geometri`, `jarak_antar_wilayah`).
   - [x] Menyusun ulang struktur dan urutan kolom fisik sesuai spesifikasi arsitektur data.
   - [x] Membangun ulang seluruh dependent views: `agregat_harga_harian`, `mv_heatmap_harga`, `gold_harga_harian`, `mv_kpi_operasional`, dan view `wilayah`.

2. **ENTITAS & DTO**:
   - [x] Menambahkan field `ID idx.ID` pada entity Go `Provinsi`, `Satuan`, `Kabupaten`, `Kecamatan`, `Komoditas`, `Pasar`.
   - [x] DTO `MasterKomoditasRowResponse`, `MasterWilayahRowResponse`, `MasterPasarRowResponse`, `KecamatanRowResponse`, dan `SatuanRowResponse` kini menyediakan `id` (numeric ID) dan `kode` (string code).

3. **QUERY & REPOSITORI**:
   - [x] Repositori referensi master data mendukung pencarian ganda (dual lookup) baik menggunakan numeric surrogate `id` maupun string natural `kode`: `id::text = ? OR kode = ?`.
   - [x] Operasi update dan delete mendukung surrogate id maupun natural kode.

4. **SEEDER & SQL SEEDS**:
   - [x] Seeder `master.reference` dan demo dioptimasi untuk meng-upsert menggunakan `kode` (`ON CONFLICT (kode) DO NOTHING / DO UPDATE`).
   - [x] Susulan (8 Nov 2026, bersama [20](20-profile.md)): seeder `harga.demo`/`lapangan.demo` memakai `pasar.kode` (kolom anak `pasar_id` berisi kode, bukan id surrogate); join `p.kode = e.pasar_id` tanpa `OR p.id::text`; web memakai kode pasar untuk usulan, pedagang, dan hapus. `cmd/seed reference demo` kembali berhasil dan idempoten; `go test ./...` hijau.
   - [x] Menyediakan berkas SQL statis mandiri `db/seeds/01_master_reference.sql`.
