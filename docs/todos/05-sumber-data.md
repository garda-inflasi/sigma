# 05 — Sumber data & akuisisi (API, scraping, pipeline)

**Prioritas:** P0/P1 (PRD §1.3–1.4) · **Status:** UI ✅ · Backend 🟡 (kerangka ingestion + konektor PIHPS, BMKG, BPS selesai; belum diverifikasi ke layanan nyata) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [sumber-page.tsx](../../apps/web/src/components/sumber/sumber-page.tsx) di `/sumber-data`: pipeline medallion, mutu data, daftar API dan scraper.
- **API Go:** `GET /v1/sumber-data`, `GET /v1/sumber-data/:id`, `POST /v1/sumber-data/bulk`. Modul ini **hanya mencatat metadata sumber**.
- **Skema:** `sumber_data` di migrasi punya `id` VARCHAR, `jenis`, `jadwal_cron`, `status`, `terakhir_jalan`, `keberhasilan_7h`, `latensi_ms`, `robots_txt_patuh`, `atribusi`. **Entity Go tidak cocok:** `id` `int64`, dan kolom `dibuat_pada`, `frekuensi_update`, `tipe`, `url` tidak ada di migrasi. Tabel kosong. `cuaca_harian` (UNIQUE `kabupaten_kode, tanggal, sumber`) ada tanpa rujukan kode.
- **Yang tidak ada:** kode yang mengambil data. Tidak ada klien BPS/BI/BMKG/PIHPS/OpenWeather, scraper, scheduler, maupun ETL.

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
   - [x] Engine (`internal/ingestion`): `cmd/ingest`, penjadwal opsional di `cmd/api` (`INGEST_SCHEDULER=on`), retry backoff, rate limiting per host, circuit breaker, kegagalan tercatat per run.
   - [x] Konektor PIHPS (**belum terverifikasi terhadap endpoint nyata**); kepatuhan robots.txt; atribusi tetap di katalog.
   - [x] Jeda/lanjutkan berefek nyata (penjadwal dan run manual menolak sumber dijeda); `POST /v1/sumber-data/:id/jalankan` (`sumber.kelola`, diaudit) menggantikan "jalankan" di bulk.
   - [ ] Klien Bapanas (scraper) belum ada.
4. **FE**: panel detail sumber (riwayat eksekusi, galat, alias belum dipetakan, sampel bronze), tombol "Jalankan sekarang" (`sumber.kelola`). Edit jadwal belum.
5. **TES**: parser PIHPS dengan fixture, `httptest` untuk fetcher/robots/retry/breaker, engine dengan Postgres (dua kali jalan tidak menggandakan data), endpoint 202/403/409/422, CLI.

---

## 05-C — Silver/gold dan sumber lain *(Fase F, gelombang F-3)*

1. **MIG** (`000081`)
   - [x] **Silver** `harga_eksternal` (dibuat di 05-B) dan `cuaca_harian` (lineage `eksekusi_id`, `data_mentah_id`; UNIQUE yang sudah ada menjamin upsert).
   - [x] **Gold** `gold_harga_harian`: materialized view harga konsumen harian dari sisi lapangan (`agregat_harga_harian`) dan eksternal, beserta `selisih_pct`; disegarkan oleh `dashboard/refresher`.
   - [ ] Partisi bulanan `data_mentah` dan `harga_eksternal` + retensi bronze: **belum** (volume sekarang kecil).
2. **API/Worker**
   - [x] BMKG (prakiraan 3-jaman diringkas per hari; hari parsial ditolak dan dilaporkan), BPS (IHK/inflasi dan andil; baris resmi menggantikan baris contoh `DEMO` dan tidak menimpa sumber resmi lain). **Keduanya belum terverifikasi terhadap endpoint nyata.**
   - [x] Validasi silang lapangan vs eksternal (kolom `selisih_pct` di gold, ukuran `akurasi`).
   - [ ] OpenWeather, BI (selain PIHPS), DJPb, Bapanas/scraper: belum ada konektor.
   - [ ] Id variabel BPS dan id wilayah PIHPS tidak diketahui; operator mengisinya lewat `cmd/ingest konfigurasi|pemetaan`. Titik pantau BMKG (adm4) di seed adalah tebakan pola kode, belum terverifikasi.
3. **FE**: kartu pipeline dan mutu data di `/sumber-data` dari `GET /v1/sumber-data/pipeline` (kelengkapan, akurasi, ketepatan waktu, konsistensi dihitung dari eksekusi nyata; `—` bila belum ada).
4. **TES**: parser BMKG/BPS dengan fixture, ingestion idempoten, gold, mutu, seeder demo tidak menimpa data nyata.
5. Data historis harga/cuaca masuk lewat job impor/backfill, **bukan** seeder. Data lineage per record ([03-C](03-audit.md)); alert saat sumber gagal berulang ke [EWS](10-ews.md): belum.
