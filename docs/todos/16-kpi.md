# 16 — Indikator kinerja (KPI)

**Prioritas:** P1 (PRD §3.4) · **Status:** UI ✅ · Backend 🟡 (KPI berbasis model di 16-B) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [kpi-page.tsx](../../apps/web/src/components/kpi/kpi-page.tsx) di `/kpi` (izin `kpi.lihat`), 4 kelompok KPI sesuai blueprint. Sepenuhnya statis; tidak ada hook query.
- **Tidak ada:** modul Go, tabel, route. Jangan tertukar dengan `GET /v1/dashboard/operasional/kpi` (kartu angka di dashboard; [08-A](08-dashboard-operasional.md)).

---

## 16-A — KPI dari data yang sudah ada *(Fase P, urutan #16)*

**Tujuan:** `/kpi` tampil dari DB. KPI yang bisa dihitung sekarang dihitung dari data nyata/demo; KPI yang butuh model menunggu.
**Bergantung pada:** 06-A, 10-A, 13-A, 14-A.

1. **MIG**
   - [x] `kpi_definisi` (`kode` PK, `kelompok` enum 4 nilai [Data & Sistem / Logistik / Dampak Ekonomi / Tindak Lanjut], `nama`, `deskripsi`, `rumus`, `satuan`, `arah_baik`, `frekuensi`, `sumber_query`).
   - [x] `kpi_target` (`kpi_kode` FK, `wilayah_kode` NULL = semua, `periode_mulai`, `target`, `ambang_waspada`, `baseline`).
   - [x] `kpi_nilai` (`kpi_kode`, `wilayah_kode`, `periode` DATE, `nilai`, `dihitung_pada`, UNIQUE `(kpi_kode, wilayah_kode, periode)`); revisi = versi baru, bukan menimpa. Index `(kpi_kode, periode DESC)`.
2. **SEED**
   - [x] `reference`: `kpi_definisi` dan `kpi_target` untuk empat kelompok dengan target PRD §12 (MAPE harga ≤ 5,4%, F1 EWS ≥ 0,88, AUC-ROC ≥ 0,94, RMSE shelf-life ≤ 1 hari). **Baseline perlu keputusan bisnis**, jangan diisi sembarang.
   - [x] `demo`: nilai historis 12 bulan **dihitung oleh job** dari data demo, bukan angka tulis tangan.
3. **API**
   - [x] Modul `kpi` baru (`bash scripts/gen-module.sh kpi`): `GET /v1/kpi` dengan filter periode/wilayah dan riwayat per KPI.
   - [x] Job penghitung untuk KPI yang datanya sudah ada: kelengkapan produsen terverifikasi dan ketepatan data (dari harga), utilisasi cold storage, rasio rekomendasi AI yang dieksekusi (dari intervensi). KPI berbasis model (MAPE, F1, AUC, RMSE) ditandai "menunggu data" sampai 16-B. Dijalankan lewat `cmd/kpi` (cron) dan seeder `kpi.demo`.
   - [ ] Satukan kartu "KPI operasional" dashboard dengan `kpi_definisi` (subset), jangan dua tabel berbeda. *(ditunda: kartu dashboard operasional dikerjakan di 08-A)*
4. **FE**
   - [x] Tambah `api.kpi` di [endpoints.ts](../../apps/web/src/lib/api/endpoints.ts), key query, dan hook; ganti data statis di `kpi-page.tsx`; status capaian vs target dan tren beberapa periode.
5. **TES**: perhitungan tiap KPI pada data kecil yang hasilnya bisa dihitung tangan; izin dan kontrak respons.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 16-B — KPI berbasis model dan dampak *(Fase F, gelombang F-5)*

**Bergantung pada:** 11-B, 10-B.

Penghitung baru terdaftar di `kpi_definisi.sumber_query` (diterapkan oleh `make db/seed/reference`, tanpa mengubah definisi lain), dijalankan oleh job yang sama (`cmd/kpi`, `kpi.demo`). Tanpa dasar data hasilnya NULL → "menunggu data", bukan 0.

- [x] **MAPE prediksi harga** (`DATA-MAPE-HARGA`): proyeksi `prediksi_harga` yang tanggal targetnya jatuh pada bulan itu vs rata-rata harga konsumen tervalidasi pada hari dan kabupaten yang sama. Tabel `prediksi_evaluasi` tidak dibuat: evaluasi dihitung langsung dari proyeksi dan harga. Seed demo hanya punya proyeksi masa depan, jadi di demo KPI ini tetap "menunggu data" sampai tanggal targetnya lewat dan ada harga pembanding (sengaja tidak dibuatkan backtest palsu).
- [ ] **F1/AUC EWS**: butuh label kejadian (apa yang dihitung sebagai lonjakan sebenarnya) dan skor per alert; belum ada, jadi tetap menunggu. **RMSE shelf-life** dan **akurasi proyeksi produksi**: belum ada data realisasi masa simpan/produksi. **Penurunan food loss**: butuh baseline (keputusan bisnis) dan catatan kerugian terealisasi. **Indeks koordinasi**: butuh data survei.
- [x] **KPI dampak ekonomi**: `EKO-VOLATILITAS` (CV harga produsen, rata-rata atas komoditas × kabupaten dengan ≥ 5 entri tervalidasi), `EKO-MARGIN` ((konsumen − produsen) ÷ konsumen), `EKO-EFISIENSI-BIAYA` (penghematan rencana optimal 12-C terhadap rencana pembanding serakah untuk volume yang sama; kolom `optimasi_rute.biaya_pembanding`, migrasi `000103`). Efisiensi bukan penghematan terhadap biaya riil sebelum program: itu butuh baseline pemangku kepentingan.
- [x] **Logistik**: `LOG-VOLUME-AGREGASI` = volume MASUK cold storage per bulan (buku besar 13-B; pembelian di luar cold storage belum tercatat). `LOG-UTILISASI-CS` dan `LOG-RISIKO-FOODLOSS` kini dihitung dari buku besar `transaksi_stok` (stok historis nyata, bukan "lot tersimpan sampai kedaluwarsa"); instalasi tanpa satu pun transaksi tetap memakai pendekatan lama.
- [ ] Tetapkan baseline dan periode evaluasi bersama pemangku kepentingan (keputusan bisnis; kolom `baseline` tetap kosong).
- [x] Ekspor laporan KPI: `GET /v1/kpi/export?periode=&wilayah=` (CSV, tercatat `EKSPOR_DATA`) dan tombol "Unduh laporan CSV" di `/kpi`. Penjadwalan pengiriman laporan periodik (email/penjadwal) belum ada; `cmd/kpi` menghitung nilainya secara berkala.
