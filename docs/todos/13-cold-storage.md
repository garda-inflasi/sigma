# 13 — Cold storage & food loss

**Prioritas:** P1 (PRD §2.4) · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

Fitur paling matang: migrasi, backend, BFF, seeder, dan UI sudah terhubung.

## Yang sudah ada

- **UI:** [cs-page.tsx](../../apps/web/src/components/cold-storage/cs-page.tsx), [facility-card](../../apps/web/src/components/cold-storage/facility-card.tsx), [stock-table](../../apps/web/src/components/cold-storage/stock-table.tsx) (paginasi, sort, pencarian, aksi massal), [lot-cells](../../apps/web/src/components/cold-storage/lot-cells.tsx).
- **BFF:** `app/api/cold-storage` dan `.../bulk`; klien [cold-storage.ts](../../apps/web/src/lib/api/cold-storage.ts).
- **API Go:** `GET /v1/cold-storage` (`coldstorage.lihat`), `POST /v1/cold-storage/bulk` (`coldstorage.kelola` atau `logistik.kelola`).
- **Skema:** kunci primer dan kunci asing mengikuti [SKEMA-DATABASE.md](../SKEMA-DATABASE.md) sejak migrasi 000113: fasilitas **UUID**, lot dan transaksi **BIGINT**, telemetri BIGSERIAL (lihat [Kunci primer](#kunci-primer-000113-)). `cold_storage` dan `stok_cold_storage` (000002); 000007 menambah `suhu_aktual_c`, `kedaluwarsa_pada` (diisi dari `komoditas.shelf_life_hari`), `CHECK` volume dan `risiko_food_loss` 0–1, dua index; `Down` ada. Entity cocok.
- **Seeder** `internal/cold-storage/seeder`, terdaftar di registry (`cmd/seed/register_coldstorage.go`): `coldstorage.reference` (5 fasilitas, aman di produksi) dan `coldstorage.demo` (23 lot; tanggal digeser mengikuti `now`, ditolak di produksi). Idempoten, diuji termasuk lewat PostgreSQL (`seeder_db_test.go`, coverage 94%).
- **Tes:** usecase 100%, handler 93,9%, seeder 94%; integrasi memakai seeder (`master.reference`, `auth.demo`, fasilitas, lot), bukan data migrasi; kasus asap `/v1/cold-storage`; tes web lolos.

---

## 13-A — Rapikan seeder ke registry *(Fase P, urutan #11; tanpa migration)*

**Tujuan:** fitur ini ikut aturan seeder (lapis, dependensi, guard produksi) tanpa mengubah perilakunya.
**Bergantung pada:** F0-2, F0-4.

1. **MIG**: tidak ada.
2. **SEED**
   - [x] Pisahkan isi seeder: fasilitas (`cold_storage`) = `reference`; lot dan umur stok yang digeser mengikuti `now` = `demo`. Seeder sekarang **menggeser tanggal lot setiap dijalankan**, jadi tidak boleh jalan di produksi (guard `APP_ENV`).
   - [x] Pindahkan 2 komoditas yang disisipkannya ke seed master ([04-A](04-master-data.md)).
   - [x] Naikkan coverage seeder (28,6%) dengan tes `BuildFasilitas` dan `BuildStok`.
3. **API/FE**: tidak ada perubahan perilaku.
4. **TES**
   - [x] Setelah F0-2, tes integrasi tidak bergantung pada akun dari 000006: pakai fixture atau seeder `demo` ([01-A](01-auth.md)).
5. **DoD** — checklist [00 §4](00-cara-kerja.md); tes asap F0-5 mencakup `/v1/cold-storage`.

---

## 13-B — Telemetri dan transaksi stok *(Fase F, gelombang F-5)*

1. **MIG** (`000100_13b_cold_storage_telemetri_transaksi.sql`)
   - [x] `telemetri_suhu` (`fasilitas_id`, `waktu`, `suhu_c`, `kelembapan_pct`, `sumber`; unik per fasilitas+waktu); `suhu_aktual_c`, `kelembapan_aktual_pct`, `telemetri_pada` pada `cold_storage` diturunkan dari baris terbaru oleh aplikasi.
   - [ ] Partisi bulanan dan retensi `telemetri_suhu` (belum; tabel biasa, perlu sebelum volume sensor nyata). Kunci: `id` tetap BIGSERIAL, `fasilitas_id` uuid; indeks sudah ada (`UNIQUE (fasilitas_id, waktu)` untuk grafik per fasilitas, **BRIN** `idx_telemetri_suhu_waktu` untuk jendela waktu). Partisi harus mempertahankan `UNIQUE (fasilitas_id, waktu)` (memuat kolom partisi `waktu`) dan BRIN per partisi.
   - [x] `transaksi_stok` (`id` BIGINT identity, `lot_id`, `lot_baru_id` BIGINT → `stok_cold_storage` RESTRICT, `jenis` MASUK/KELUAR/PINDAH, `volume_ton`, asal/tujuan uuid → `cold_storage` RESTRICT, `oleh_id`, `waktu`, `alasan`); `stok_cold_storage.status` (AKTIF, PRIORITAS_JUAL, HABIS).
   - [x] `cold_storage`: batas alarm `suhu_min_c`/`suhu_maks_c` (NULL = target ± 2 °C). Kolom `aktif` tidak ditambah: status fasilitas sudah ada (`OPERASIONAL`/`PERAWATAN`/`RENCANA`). `Down` menghapus tabel telemetri/transaksi; fasilitas dan lot tidak tersentuh.
2. **SEED** (`coldstorage.demo`)
   - [x] Telemetri sintetis 48 jam (96 titik per fasilitas beroperasi, grid 30 menit; CS-04 punya lonjakan suhu yang sudah pulih) dan satu transaksi MASUK per lot demo. Jumlah baris tetap pada pengulangan (baris demo di luar jendela dibuang). Seeder `reference` tidak lagi menimpa `suhu_aktual_c`.
3. **API**
   - [x] `POST /v1/cold-storage/telemetri` (HTTP, idempoten, `coldstorage.kelola`; pembacaan di luar batas dilaporkan di respons dan audit).
   - [ ] Ingest MQTT dan alert otomatis ke EWS (menunggu rule engine [10-B](10-ews.md)).
   - [x] `POST /v1/cold-storage/transaksi` (MASUK/KELUAR/PINDAH, atomik dengan penguncian baris; 409 bila stok/kapasitas tak cukup atau fasilitas tak beroperasi; ID transaksi dan lot dari `RETURNING`), `PATCH /v1/cold-storage/fasilitas/:id` (kapasitas, batas alarm, status), `GET /v1/cold-storage/fasilitas/:id` (detail). `:id` fasilitas adalah **UUID atau kode `CS-…`** (`idx.ParamUUIDAtauKode`; selain itu 404).
   - [ ] Membuat/menghapus fasilitas lewat API (hanya seeder/migrasi sejauh ini). Rancangan: `POST /v1/cold-storage/fasilitas` (id UUID dibuat server; `kode` opsional dan UNIQUE, kode bentrok dijawab **409**), `DELETE /v1/cold-storage/fasilitas/:id` (`:id` UUID atau kode; **409** bila masih dirujuk buku besar `transaksi_stok` karena FK RESTRICT, sedangkan stok dan telemetri fasilitas ikut terhapus lewat CASCADE).
   - [x] Aksi `bulk` nyata: `tandai_prioritas_jual`, `buang` (hasil per lot di `rincian`). `pindahkan` massal belum ada (butuh tujuan per lot); pindah dilakukan per lot lewat `/transaksi`.
4. **FE**
   - [x] Panel detail fasilitas (riwayat suhu terhadap batas alarm, lot dengan peringatan kedaluwarsa, transaksi terbaru, formulir transaksi), lencana alarm suhu di kartu, aksi massal baru di tabel stok. BFF: `app/api/cold-storage/fasilitas/[id]` dan `.../transaksi`.
   - [x] "Ubah fasilitas" (`PATCH /v1/cold-storage/fasilitas/:id`: kapasitas, batas suhu min/maks, status) dan "Catat suhu manual" (`POST /v1/cold-storage/telemetri`, sumber MANUAL, waktu bawaan sekarang; hasil menampilkan duplikat dan alarm di luar batas) di panel detail, hanya untuk `coldstorage.kelola` (MANAJER_MUTU, OPS_AGREGATOR). Formulir memeriksa min < maks di klien; hanya kolom yang berubah yang dikirim. Daftar "Pembacaan terbaru" (5 titik, dengan sumber Sensor/Manual/Demo) menampilkan hasilnya. BFF: PATCH di `app/api/cold-storage/fasilitas/[id]/route.ts` dan `app/api/cold-storage/telemetri/route.ts` (CSRF + whitelist kolom). Komponen: [facility-manage.tsx](../../apps/web/src/components/cold-storage/facility-manage.tsx). Fasilitas berstatus RENCANA tidak menerima telemetri (tombol catat suhu disembunyikan). Mode mock menolak kedua aksi (perlu backend).
   - [x] Ekspor CSV hasil filter sudah ada di aksi massal tabel stok.
5. **TES**
   - [x] e2e `apps/web/e2e/cold-storage.spec.ts` (CS-03): manajer.mutu mengubah kapasitas/batas suhu (bertahan setelah muat ulang) lalu dipulihkan, mencatat suhu manual (tampil di "Pembacaan terbaru", alarm dilaporkan), duplikat dan alarm lewat API, strategis.djpb tanpa tombol dan 403 dari API. Pembersihan lewat SQL karena tidak ada endpoint hapus telemetri: baris uji MANUAL dihapus dan kolom `suhu_aktual_c`, `kelembapan_aktual_pct`, `telemetri_pada`, `suhu_min_c`, `suhu_maks_c` dipulihkan dari potret awal.
   - [x] Konkurensi (keluar serentak dari satu lot, pindah sebagian serentak, masuk serentak melewati kapasitas, pindah silang tanpa deadlock); buku besar konsisten dengan volume lot; migrasi Up/Down dengan data; kontrak OpenAPI; otorisasi per jenis transaksi; unit validasi usecase.

---

## 13-C — Analitik pascapanen *(Fase F, Tahap 3)*

- [ ] Model shelf-life (target RMSE ≤ 1 hari, PRD §12) dan prediksi food loss + rekomendasi penyelamatan/olahan turunan (tabel `prediksi_food_loss` bila model dibuat; lihat [11](11-prediksi-xai.md)).
- [ ] Rekomendasi lokasi cold storage baru ([12-C](12-peta-spasial.md)).
- [ ] Integrasi siklus **Surplus**: deteksi over-supply → arahkan ke cold storage → kartu intervensi ([14-C](14-intervensi.md)).
- [ ] **Computer vision QC** (P1, Fase 3): klasifikasi mutu dari foto.

---

## Kunci primer (000113, ✅)

Migrasi `000113_pk_coldstorage_spasial.sql`; rujukan [SKEMA-DATABASE.md](../SKEMA-DATABASE.md).

- **Fasilitas `cold_storage`: UUID** (`gen_random_uuid()`). ID lama `CS-01` menjadi `kode` (UNIQUE) dan UUID-nya `garda_uuid_stabil('cold_storage', id_lama)`, sama dengan `idx.UUIDStabil("cold_storage", "CS-01")` di seeder, sehingga basis data baru dan hasil migrasi bertemu di UUID yang sama.
- **Lot `stok_cold_storage` dan `transaksi_stok`: BIGINT** `GENERATED ALWAYS AS IDENTITY` (urut `masuk_pada` / `waktu`, lalu ID lama). `kode` = ID lama (`STK-CS01-01`, `TRX-DEMO-...`). Aplikasi tidak lagi membuat ID: MASUK dan PINDAH sebagian memakai ID lot dari `RETURNING`; lot baru diberi `kode` `STK-000042` (turunan ID, tidak bisa bentrok) karena UI menampilkannya; transaksi baru tanpa kode.
- **Telemetri `telemetri_suhu`: BIGSERIAL** dipertahankan; `fasilitas_id` uuid; indeks BRIN pada `waktu` dan `UNIQUE (fasilitas_id, waktu)` (melayani grafik per fasilitas).
- **ON DELETE:** buku besar **RESTRICT** (`transaksi_stok.fasilitas_asal_id`, `fasilitas_tujuan_id`, `lot_id`, `lot_baru_id`: fasilitas dan lot yang pernah bertransaksi tidak bisa dihapus); stok dan telemetri milik fasilitas **CASCADE**; `kabupaten_kode` dan `komoditas_kode` RESTRICT; `oleh_id` SET NULL.
- **Rute `:id`:**
  - `/v1/cold-storage/fasilitas/:id`: UUID atau kode (`idx.ParamUUIDAtauKode`, maks 64 karakter); UUID huruf besar dinormalkan; ID tidak ada atau salah bentuk dijawab **404** tanpa menyentuh kolom `uuid`. Body `fasilitas_id` dan `fasilitas_tujuan_id` (telemetri, transaksi) serta query `cold_storage_id` menerima UUID maupun kode; fasilitas yang tidak ada pada daftar menghasilkan daftar kosong.
  - `lot_id` dan `ids` aksi massal: BIGINT sebagai string desimal atau angka; kode lama `STK-…`, UUID, nol, negatif, atau pecahan dijawab **400**.
  - **Belum ada** rute detail transaksi. Bila dibuat: `GET /v1/cold-storage/transaksi/:id` memakai `idx.ParamIDAtauKode` (BIGINT atau kode `TRX-…`).
- **Respons:** `id` string (UUID atau angka desimal) dan `kode` pada fasilitas, lot, dan transaksi; transaksi memuat `fasilitas_asal_kode` dan `fasilitas_tujuan_kode`. UI menampilkan `kode ?? id`. Audit `cold_storage` memakai UUID (kiriman telemetri banyak fasilitas dicatat `N fasilitas` dengan daftar UUID di metadata); audit `stok_cold_storage` memakai ID lot.
- **Seeder:** fasilitas `ON CONFLICT (id)` pada UUID stabil; lot dan transaksi demo `ON CONFLICT (kode)`; transaksi demo menemukan ID lot lewat kode. Idempoten, juga pada basis data hasil migrasi.
- **View:** `mv_kpi_operasional` dibuat ulang oleh migrasi (nama, kolom, dan `uq_mv_kpi_operasional` sama).
- **Tes:** `schematest/pk_coldstorage_spasial_migration_test.go` (Up, Down, Up ulang dengan data lama, ON DELETE, identity, view), handler (alias, 404, 400), integrasi (ID dari `RETURNING`, kode lot, UUID dan kode setara), seeder idempoten.
