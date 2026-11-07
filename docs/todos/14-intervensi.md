# 14 — Intervensi (kanban 3 siklus) & workflow engine

**Prioritas:** P0 (Fase 3, PRD Layer 4) · **Status:** UI ✅ · Backend 🟡 (daftar, buat, pindah tahap, riwayat, konkurensi, tugaskan/eskalasi, persetujuan berjenjang, kartu otomatis; what-if/AI belum) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [kanban-board](../../apps/web/src/components/intervensi/kanban-board.tsx) (dnd-kit), kolom dan kartu, tampilan tabel, izin per tahap, [store kanban](../../apps/web/src/lib/stores/kanban-store.ts). Tiga siklus: Normal, Tekanan Harga/Defisit, Surplus.
- **Uji:** Go (integrasi dockertest `internal/intervensi/integration`), web (`lib/api/intervensi.test.ts`, `lib/stores/kanban-store.test.ts`), e2e Playwright `apps/web/e2e/intervensi.spec.ts` (kolom = API tahap, tambah kartu lewat dialog, detail + riwayat, tombol tersembunyi untuk peran tanpa `intervensi.kelola`, 403/400/404 API). Kartu uji e2e dihapus lewat SQL berdasarkan judul `E2E <timestamp>` (riwayat, penanggung jawab, persetujuan ikut terhapus oleh `ON DELETE CASCADE`; baris `audit_log` append-only tetap ada).
- **API Go:** `GET /v1/intervensi` (filter `siklus`/`tahap`/`prioritas`; kartu beserta penanggung jawab), `GET /v1/intervensi/tahap` (definisi tahap), `POST /v1/intervensi` (`intervensi.kelola`; masuk tahap pertama siklusnya; judul berisi spasi saja ditolak 400), `PATCH /v1/intervensi/:id/tahap` (`:id` UUID atau alias `INT-…`; menomori ulang kolom; izin tahap tujuan dari `tahap_intervensi_def.izin_dibutuhkan` ditegakkan di usecase; dicatat di audit). Endpoint `POST /v1/intervensi/bulk` (arsip/hapus) dicabut: tidak pernah berjalan (tabel `kartu_intervensi` tak ada), tidak dipakai web, dan `hapus` merusak data.
- **Web:** `api.intervensi.list` membaca API (`lib/api/intervensi.ts`) dan disegarkan tiap 30 detik. Kolom papan kini berasal dari `GET /v1/intervensi/tahap` (`api.intervensi.tahap`, hook `useTahapIntervensi`); `SIKLUS_TAHAP` di `lib/data/siklus.ts` hanya cadangan klien (mode mock dan bila endpoint gagal, dengan peringatan di papan) dan dijaga tes terhadap `packages/contracts/intervensi-tahap.json`, begitu pula pemeta `toTahapPerSiklus` (respons yang dibangun dari fixture harus menghasilkan kolom yang sama).
- **Detail kartu:** klik kartu (atau tombol "Buka detail …") membuka lembar detail ([kartu-detail-sheet](../../apps/web/src/components/intervensi/kartu-detail-sheet.tsx)) berisi rincian, persetujuan yang akan dilalui kartu (dari `GET /v1/intervensi/alur`), dan linimasa riwayat (`GET /v1/intervensi/:id/riwayat`: dibuat, pindah tahap, penugasan, eskalasi, ajuan/persetujuan/penolakan). Seret tetap berfungsi: tombol detail menghentikan `pointerdown`, dan dnd-kit (jarak aktivasi 6 px) menelan klik setelah seretan.
- **Tambah intervensi:** tombol dan dialog [tambah-intervensi-dialog](../../apps/web/src/components/intervensi/tambah-intervensi-dialog.tsx) (`POST /v1/intervensi`, hanya untuk `intervensi.kelola`) dengan validasi sisi klien yang mencerminkan `CreateIntervensiRequest`; setelah tersimpan papan berpindah ke siklus kartu dan kartu muncul di tahap pertamanya. Alert EWS dan penanggung jawab selain pembuat belum bisa dipilih di dialog (daftar pengguna butuh `pengguna.kelola`).
- **Alur persetujuan:** panel "Alur persetujuan" di bawah papan, label gembok kolom dan toast penolakan drop memakai rantai langkah dari `GET /v1/intervensi/alur`.
- **Skema:** `intervensi` (+ `deskripsi`, `selesai_pada`), `intervensi_pj`, `tahap_intervensi_def` dengan FK komposit `(siklus, tahap)`; entity cocok (migrasi 000031). Seeder `intervensi.reference` (16 tahap) dan `intervensi.demo` (19 kartu meniru `mockKanban`, tertaut ke alert `ews.demo`).

