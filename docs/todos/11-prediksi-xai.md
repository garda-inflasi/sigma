# 11 — Prediksi harga & XAI

**Prioritas:** P0 (PRD §2.1) · **Status:** UI ✅ · Backend 🟡 (membaca tabel; model nyata di 11-B) · Terhubung ✅ · Skema & seed ✅ · Tes ✅ (11-A selesai)
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [prediksi-page.tsx](../../apps/web/src/components/prediksi/prediksi-page.tsx) di `/prediksi` (panel XAI, simulasi skenario, proyeksi semua komoditas, registri model) dan [xai-panel](../../apps/web/src/components/eksekutif/xai-panel.tsx).
- **API Go (`prediksi`):** `GET /v1/prediksi`, `/:kode`, `/:kode/export` (izin `prediksi.lihat`); usecase hanya membaca baris `ProyeksiXai`. *(Riwayat; kini membaca `prediksi_harga`, lihat 11-A.)*
- **API Go (`analytics`, MongoDB, tidak dipakai web):** `/v1/predictions/*`, `/v1/models/registry` (izin `prediksi.baca`, yatim).
- **Halaman `/prediksi` dan `/policy-brief` memanggil `mockXai` langsung**, jadi tetap memakai data palsu walau backend aktif.
- **Skema:** entity `ProyeksiXai` memetakan tabel **`proyeksi_xai` yang tidak ada**. Migrasi punya `prediksi_harga` (`nilai`, `bawah`, `atas`, `horizon_hari`, `model_versi`, `faktor` JSONB) yang pas untuk XAI + interval kepercayaan. `model_registry` (Postgres, + `algorithm`, `metrics`, `deployed_at`, `feature_importance` dari 000005) hanya dirujuk entity `analytics`; seeder MongoDB juga membuat koleksi `model_registry` (duplikat). Semua kosong.
- **Yang tidak ada:** model apa pun (pelatihan, inferensi, skenario).

---

## 11-A — Proyeksi dan registri dari DB, buang `mockXai` *(Fase P, urutan #17)*

**Tujuan:** `/prediksi`, panel XAI di eksekutif, dan policy brief tampil dari database (data demo).
**Bergantung pada:** 06-A. Putuskan D4 (registri Postgres vs MongoDB) sebelum mulai.

1. **MIG**
   - [x] Migrasi `000051`: `prediksi_harga` mendapat `UNIQUE (komoditas_kode, kabupaten_kode, tanggal_target, horizon_hari, model_versi)`, FK komposit ke `model_registry (komoditas_kode, versi)`, `CHECK bawah <= nilai <= atas`, kolom `harga_dasar` (pembagi persen perubahan) dan `sumber` (`MODEL`|`DEMO`); `model_registry.sumber` dan indeks unik satu model `PRODUKSI` per komoditas.
   - [x] Entity diarahkan ke `prediksi_harga` dan `model_registry` (tidak membuat tabel baru). Registri hidup di satu tempat (Postgres, D4); duplikat Mongo sudah hilang bersama modul `analytics`.
2. **SEED** (`demo`)
   - [x] `prediksi.demo` (`DependsOn master.reference`, `After harga.demo`): per komoditas satu model `PRODUKSI` (v2.3.1) dan satu `KANDIDAT` (v2.4.0-rc1), semua `sumber = DEMO`; proyeksi horizon 7/14/30 hari untuk semua komoditas di Kota Banda Aceh dan komoditas pilot di 22 kabupaten lain (240 baris), `faktor` ala SHAP yang jumlahnya sama dengan perubahan proyeksi. Id baris tetap (tanpa tanggal): menjalankan ulang di hari lain menggeser `tanggal_target`, tidak menambah baris. Produksi tidak di-seed.
3. **API**
   - [x] `GET /v1/prediksi?kabupaten=&horizon=` (ringkasan semua komoditas), `/:kode` (seri riwayat + proyeksi, faktor, narasi, rekomendasi, model), `/:kode/export` (CSV titik koma ber-BOM, satu baris per titik seri; `proyeksi_<kode>.csv`), dan `GET /v1/prediksi/registri`. Hanya model `PRODUKSI` yang dipakai. Riwayat aktual dibaca dari `entri_harga` (konsumen, tervalidasi/terpublikasi) bila ada. Narasi dan rekomendasi dari templat aturan, bukan LLM.
   - [x] Izin yatim `prediksi.baca` hilang bersama modul `analytics`; semua route memakai `prediksi.lihat`.
