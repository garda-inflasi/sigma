# 14 — Intervensi (kanban 3 siklus) & workflow engine

**Prioritas:** P0 (Fase 3, PRD Layer 4) · **Status:** UI ✅ · Backend 🟡 (daftar, buat, pindah tahap, riwayat, konkurensi, tugaskan/eskalasi, persetujuan berjenjang, kartu otomatis; what-if/AI belum) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [kanban-board](../../apps/web/src/components/intervensi/kanban-board.tsx) (dnd-kit), kolom dan kartu, tampilan tabel, izin per tahap, [store kanban](../../apps/web/src/lib/stores/kanban-store.ts). Tiga siklus: Normal, Tekanan Harga/Defisit, Surplus.
- **API Go:** `GET /v1/intervensi` (filter `siklus`/`tahap`/`prioritas`; kartu beserta penanggung jawab), `GET /v1/intervensi/tahap` (definisi tahap), `POST /v1/intervensi` (`intervensi.kelola`; masuk tahap pertama siklusnya), `PATCH /v1/intervensi/:id/tahap` (menomori ulang kolom; izin tahap tujuan dari `tahap_intervensi_def.izin_dibutuhkan` ditegakkan di usecase; dicatat di audit). Endpoint `POST /v1/intervensi/bulk` (arsip/hapus) dicabut: tidak pernah berjalan (tabel `kartu_intervensi` tak ada), tidak dipakai web, dan `hapus` merusak data.
- **Web:** `api.intervensi.list` membaca API (`lib/api/intervensi.ts`) dan disegarkan tiap 30 detik; `SIKLUS_TAHAP` di `lib/data/siklus.ts` dipertahankan sebagai cache klien dan dijaga tes kesamaan terhadap `packages/contracts/intervensi-tahap.json` (cermin seed `intervensi.reference`).
- **Skema:** `intervensi` (+ `deskripsi`, `selesai_pada`), `intervensi_pj`, `tahap_intervensi_def` dengan FK komposit `(siklus, tahap)`; entity cocok (migrasi 000031). Seeder `intervensi.reference` (16 tahap) dan `intervensi.demo` (19 kartu meniru `mockKanban`, tertaut ke alert `ews.demo`).

---

## 14-A — Kanban dari DB *(Fase P, urutan #10)*

**Tujuan:** `/intervensi` menampilkan dan memindahkan kartu dari database; definisi tahap punya satu sumber.
**Bergantung pada:** 10-A (kartu merujuk alert), 02-A.

1. **MIG**
   - [x] `tahap_intervensi_def` (`siklus`, `kode_tahap`, `urutan`, `label`, `izin_dibutuhkan`, `sla_hari`); validasi `intervensi.tahap` terhadapnya.
   - [x] `intervensi`: tambah `deskripsi` dan `selesai_pada` (nullable). Entity diselaraskan ke `intervensi` + `intervensi_pj`: `id` string, `siklus` enum, `kabupaten_kode`; hapus `pic` (digantikan `intervensi_pj`) dan tipe `tahap_intervensi`.
2. **SEED**
   - [x] `reference`: definisi tahap untuk tiga siklus beserta izin per tahap (sumber: `lib/data/siklus.ts` dan PRD §4.1).
   - [x] `demo`: kartu dan penanggung jawab yang meniru `mockKanban`; `ews_alert_id` merujuk seed EWS ([10-A](10-ews.md)). Tidak ada kartu contoh di produksi.
3. **API**
   - [x] `GET /v1/intervensi` mengembalikan bentuk `KartuIntervensi` (dengan daftar penanggung jawab); `PATCH /:id/tahap` memvalidasi tahap terhadap definisi.
   - [x] Endpoint `GET /v1/intervensi/tahap` untuk definisi tahap.
4. **FE**
   - [x] `api.intervensi.*` sudah memanggil API; ganti sumber definisi tahap di `lib/data/siklus.ts` dengan data API (atau pertahankan sebagai cache dengan tes kesamaan).
   - [x] `useIntervensi` memakai `staleTime: Infinity`: kartu yang digeser pengguna lain tidak muncul tanpa refresh; tambahkan polling ringan.
5. **TES**: setiap `intervensi.tahap` ada di `tahap_intervensi_def` untuk siklusnya; hapus `intervensi` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 14-B — Izin per tahap, riwayat, konkurensi *(Fase F, gelombang F-2)* — ✅ (komentar/lampiran belum)

**Tujuan:** aturan persetujuan berjenjang ditegakkan di backend dan setiap perpindahan tercatat.
**Bergantung pada:** 14-A, 03-A.

1. **MIG** (`000073_14b_intervensi_riwayat_versi.sql`)
   - [x] `intervensi_riwayat` (`intervensi_id`, `aksi` BUAT/PINDAH_TAHAP/TUGASKAN/ESKALASI, `dari`, `ke`, `oleh` NULL = sistem, `waktu`, `alasan`).
   - [x] `intervensi`: `versi` INT untuk optimistic locking, `dieskalasi_pada`/`dieskalasi_oleh_id`.
   - [x] Urutan kartu: tetap nomor bulat yang dinomori ulang dalam transaksi dengan `FOR UPDATE` (cukup untuk konkurensi; indeks pecahan tidak diperlukan).
   - [ ] `intervensi_komentar`, `intervensi_lampiran`.
