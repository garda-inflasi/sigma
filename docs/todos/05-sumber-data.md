
# 05 — Sumber data & akuisisi (API, scraping, pipeline)

**Prioritas:** P0/P1 (PRD §1.3–1.4) · **Status:** UI ✅ · Backend ✅ (kerangka ingestion + 5 konektor: PIHPS, BMKG, BPS, OpenWeather, Bapanas; alert EWS kegagalan berulang; uji integrasi & E2E lengkap) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [sumber-page.tsx](../../apps/web/src/components/sumber/sumber-page.tsx) di `/sumber-data`: pipeline medallion, mutu data, daftar API dan scraper.
- **API Go:** `GET /v1/sumber-data`, `GET /v1/sumber-data/:id`, `POST /v1/sumber-data/bulk`. Modul ini **hanya mencatat metadata sumber**.
- **Skema (sebelum 000115):** `sumber_data` di migrasi punya `id` VARCHAR, `jenis`, `jadwal_cron`, `status`, `terakhir_jalan`, `keberhasilan_7h`, `latensi_ms`, `robots_txt_patuh`, `atribusi`. **Entity Go tidak cocok:** `id` `int64`, dan kolom `dibuat_pada`, `frekuensi_update`, `tipe`, `url` tidak ada di migrasi. Tabel kosong. `cuaca_harian` (UNIQUE `kabupaten_kode, tanggal, sumber`) ada tanpa rujukan kode.
- **Yang tidak ada:** kode yang mengambil data. Tidak ada klien BPS/BI/BMKG/PIHPS/OpenWeather, scraper, scheduler, maupun ETL.
- **Detail segar (✅, terhubung):** [sumber-detail.tsx](../../apps/web/src/components/sumber/sumber-detail.tsx) memuat `GET /v1/sumber-data/:id` saat panel dibuka (baris daftar hanya dipakai selama memuat atau bila gagal) dan menampilkan "Informasi sumber": jenis, cakupan, cron WIB mentah, terakhir dijalankan, keberhasilan 7 hari, latensi, kepatuhan robots.txt, URL dasar, atribusi. Setelah "Jalankan sekarang" atau selesainya run, cache `sumber-data` (daftar dan detail) disegarkan. `konfigurasi` sumber (bisa memuat kunci) sengaja tidak ada di respons API. Belum ada: waktu "terakhir sukses" per sumber di DTO (hanya dipakai pipeline).

### Kunci primer (000115, ✅)

- **`sumber_data.id` kini UUID** (`garda_uuid_stabil('sumber_data', id_lama)` = `idx.UUIDStabil("sumber_data", kode)`); ID lama (`SRC-BI`, …) disimpan di `kode` (UNIQUE, NOT NULL) dan tetap menjadi kode tampilan. Bila kelak sumber baru dibuat lewat form, `kode` isian itu UNIQUE dan bentrok dijawab 409.
- **Rute `:id`** (`GET /v1/sumber-data/:id`, `/:id/eksekusi`, `/:id/sampel`, `POST /:id/jalankan`) memakai `idx.ParamUUIDAtauKode` (maks 64 karakter): UUID, atau kode sebagai alias sehingga tautan lama seperti `/v1/sumber-data/SRC-BPS` tetap berlaku. Teks yang bukan UUID dibaca sebagai kode; kode yang tidak ada atau terlalu panjang dijawab **404**, tidak pernah 500. `POST /v1/sumber-data/bulk` menerima UUID maupun kode di `ids` (maks 64 karakter per entri; entri yang tidak ada dihitung `gagal`; dua ejaan sumber yang sama dihitung satu). Respons memuat `id` (UUID) dan `kode`; audit `JALANKAN_SUMBER` mencatat UUID di `entitas_id` dan kode di metadata.
- **Konektor, CLI, dan seeder merujuk sumber lewat kode.** `Konektor.Kode()` (bukan ID) mengenali konektor; `gardactl ingest` menerima kode atau UUID; engine membaca baris sumber lewat kode/UUID lalu memakai UUID untuk riwayat. Seeder memakai `sumberseeder.IDSumber(kode)` dan `ON CONFLICT (kode)`; pemetaan seed memakai UUID yang sama, jadi basis data hasil migrasi dan basis data baru tidak berbeda.
- **Turunan:** `eksekusi_sumber` (BIGSERIAL tetap) dan `data_mentah` (BIGSERIAL tetap) memakai `sumber_id` uuid → **CASCADE**; `data_mentah.eksekusi_id` → CASCADE; `pemetaan_sumber` (KOMPOSIT `(sumber_id, jenis, alias)`) → CASCADE. `harga_eksternal` kini **BIGINT `id`** dengan `UNIQUE (sumber_id, komoditas_kode, kabupaten_kode, tanggal, tipe)` (target upsert ingestion) dan `sumber_id` → **RESTRICT** (bukti asal data: sumber yang punya harga dinonaktifkan lewat jeda, bukan dihapus); `komoditas_kode`/`kabupaten_kode` RESTRICT; `data_mentah_id`/`eksekusi_id` **SET NULL**. `cuaca_harian` kini **BIGINT** (ID `CUA-…` aplikasi dihapus; `kode` = ID lama) dengan UNIQUE `(kabupaten_kode, tanggal, sumber)` sebagai target upsert; `eksekusi_id`/`data_mentah_id` SET NULL.
- **Indeks:** BRIN `data_mentah(diambil_pada)` dan `harga_eksternal(tanggal)`; B-tree untuk setiap FK (`harga_eksternal`: `kabupaten_kode`, `data_mentah_id`, `eksekusi_id`; `cuaca_harian`: `eksekusi_id`, `data_mentah_id`, `tanggal`); materialized view `gold_harga_harian` tidak berubah (tidak membaca PK atau `sumber_id`).