### Kunci primer UUID (migrasi 000112)

Rujukan [SKEMA-DATABASE.md](../SKEMA-DATABASE.md) (matriks "000112") dan `db/migrations/000112_pk_ews_intervensi.sql`.

- **`intervensi.id` UUID.** Kolom `id uuid DEFAULT gen_random_uuid()` (dulu VARCHAR `INT-n`). ID lama dipetakan stabil ke `garda_uuid_stabil('intervensi', id_lama)`, identik dengan `idx.UUIDStabil("intervensi", "INT-301")`, sehingga basis data yang dimigrasi dan yang di-seed memakai UUID yang sama. ID lama disimpan di `kode varchar(64)` UNIQUE sebagai kode tampilan (UI menampilkan `kode`, tautan memakai `id`). Kartu baru: `id` dari basis data, `kode` = `'INT-' || nextval('intervensi_kode_seq')` (sequence dulu bernama `intervensi_id_seq`, mulai 1000, dimajukan melewati kode lama saat migrasi), jadi kode baru tidak bisa bentrok antar-instance. Kartu otomatis dari alert EWS memakai sequence yang sama.
- **Rute `:id` UUID dengan alias kode.** `/v1/intervensi/:id/tahap`, `/:id/riwayat`, dan `/:id/persetujuan` membaca parameter lewat `idx.ParamUUIDAtauKode` (maks 64 karakter): UUID dibaca sebagai ID, selain itu dicari di kolom `kode`, jadi URL lama `/intervensi/INT-301/riwayat` tetap berlaku; yang tidak ditemukan atau salah bentuk **404**. `POST /v1/intervensi/persetujuan/:pid/keputusan`: `:pid` BIGINT lewat `idx.ParamID`, salah bentuk **404** (dulu 400). `POST /v1/intervensi/bulk`: `ids` wajib UUID kanonik (400 bila bukan). `POST /v1/intervensi`: `ews_alert_id` opsional BIGINT (400 bila salah bentuk atau alert tidak ada); respons `{id, kode, tahap}`.
- **Bentuk JSON.** Kartu membawa `id` (UUID), `kode`, `ews_alert_id` (BIGINT string), dan `ews_alert_kode`. `id` riwayat kartu dan `id` permintaan persetujuan kini string BIGINT (dulu angka JSON), `intervensi_id` pada persetujuan berupa UUID.
- **Tipe FK anak.** `intervensi_persetujuan.intervensi_id`, `intervensi_pj.intervensi_id` (bagian PK komposit), dan `intervensi_riwayat.intervensi_id` → `uuid`, ON DELETE **CASCADE**. `intervensi.ews_alert_id` → `bigint` (induk `ews_alert`), ON DELETE **RESTRICT** (alert pemicu adalah bukti asal kartu). `intervensi_persetujuan.alur_kode` tetap `varchar(50)` → `alur_persetujuan(kode)`, **SET NULL**.
- **FK lain.** `intervensi.komoditas_kode`/`kabupaten_kode` → `komoditas`/`kabupaten` **RESTRICT**; `intervensi (siklus, tahap)` dan `alur_persetujuan (siklus, tahap_tujuan)` → `tahap_intervensi_def` **RESTRICT**. FK ke `pengguna` (`dibuat_oleh_id` RESTRICT, `dieskalasi_oleh_id`/`diajukan_oleh`/`oleh` SET NULL, `intervensi_pj.pengguna_id` CASCADE) tidak berubah sejak 000110. `alur_persetujuan` tetap KODE (PK `kode`); `intervensi_persetujuan_langkah` dan `intervensi_pj` tetap KOMPOSIT.
- **Indeks baru.** Antrean SLA: `idx_intervensi_aktif_tahap_sejak (tahap_sejak, id) WHERE selesai_pada IS NULL AND dieskalasi_pada IS NULL` (parsial, kartu aktif yang belum dieskalasi). Antrean persetujuan: `idx_intervensi_persetujuan_menunggu (diajukan_pada DESC, id DESC) WHERE status = 'MENUNGGU'` (parsial) dan `idx_intervensi_persetujuan_status_diajukan (status, diajukan_pada DESC, id DESC)` menggantikan `idx_intervensi_persetujuan_status`. FK yang belum terindeks: `idx_intervensi_komoditas_kode`, `idx_intervensi_kabupaten_kode`, `idx_intervensi_persetujuan_intervensi_id` (penuh, karena `uq_intervensi_persetujuan_terbuka` parsial tidak bisa melayani pemeriksaan FK), `idx_intervensi_persetujuan_alur_kode`, `idx_alur_persetujuan_tahap`. Daftar papan tetap dilayani `idx_intervensi_siklus (siklus, tahap, urutan)`.
- **Rujukan lunak.** `audit_log.entitas_id` pada entitas `intervensi` yang berisi kode lama dipetakan ke UUID oleh migrasi; audit baru menulis UUID. Teks riwayat ("Dibuat otomatis dari alert EWS-…") dan snapshot JSON dibiarkan sebagai sejarah.
- **Seeder.** `intervensi.demo` memakai `idx.UUIDStabil("intervensi", "INT-301")` dan seterusnya dengan `kode` = ID lama, dan menemukan `ews_alert_id` lewat kode alert; idempoten juga pada basis data hasil migrasi.

