# 05 — Sumber data & akuisisi (API, scraping, pipeline)

**Prioritas:** P0/P1 (PRD §1.3–1.4) · **Status:** UI ✅ · Backend 🟡 (katalog; pengambilan data di 05-B) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
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

1. **MIG**
   - [ ] `eksekusi_sumber` (`id`, `sumber_id` FK, `mulai`, `selesai`, `status`, `baris_diambil`, `baris_ditolak`, `galat`, `durasi_ms`). `keberhasilan_7h` dan `latensi_ms` dihitung darinya.
   - [ ] **Bronze** `data_mentah` (`sumber_id`, `diambil_pada`, `tipe_konten`, `payload` JSONB atau `lokasi_objek`, `checksum`, UNIQUE `(sumber_id, checksum)`).
   - [ ] `pemetaan_sumber` (alias komoditas/wilayah/satuan per sumber → kode master).
   - [ ] `kredensial_sumber` terenkripsi (kunci dari env, bukan di DB).
2. **SEED**: pemetaan awal PIHPS → kode master (`reference`).
3. **API/Worker**
   - [ ] Layanan ingestion terpisah dari API (worker/cron): scheduler, retry backoff, rate limiting, circuit breaker, metrik kegagalan.
   - [ ] Klien PIHPS/Bapanas; kepatuhan robots.txt dan atribusi.
   - [ ] Aksi `bulk` (jalankan sekarang, jeda, aktifkan) benar-benar menggerakkan worker.
4. **FE**: halaman detail sumber (riwayat eksekusi, log galat, sampel data), tombol "jalankan sekarang", edit jadwal (izin `sumber.kelola`).
5. **TES**: klien dengan `httptest`, parser dengan fixture HTML/JSON tersimpan, **ingestion dua kali tidak menggandakan data**.

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