---

## 05-A — Katalog sumber dari DB *(Fase P, urutan #13)*

**Tujuan:** `/sumber-data` tampil dari database. Belum ada pengambilan data nyata.
**Bergantung pada:** 04-A.

1. **MIG**
   - [x] Selaraskan dengan entity (D1: migrasi menang): tambah hanya yang dibutuhkan, mis. `url_dasar`, `konfigurasi JSONB`. Entity memakai `id` string; buang kolom entity yang tidak ada.
2. **SEED**
   - [x] `reference`: katalog sumber (BPS, BI, BMKG, Bapanas/PIHPS, DJPb, OpenWeather, scraper portal daerah) dengan `jadwal_cron`, `atribusi`, `robots_txt_patuh`. Status awal **jujur** (belum dikonfigurasi/nonaktif), bukan `SEHAT` sebelum pernah berjalan.
   - [x] `demo`: `terakhir_jalan`, `keberhasilan_7h`, `latensi_ms` agar tampilan sama dengan `mockSumber`.
3. **API**: selaraskan entity/DTO/handler; `bulk` untuk jeda/aktifkan (mengubah metadata dulu; efek nyata di 05-B).
4. **FE**: `useSumberData` sudah ada; cocokkan bentuk dengan `SumberData`.
5. **TES**: handler + kontrak; hapus `sumber-data` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 05-B — Framework ingestion + sumber pertama (PIHPS) *(Fase F, gelombang F-3)*

**Tujuan:** satu sumber benar-benar mengalir ke database secara terjadwal, idempoten, dan tercatat.
**Bergantung pada:** 05-A, 06-A.

1. **MIG** (`000080`)
   - [x] `eksekusi_sumber` (+ `pemicu`, `latensi_ms`, rincian baris ditolak, `ringkasan` JSONB; indeks unik parsial: satu run BERJALAN per sumber). `keberhasilan_7h` dan `latensi_ms` dihitung darinya.
   - [x] **Bronze** `data_mentah` (payload JSONB, `checksum`, UNIQUE `(sumber_id, checksum)`).
   - [x] `pemetaan_sumber` (alias komoditas/wilayah → kode master; alias yang sengaja diabaikan ditandai eksplisit).
   - [x] `sumber_data.konfigurasi` JSONB dan silver `harga_eksternal` (dimajukan dari 05-C karena PIHPS perlu tujuan tulis).
   - [ ] ~~`kredensial_sumber` terenkripsi~~: **tidak dibuat**; kunci API dibaca dari lingkungan (`BPS_API_KEY`), tidak disimpan di DB.