---

## 14-A — Kanban dari DB *(Fase P, urutan #10)*

**Tujuan:** `/intervensi` menampilkan dan memindahkan kartu dari database; definisi tahap punya satu sumber.
**Bergantung pada:** 10-A (kartu merujuk alert), 02-A.

1. **MIG**
   - [x] `tahap_intervensi_def` (`siklus`, `kode_tahap`, `urutan`, `label`, `izin_dibutuhkan`, `sla_hari`); validasi `intervensi.tahap` terhadapnya.
   - [x] `intervensi`: tambah `deskripsi` dan `selesai_pada` (nullable). Entity diselaraskan ke `intervensi` + `intervensi_pj`: `id` string (sejak 000112 UUID, lihat "Kunci primer"), `siklus` enum, `kabupaten_kode`; hapus `pic` (digantikan `intervensi_pj`) dan tipe `tahap_intervensi`.
2. **SEED**
   - [x] `reference`: definisi tahap untuk tiga siklus beserta izin per tahap (sumber: `lib/data/siklus.ts` dan PRD §4.1).
   - [x] `demo`: kartu dan penanggung jawab yang meniru `mockKanban`; `ews_alert_id` merujuk seed EWS ([10-A](10-ews.md)). Tidak ada kartu contoh di produksi.
3. **API**
   - [x] `GET /v1/intervensi` mengembalikan bentuk `KartuIntervensi` (dengan daftar penanggung jawab); `PATCH /:id/tahap` memvalidasi tahap terhadap definisi.
   - [x] Endpoint `GET /v1/intervensi/tahap` untuk definisi tahap.