4. **FE**
   - [x] `prediksi-page.tsx` dan `xai-panel.tsx` memakai `useProyeksiRingkasan`, `useModelPrediksi`, `useXai` (API nyata, mapper `lib/api/prediksi.ts`); tidak ada `mockXai` di komponen (hanya di lapisan mode mock). Data seed berlabel "Data demo".
   - [x] "Simulasi skenario" tetap dihitung di klien dari perubahan proyeksi cabai dan diberi label koefisien contoh. Aksi "Promosikan"/"Arsipkan" di registri dihapus dari UI karena tidak tersimpan; kembali di 11-C dengan endpoint dan izin `model.kelola`.
5. **TES**: [x] tiap komoditas pilot punya proyeksi demo di semua kabupaten dengan `bawah <= nilai <= atas`; jumlah `faktor` = perubahan proyeksi; seeder idempoten lintas hari; endpoint 200/401/403/400/404 pada DB hasil migrasi; `prediksi` keluar dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md). Khusus: tidak ada `import … mockXai` yang tersisa di komponen.

---

## 11-B — Model prediksi pertama *(Fase F, gelombang F-4)* — baseline selesai

**Tujuan:** proyeksi dihasilkan model, bukan seed. Target PRD §12: MAPE ≤ 5,4%. **Belum diklaim tercapai:** baseline di bawah diukur jujur dengan backtest, dan akurasi sungguhan baru terlihat dari MAPE realisasi setelah proyeksi jatuh tempo.
**Bergantung pada:** 11-A, 06-A, 05-C (cuaca).

1. **MIG**
   - [x] `000090`: `prediksi_evaluasi` (`prediksi_id` PK/FK, `realisasi`, `galat_pct`, `dievaluasi_pada`) untuk MAPE realisasi; `model_registry`: `artefak_uri`, `catatan`.
2. **API/ML**
   - [x] **Layanan ML terpisah** `apps/ml` (Python, pustaka standar, tanpa dependensi): `GET /health`, `POST /train`, `POST /predict`; image `docker/ml.Dockerfile`, service `ml` di `compose.yaml` (jaringan internal, tanpa port publik). Go memanggilnya lewat `internal/prediksi/mlclient` (antarmuka + timeout, `ML_SERVICE_URL`); layanan mati/tak dikonfigurasi = `503` pada `POST /v1/prediksi/latih|segarkan`, API dan proyeksi demo tetap jalan.
   - [x] Baseline: naive, seasonal naive (mingguan), Holt (tren teredam), Holt-Winters aditif, dipilih per kabupaten lewat backtest walk-forward; metrik (MAPE/RMSE lipatan uji, MAPE/RMSE baseline naif pada lipatan yang sama, `optimistis` bila riwayat terlalu pendek untuk dipisah) ke `model_registry.metrics`. Interval 80% dari galat backtest. Dilatih dari `entri_harga` tervalidasi (konsumen, rata-rata harian per kabupaten, 180 hari). Hasil ditulis idempoten ke `prediksi_harga` (`sumber = MODEL`) berstatus KANDIDAT sampai dipromosikan (11-C). Komoditas pilot dengan riwayat demo 90 hari memenuhi syarat; cuaca/stok/hari raya/ensemble belum.
   - [x] **XAI:** `faktor` = penguraian ramalan (penyesuaian level, tren, musiman) dalam poin persen, jumlahnya sama dengan perubahan proyeksi; `feature_importance` = rata-rata |kontribusi| ternormalisasi. Belum SHAP/gradient boosting.
   - [x] `POST /v1/prediksi/segarkan` (ramal ulang dengan parameter model produksi hasil pelatihan), `POST /v1/prediksi/evaluasi` (galat nyata proyeksi jatuh tempo), `GET /v1/prediksi/ml` (status layanan). Izin `model.kelola` + audit.
   - [ ] Ensemble (regresi, gradient boosting), fitur cuaca/hari raya/stok/pasokan, estimasi volume panen dan risiko inflasi, jadwal retraining otomatis.