2. **SEED**: `ingestion.reference` memetakan alias komoditas PIHPS → kode master (insert-only agar pemetaan operator tidak tertimpa).
3. **API/Worker**
   - [x] Engine (`internal/ingestion`): `gardactl ingest`, penjadwal opsional di `cmd/api` (`INGEST_SCHEDULER=on`), retry backoff, rate limiting per host, circuit breaker, kegagalan tercatat per run.
   - [x] Konektor PIHPS (**belum terverifikasi terhadap endpoint nyata**); kepatuhan robots.txt; atribusi tetap di katalog.
   - [x] Jeda/lanjutkan berefek nyata (penjadwal dan run manual menolak sumber dijeda); `POST /v1/sumber-data/:id/jalankan` (`sumber.kelola`, diaudit) menggantikan "jalankan" di bulk.
   - [x] Klien Bapanas (`SRC-BAPANAS`): konektor harga pangan strategis per kabupaten/kota.
4. **FE**: panel detail sumber (riwayat eksekusi, galat, alias belum dipetakan, sampel bronze), tombol "Jalankan sekarang" (`sumber.kelola`). Edit jadwal belum.
5. **TES**: parser PIHPS/BMKG/BPS/OWM/Bapanas dengan fixture, `httptest` untuk handler delivery/http & fetcher/robots/retry/breaker, engine dengan Postgres (dua kali jalan tidak menggandakan data), endpoint 202/403/409/422, CLI `gardactl ingest`. E2E Playwright `apps/web/e2e/sumber-data.spec.ts`.

---

## 05-C — Silver/gold dan sumber lain *(Fase F, gelombang F-3)*

1. **MIG** (`000081`)
   - [x] **Silver** `harga_eksternal` (dibuat di 05-B) dan `cuaca_harian` (lineage `eksekusi_id`, `data_mentah_id`; UNIQUE yang sudah ada menjamin upsert).
   - [x] **Gold** `gold_harga_harian`: materialized view harga konsumen harian dari sisi lapangan (`agregat_harga_harian`) dan eksternal, beserta `selisih_pct`; disegarkan oleh `dashboard/refresher`.
   - [ ] Partisi bulanan `data_mentah` dan `harga_eksternal` + retensi bronze: **belum** (volume sekarang kecil). PK tabel berpartisi harus memuat kolom partisi (`(diambil_pada, id)` dan `(tanggal, id)`), dan `uq_harga_eksternal_alami` sudah memuat `tanggal` sehingga tetap bisa menjadi target upsert; `data_mentah_id`/`eksekusi_id` pada silver tetap SET NULL, jadi retensi bronze tidak menghapus silver.
2. **API/Worker**
   - [x] BMKG (prakiraan 3-jaman diringkas per hari; hari parsial ditolak dan dilaporkan), BPS (IHK/inflasi dan andil; baris resmi menggantikan baris contoh `DEMO` dan tidak menimpa sumber resmi lain).
   - [x] Validasi silang lapangan vs eksternal (kolom `selisih_pct` di gold, ukuran `akurasi`).
   - [x] OpenWeather (`SRC-OWM`) dan Bapanas (`SRC-BAPANAS`): konektor diimplementasikan dan terdaftar di `ingestion.Konektor()`.
   - [ ] Id variabel BPS dan id wilayah PIHPS tidak diketahui; operator mengisinya lewat `gardactl ingest konfigurasi|pemetaan`. Titik pantau BMKG (adm4) di seed adalah tebakan pola kode, belum terverifikasi.
3. **FE**: kartu pipeline dan mutu data di `/sumber-data` dari `GET /v1/sumber-data/pipeline` (kelengkapan, akurasi, ketepatan waktu, konsistensi dihitung dari eksekusi nyata; `—` bila belum ada).
4. **TES**: parser BMKG/BPS/OWM/Bapanas dengan fixture, ingestion idempoten, gold, mutu, seeder demo tidak menimpa data nyata, unit test `delivery/http`, dan E2E Playwright `sumber-data.spec.ts`.
5. Data historis harga/cuaca masuk lewat job impor/backfill, **bukan** seeder. Data lineage per record ([03-C](03-audit.md)); alert saat sumber gagal berulang ke [EWS](10-ews.md): **selesai** (`PeriksaAlertKegagalan` menerbitkan alert `ANOMALI_DATA` saat 3x gagal beruntun dan menyelesaikannya saat pulih).