4. **FE**
   - [x] `api.intervensi.*` sudah memanggil API; ganti sumber definisi tahap di `lib/data/siklus.ts` dengan data API (atau pertahankan sebagai cache dengan tes kesamaan).
   - [x] Papan memakai definisi tahap dari `GET /v1/intervensi/tahap` (14-A, ganti sumber ke API; cadangan statis hanya bila gagal).
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
   - [ ] `intervensi_komentar` dan `intervensi_lampiran` (belum ada migrasi; rancangan skema mengikuti [SKEMA-DATABASE.md](../SKEMA-DATABASE.md)):
     - **`intervensi_komentar`**: `id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY`; `intervensi_id uuid NOT NULL REFERENCES intervensi(id) ON DELETE CASCADE`; `penulis_id uuid NOT NULL REFERENCES pengguna(id) ON DELETE RESTRICT` (penulis adalah bukti asal; akun yang pernah berkomentar tidak bisa dihapus, hanya dinonaktifkan); `isi text NOT NULL`; `dibuat_pada timestamptz NOT NULL DEFAULT now()`. Indeks `idx_intervensi_komentar_intervensi (intervensi_id, dibuat_pada DESC)` dan `idx_intervensi_komentar_penulis_id (penulis_id)`.
     - **`intervensi_lampiran`**: `id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY`; `intervensi_id uuid NOT NULL REFERENCES intervensi(id) ON DELETE CASCADE`; `pengunggah_id uuid NOT NULL REFERENCES pengguna(id) ON DELETE RESTRICT`; `nama_berkas`, `tipe_konten`, `ukuran_byte`; **`kunci_objek` (kunci di penyimpanan objek) dan `hash_sha256`, bukan blob** di basis data; `dibuat_pada timestamptz NOT NULL DEFAULT now()`. Indeks `idx_intervensi_lampiran_intervensi (intervensi_id, dibuat_pada DESC)` dan `idx_intervensi_lampiran_pengunggah_id (pengunggah_id)`.
     - **Rute** (kartu = `idx.ParamUUIDAtauKode`, anak = `idx.ParamID`, salah bentuk 404): `/v1/intervensi/:id/komentar` dan `/v1/intervensi/:id/komentar/:komentarId` (BIGINT); pola sama untuk `/v1/intervensi/:id/lampiran` dan `/v1/intervensi/:id/lampiran/:lampiranId` (BIGINT). Sub-sumber dicari dengan `WHERE id = ? AND intervensi_id = ?`, sehingga ID milik kartu lain dijawab 404. Hanya spesifikasi; metode, izin, dan UI belum ditetapkan.
2. **SEED** (`demo`): riwayat tiap kartu demo (dibuat di tahap pertama, lalu dipindah sampai tahap sekarang); dibangun ulang tiap seed. Komentar: belum.
3. **API**
   - [x] Izin per tahap dari `tahap_intervensi_def.izin_dibutuhkan` (14-A) ditambah aturan berjenjang: pemindahan maju tidak boleh melompati tahap persetujuan yang izinnya tidak dimiliki; galat 403 menyebut tahap dan izinnya.
   - [x] `PATCH /:id/tahap` wajib `versi`; versi usang 409; respons membawa versi baru; audit sukses dan penolakan.
   - [x] `POST /v1/intervensi/bulk` (`intervensi.kelola`, field `action`): `tugaskan` (jadikan pemanggil penanggung jawab) dan `eskalasi` (tandai ke Dewan Pengarah, prioritas TINGGI), hasil per kartu, audit. Row scope per peran/wilayah: belum.
   - [x] `GET /:id/riwayat` (`intervensi.lihat`).
4. **FE**: "Tugaskan ke saya" dan "Eskalasi ke Dewan Pengarah" memanggil API (tidak lagi toast simulasi), papan memakai dan memperbarui `versi`, 409 memuat ulang papan dengan pesan jelas, pesan izin dari server saat drop ditolak, lencana Eskalasi pada kartu. Dialog buat kartu: ✅ (lihat di atas); dialog ubah kartu: belum.
5. **TES**: unit usecase (versi, lompat tahap, bulk, riwayat), integrasi Postgres (versi usang 409, dua pemindahan bersamaan tepat satu menang, riwayat hanya saat tahap berganti, hasil bulk per kartu, 401/403/400), seeder, `kanban-store`.

