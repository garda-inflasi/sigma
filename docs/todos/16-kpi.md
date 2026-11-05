# 16 — Indikator kinerja (KPI)

**Prioritas:** P1 (PRD §3.4) · **Status:** UI ✅ · Backend ⬜ · Terhubung ⬜ · Skema & seed ⬜ · Tes ⬜
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [kpi-page.tsx](../../apps/web/src/components/kpi/kpi-page.tsx) di `/kpi` (izin `kpi.lihat`), 4 kelompok KPI sesuai blueprint. Sepenuhnya statis; tidak ada hook query.
- **Tidak ada:** modul Go, tabel, route. Jangan tertukar dengan `GET /v1/dashboard/operasional/kpi` (kartu angka di dashboard; [08-A](08-dashboard-operasional.md)).

---

## 16-A — KPI dari data yang sudah ada *(Fase P, urutan #16)*

**Tujuan:** `/kpi` tampil dari DB. KPI yang bisa dihitung sekarang dihitung dari data nyata/demo; KPI yang butuh model menunggu.
**Bergantung pada:** 06-A, 10-A, 13-A, 14-A.

1. **MIG**
   - [ ] `kpi_definisi` (`kode` PK, `kelompok` enum 4 nilai [Data & Sistem / Logistik / Dampak Ekonomi / Tindak Lanjut], `nama`, `deskripsi`, `rumus`, `satuan`, `arah_baik`, `frekuensi`, `sumber_query`).
   - [ ] `kpi_target` (`kpi_kode` FK, `wilayah_kode` NULL = semua, `periode_mulai`, `target`, `ambang_waspada`, `baseline`).
   - [ ] `kpi_nilai` (`kpi_kode`, `wilayah_kode`, `periode` DATE, `nilai`, `dihitung_pada`, UNIQUE `(kpi_kode, wilayah_kode, periode)`); revisi = versi baru, bukan menimpa. Index `(kpi_kode, periode DESC)`.
2. **SEED**
   - [ ] `reference`: `kpi_definisi` dan `kpi_target` untuk empat kelompok dengan target PRD §12 (MAPE harga ≤ 5,4%, F1 EWS ≥ 0,88, AUC-ROC ≥ 0,94, RMSE shelf-life ≤ 1 hari). **Baseline perlu keputusan bisnis**, jangan diisi sembarang.
   - [ ] `demo`: nilai historis 12 bulan **dihitung oleh job** dari data demo, bukan angka tulis tangan.
3. **API**
   - [ ] Modul `kpi` baru (`bash scripts/gen-module.sh kpi`): `GET /v1/kpi` dengan filter periode/wilayah dan riwayat per KPI.
   - [ ] Job penghitung untuk KPI yang datanya sudah ada: kelengkapan produsen terverifikasi dan ketepatan data (dari harga), utilisasi cold storage, rasio rekomendasi AI yang dieksekusi (dari intervensi). KPI berbasis model (MAPE, F1, AUC, RMSE) ditandai "menunggu data" sampai 16-B.
   - [ ] Satukan kartu "KPI operasional" dashboard dengan `kpi_definisi` (subset), jangan dua tabel berbeda.
4. **FE**
   - [ ] Tambah `api.kpi` di [endpoints.ts](../../apps/web/src/lib/api/endpoints.ts), key query, dan hook; ganti data statis di `kpi-page.tsx`; status capaian vs target dan tren beberapa periode.
5. **TES**: perhitungan tiap KPI pada data kecil yang hasilnya bisa dihitung tangan; izin dan kontrak respons.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 16-B — KPI berbasis model dan dampak *(Fase F, gelombang F-5)*

**Bergantung pada:** 11-B, 10-B.

- [ ] Hitung KPI akurasi dari `prediksi_evaluasi` (MAPE), kinerja EWS (F1, AUC), shelf-life (RMSE).
- [ ] KPI dampak ekonomi: volatilitas harga, margin, efisiensi rute ([12-C](12-peta-spasial.md)).
- [ ] Tetapkan baseline dan periode evaluasi bersama pemangku kepentingan; ekspor laporan KPI periodik.