2. **SEED** (`demo`): riwayat tiap kartu demo (dibuat di tahap pertama, lalu dipindah sampai tahap sekarang); dibangun ulang tiap seed. Komentar: belum.
3. **API**
   - [x] Izin per tahap dari `tahap_intervensi_def.izin_dibutuhkan` (14-A) ditambah aturan berjenjang: pemindahan maju tidak boleh melompati tahap persetujuan yang izinnya tidak dimiliki; galat 403 menyebut tahap dan izinnya.
   - [x] `PATCH /:id/tahap` wajib `versi`; versi usang 409; respons membawa versi baru; audit sukses dan penolakan.
   - [x] `POST /v1/intervensi/bulk` (`intervensi.kelola`, field `action`): `tugaskan` (jadikan pemanggil penanggung jawab) dan `eskalasi` (tandai ke Dewan Pengarah, prioritas TINGGI), hasil per kartu, audit. Row scope per peran/wilayah: belum.
   - [x] `GET /:id/riwayat` (`intervensi.lihat`).
4. **FE**: "Tugaskan ke saya" dan "Eskalasi ke Dewan Pengarah" memanggil API (tidak lagi toast simulasi), papan memakai dan memperbarui `versi`, 409 memuat ulang papan dengan pesan jelas, pesan izin dari server saat drop ditolak, lencana Eskalasi pada kartu. Dialog buat/ubah kartu: belum.
5. **TES**: unit usecase (versi, lompat tahap, bulk, riwayat), integrasi Postgres (versi usang 409, dua pemindahan bersamaan tepat satu menang, riwayat hanya saat tahap berganti, hasil bulk per kartu, 401/403/400), seeder, `kanban-store`.

---

## 14-C — Workflow engine *(Fase F, gelombang F-5; PRD §4)* — ✅ persetujuan berjenjang, kartu otomatis, eskalasi SLA (sebagian)

**Bergantung pada:** 10-B, 08-B.

**MIG** (`000074_14c_workflow_persetujuan.sql`): `alur_persetujuan` + `alur_persetujuan_langkah` (definisi alur sebagai data), `intervensi_persetujuan` + `intervensi_persetujuan_langkah` (permintaan, rantai langkah yang disalin saat diajukan, keputusan per langkah), `intervensi.tahap_sejak` (jam SLA tahap), `dibuat_oleh_id` boleh NULL (kartu buatan sistem), UNIQUE satu kartu otomatis per alert. **SEED** `reference`: lima alur (petakan suplai, dukungan fiskal [Dewan Pengarah lalu Kanwil DJPb], dua penutupan, eskalasi), hanya ditambahkan bila belum ada sehingga perubahan lewat data bertahan; `demo`: satu permintaan terbuka (INT-304 ke Dukungan instrumen belanja).

- [x] **Persetujuan berjenjang:** `POST /v1/intervensi/:id/persetujuan` (`intervensi.kelola`) membentuk rantai dari alur tiap tahap persetujuan yang dilalui; `POST /persetujuan/:pid/keputusan` (`intervensi.setujui` atau `setujui_fiskal` menurut langkah) dengan pemisahan tugas (pengaju dan pemutus langkah lain tidak boleh memutus), tolak wajib bercatatan, setuju di langkah akhir memindahkan kartu otomatis; `GET /persetujuan`, `GET /alur`. Semua tercatat di riwayat kartu dan audit. Satu permintaan terbuka per kartu.
- [x] **Eskalasi nyata:** eskalasi (aksi massal 14-B atau SLA) membuka permintaan telaah Dewan Pengarah; menolaknya mencabut tanda eskalasi.
- [x] **Kartu otomatis dari alert EWS:** alert >= `INTERVENSI_KARTU_OTOMATIS_MIN` (bawaan SIAGA; lonjakan harga, kelangkaan stok, dan cuaca memicu siklus Tekanan, over-supply memicu Surplus) menjadi kartu di tahap pertama dengan tautan `ews_alert_id`, tenggat dari SLA tahap pertama, ditugasi otomatis ke peran MANAJER_PROGRAM; idempoten, tidak menggandakan kartu yang sudah ada.
- [x] **Eskalasi bila lewat SLA:** `INTERVENSI_SLA_INTERVAL` (bawaan mati) mengeskalasi kartu yang lebih lama di tahapnya daripada `sla_hari`.
- [x] Definisi alur sebagai data (tabel, bukan kode). Antarmuka mengubah alur: belum (lewat basis data).
- [x] FE: antrean "Menunggu persetujuan" di `/intervensi` (rantai langkah, setujui/tolak dengan catatan), tombol "Ajukan persetujuan" saat drop ditolak karena izin.
- [ ] Pemicu siklus otomatis (siklus aktif berubah menurut indikator, kembali Normal bila pulih): siklus aktif dihitung modul 08, belum dipicu workflow.
- [ ] Pengingat tenggat, simulasi what-if, rekomendasi AI + validasi manusia, integrasi ke APBD/TPID lewat policy brief, `checklist_item`.
