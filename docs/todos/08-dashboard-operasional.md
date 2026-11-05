# 08 — Dashboard operasional

**Prioritas:** P0 (Fase 1 "Dashboard dasar") · **Status:** UI ✅ · Backend 🟡 · Terhubung 🟡 · Skema & seed ⬜ · Tes ⬜
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** `/operasional` memakai [kpi-grid](../../apps/web/src/components/dashboard/kpi-grid.tsx), [price-heatmap](../../apps/web/src/components/dashboard/price-heatmap.tsx), [ops-map](../../apps/web/src/components/dashboard/ops-map.tsx), [field-activity](../../apps/web/src/components/dashboard/field-activity.tsx), feed EWS, [trend-chart](../../apps/web/src/components/charts/trend-chart.tsx), antrean validasi. Polling 30 detik.
- **API Go (`dashboard`):** `GET /v1/dashboard/operasional/{kpi,heatmap}`, `GET /v1/siklus/aktif`, `GET /v1/harga/tren`.
- **API Go (`analytics`, MongoDB):** `/v1/operations/*`, `/v1/regions/status-map`, `/v1/field-activities/progress`. **Tidak dipanggil web.**
- **Kritis:** repository dashboard ([repo.go](../../apps/api/internal/dashboard/repository/postgres/repo.go)) memakai SQL mentah pada tabel `kpi_operasional` dan `heatmap_sel` yang **tidak ada di migrasi**. Entity siklus memakai tabel `siklus_ekonomi` dan tipe `siklus_ekonomi`; migrasi hanya punya enum `siklus` dan tidak punya tabelnya. Jadi `/v1/dashboard/operasional/*` dan `/v1/siklus/aktif` (warna ambient seluruh UI) error pada DB hasil migrasi.

---

## 08-A — KPI, heatmap, dan siklus dari data nyata *(Fase P, urutan #8)*

**Tujuan:** dashboard tampil dari agregat atas tabel harga/EWS/tugas (bukan tabel statis), dan siklus aktif tersedia.
**Bergantung pada:** 06-A (data harga). Putuskan D4 (modul `analytics`) sebelum mulai.

1. **MIG**
   - [x] `agregat_harga_harian` (komoditas, kabupaten, tanggal, rata-rata, min, maks, n) sebagai materialized view atau tabel dengan UNIQUE index (agar `REFRESH … CONCURRENTLY` bisa dipakai).
   - [x] `mv_heatmap_harga` (dari agregat; kategori berdasarkan deviasi terhadap `harga_acuan`) dan `mv_kpi_operasional` (dari `entri_harga`, `tugas_lapangan`, `ews_alert`).
   - [x] `target_kpi` (`indikator`, `target`, `periode`) untuk menghitung `capaian_pct`.
   - [x] **`siklus_ekonomi`** (`id`, `wilayah_kode` FK, `siklus` bertipe enum `siklus` yang sudah ada, `mulai_pada`, `akhir_pada`) + index parsial `WHERE akhir_pada IS NULL`. Entity memakai enum `siklus`.
2. **SEED**
   - [x] `reference`: `target_kpi` awal.
   - [x] `demo`: satu baris `siklus_ekonomi` aktif `TEKANAN` sejak 2026-09-27 (sama dengan mock). Agregat/MV **tidak di-seed langsung**: dihitung dari seed harga lalu di-refresh (urutan: harga → refresh MV).
3. **API**
   - [x] Tulis ulang query dashboard ke MV; siklus entity ke tabel baru; selaraskan DTO dengan `KpiOperasional`/`Heatmap`.
   - [x] Job refresh MV terjadwal, ditambah refresh setelah publikasi harga (UI polling 30 detik).
   - [x] Keputusan D4: hapus atau gabungkan route `analytics` yang menduplikasi (`/v1/operations/*`).
4. **FE**
   - [ ] `api.operasional.*` dan `api.siklus` sudah memanggil API; cocokkan bentuk. Ganti `USERS` statis di `field-activity` dengan nama petugas dari API (marker peta menyusul di [12-A](12-peta-spasial.md)).
5. **TES**
   - [ ] Setelah `reference + demo + refresh`, KPI dan heatmap mengembalikan baris dan cocok dengan tipe web; `EXPLAIN` pada data ratusan ribu baris; hapus entri dashboard/siklus dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 08-B — Filter, cakupan baris, dan penentuan siklus nyata *(Fase F)*

1. **MIG**: tabel aturan penentuan siklus bila aturannya dikonfigurasi (ambang EWS + stok).
2. **API**
   - [ ] Filter wilayah/komoditas/periode pada KPI dan heatmap; row scope Koordinator/Agregator ([02-B](02-pengguna-rbac.md)).
   - [ ] Endpoint ringkasan aktivitas per petugas/kabupaten (`field-activity`); stok real-time, agregasi produsen, pergerakan logistik (PRD §3.1).
   - [ ] **Penentuan siklus otomatis** dari ambang EWS dan stok (kaitan [10-B](10-ews.md), [14-C](14-intervensi.md)); sekarang hanya membaca baris.
   - [ ] Cache untuk agregasi berat.
3. **FE**: peta memakai token Mapbox bila ada, fallback SVG; keadaan kosong dan galat untuk tiap kartu; feed EWS memakai SSE ([10-C](10-ews.md)).
4. **TES**: usecase/repo dashboard, kontrak, performa query.