---

## 14-C — Workflow engine *(Fase F, gelombang F-5; PRD §4)* — ✅ persetujuan berjenjang, kartu otomatis, eskalasi SLA (sebagian)

**Bergantung pada:** 10-B, 08-B.

**MIG** (`000074_14c_workflow_persetujuan.sql`): `alur_persetujuan` + `alur_persetujuan_langkah` (definisi alur sebagai data), `intervensi_persetujuan` + `intervensi_persetujuan_langkah` (permintaan, rantai langkah yang disalin saat diajukan, keputusan per langkah), `intervensi.tahap_sejak` (jam SLA tahap), `dibuat_oleh_id` boleh NULL (kartu buatan sistem), UNIQUE satu kartu otomatis per alert. **SEED** `reference`: lima alur (petakan suplai, dukungan fiskal [Dewan Pengarah lalu Kanwil DJPb], dua penutupan, eskalasi), hanya ditambahkan bila belum ada sehingga perubahan lewat data bertahan; `demo`: satu permintaan terbuka (kartu `INT-304`, kode tampilan, ke Dukungan instrumen belanja).

- [x] **Persetujuan berjenjang:** `POST /v1/intervensi/:id/persetujuan` (`intervensi.kelola`) membentuk rantai dari alur tiap tahap persetujuan yang dilalui; `POST /persetujuan/:pid/keputusan` (`:pid` BIGINT, salah bentuk 404; `intervensi.setujui` atau `setujui_fiskal` menurut langkah) dengan pemisahan tugas (pengaju dan pemutus langkah lain tidak boleh memutus), tolak wajib bercatatan, setuju di langkah akhir memindahkan kartu otomatis; `GET /persetujuan`, `GET /alur`. Semua tercatat di riwayat kartu dan audit. Satu permintaan terbuka per kartu.
- [x] **Eskalasi nyata:** eskalasi (aksi massal 14-B atau SLA) membuka permintaan telaah Dewan Pengarah; menolaknya mencabut tanda eskalasi.
- [x] **Kartu otomatis dari alert EWS:** alert >= `INTERVENSI_KARTU_OTOMATIS_MIN` (bawaan SIAGA; lonjakan harga, kelangkaan stok, dan cuaca memicu siklus Tekanan, over-supply memicu Surplus) menjadi kartu di tahap pertama dengan tautan `ews_alert_id`, tenggat dari SLA tahap pertama, ditugasi otomatis ke peran MANAJER_PROGRAM; idempoten, tidak menggandakan kartu yang sudah ada.
- [x] **Eskalasi bila lewat SLA:** `INTERVENSI_SLA_INTERVAL` (bawaan mati) mengeskalasi kartu yang lebih lama di tahapnya daripada `sla_hari`.
- [x] Definisi alur sebagai data (tabel, bukan kode). Antarmuka mengubah alur: belum (lewat basis data).
- [x] FE: referensi alur persetujuan (`GET /v1/intervensi/alur`) di panel "Alur persetujuan", lembar detail kartu, dan label gembok kolom.
- [x] FE: antrean "Menunggu persetujuan" di `/intervensi` (rantai langkah, setujui/tolak dengan catatan), tombol "Ajukan persetujuan" saat drop ditolak karena izin.
- [ ] Pemicu siklus otomatis (siklus aktif berubah menurut indikator, kembali Normal bila pulih): siklus aktif dihitung modul 08, belum dipicu workflow.
- [ ] Pengingat tenggat, simulasi what-if, rekomendasi AI + validasi manusia, integrasi ke APBD/TPID lewat policy brief, `checklist_item`.
