# 05 — Sumber data & akuisisi (API, scraping, pipeline)

**Prioritas:** P0/P1 (PRD §1.3–1.4) · **Status:** UI ✅ · Backend 🟡 (kerangka ingestion + PIHPS selesai di 05-B; konektor belum diverifikasi ke layanan nyata) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
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

1. **MIG**
   - [ ] **Silver** `harga_eksternal` (`sumber_id`, `komoditas_kode`, `kabupaten_kode`/`pasar_id`, `tanggal`, `harga`, `satuan`, UNIQUE gabungan).
   - [ ] **Gold** `agregat_harga_harian` menggabungkan data lapangan dan eksternal (dibuat di [08-A](08-dashboard-operasional.md); perluas di sini).
   - [ ] Partisi bulanan `data_mentah` dan `harga_eksternal` + retensi bronze.
2. **API/Worker**: BMKG/OpenWeather (isi `cuaca_harian`, UNIQUE-nya menjamin upsert idempoten), BPS, BI, DJPb; **validasi silang** lapangan vs API vs scraping; normalisasi.
3. **FE**: data quality dashboard sesuai PRD (kelengkapan, akurasi, ketepatan waktu, konsistensi) dari data nyata.
4. **TES**: parser tiap sumber, validasi silang, idempotensi.
5. Data historis harga/cuaca masuk lewat job impor/backfill, **bukan** seeder. Data lineage per record ([03-C](03-audit.md)); alert saat sumber gagal berulang ke [EWS](10-ews.md).