3. **FE**: [x] registri menampilkan MAPE backtest, baseline naif, MAPE realisasi (jumlah), umur model (penanda "Usang" > 30 hari), label Demo; tombol "Latih model" (izin `model.kelola`, nonaktif bila layanan ML mati); "Segarkan proyeksi" (komoditas terpilih, nonaktif dengan alasan bila layanan ML mati, model produksi masih demo, atau tak ada model produksi; ringkasan jumlah proyeksi ditulis ulang; pesan server untuk 503/409/422) dan "Evaluasi akurasi" (jumlah proyeksi jatuh tempo yang baru dievaluasi; nol dijelaskan sebagai keadaan wajar), keduanya hanya untuk `model.kelola` dan memuat ulang proyeksi, XAI, dan registri; "Unduh CSV" proyeksi komoditas terpilih di panel XAI (`/prediksi` dan `/eksekutif`, izin `prediksi.lihat`). [ ] Ekspor metrik registri.
4. **TES**: [x] unit Python (fungsi murni peramalan, kontrak layanan, server loopback); kontrak Go ↔ ML dengan `httptest`; integrasi (latih, segarkan idempoten, evaluasi, 503, 403); ekspor CSV (header, satu baris per titik seri, 404/400); e2e `prediksi-kpi.spec.ts` (unduh CSV, segarkan dua kali, evaluasi, tombol tersembunyi tanpa `model.kelola`, 403 API). Model kandidat tidak dapat dipromosikan bila MAPE-nya lebih buruk dari baseline (11-C).

---

## 11-C — MLOps dan simulasi *(Fase F, gelombang F-4)* — promosi, arsip, simulasi selesai

1. **MIG**: [x] `000091`: `model_promosi` (riwayat promosi/arsip, model yang digantikan, pelaku), `skenario_simulasi` (`parameter`, `hasil` JSONB, `dibuat_oleh`), `model_registry.dipromosikan_oleh`/`dipromosikan_pada`.
2. **API**
   - [x] `POST /v1/prediksi/registri/bulk` (`action`: `promosi`|`arsip`, izin `model.kelola`, audit `PROMOSI_MODEL`/`ARSIP_MODEL`): promosi KANDIDAT atau ARSIP (rollback) mengarsipkan model produksi komoditas itu dalam satu transaksi; model produksi tak dapat diarsipkan langsung. **Gerbang:** model hasil pelatihan yang MAPE backtest-nya lebih buruk dari baseline naif ditolak; model tanpa proyeksi ditolak (komoditas tidak boleh kehilangan proyeksi). Alasan penolakan dikembalikan per model.
   - [x] `POST /v1/prediksi/simulasi` (izin `prediksi.lihat`, audit `SIMULASI_SKENARIO`): proyeksi dasar dari model produksi + efek parameter (hujan, pasokan, subsidi) dihitung di server dan disimpan. **Terbatas:** koefisien masih `ASUMSI` generik (riwayat pasokan/cuaca/subsidi belum ada untuk diestimasi); hasil berlabel `terbatas` beserta koefisien dan sumbernya.
   - [ ] Retraining terjadwal, pemantauan drift (kolom `drift` masih nol untuk model hasil pelatihan), estimasi koefisien simulasi dari data (menunggu 05-B/05-C), antarmuka riwayat `model_promosi`.
3. **FE**: [x] aksi "Promosikan ke produksi" dan "Arsipkan kandidat" di registri (izin `model.kelola`, konfirmasi, alasan penolakan ditampilkan); simulasi memanggil endpoint untuk komoditas terpilih dan menampilkan efek per parameter dengan label asumsi.
4. **TES**: [x] promosi hanya oleh pemegang izin (403 untuk `prediksi.lihat` saja), satu produksi per komoditas, rollback, penolakan baseline dan tanpa proyeksi, arsip hanya KANDIDAT, riwayat dan audit tercatat; simulasi tersimpan, netral = proyeksi model, validasi rentang.
