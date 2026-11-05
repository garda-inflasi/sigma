# 11 — Prediksi harga & XAI

**Prioritas:** P0 (PRD §2.1) · **Status:** UI ✅ · Backend 🟡 (membaca tabel; model nyata di 11-B) · Terhubung ✅ · Skema & seed ✅ · Tes ✅ (11-A selesai)
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [prediksi-page.tsx](../../apps/web/src/components/prediksi/prediksi-page.tsx) di `/prediksi` (panel XAI, simulasi skenario, proyeksi semua komoditas, registri model) dan [xai-panel](../../apps/web/src/components/eksekutif/xai-panel.tsx).
- **API Go (`prediksi`):** `GET /v1/prediksi`, `/:kode`, `/:kode/export` (izin `prediksi.lihat`); usecase hanya membaca baris `ProyeksiXai`.
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
   - [x] `GET /v1/prediksi?kabupaten=&horizon=` (ringkasan semua komoditas), `/:kode` (seri riwayat + proyeksi, faktor, narasi, rekomendasi, model), `/:kode/export`, dan `GET /v1/prediksi/registri`. Hanya model `PRODUKSI` yang dipakai. Riwayat aktual dibaca dari `entri_harga` (konsumen, tervalidasi/terpublikasi) bila ada. Narasi dan rekomendasi dari templat aturan, bukan LLM.
   - [x] Izin yatim `prediksi.baca` hilang bersama modul `analytics`; semua route memakai `prediksi.lihat`.
4. **FE**
   - [x] `prediksi-page.tsx` dan `xai-panel.tsx` memakai `useProyeksiRingkasan`, `useModelPrediksi`, `useXai` (API nyata, mapper `lib/api/prediksi.ts`); tidak ada `mockXai` di komponen (hanya di lapisan mode mock). Data seed berlabel "Data demo".
   - [x] "Simulasi skenario" tetap dihitung di klien dari perubahan proyeksi cabai dan diberi label koefisien contoh. Aksi "Promosikan"/"Arsipkan" di registri dihapus dari UI karena tidak tersimpan; kembali di 11-C dengan endpoint dan izin `model.kelola`.
5. **TES**: [x] tiap komoditas pilot punya proyeksi demo di semua kabupaten dengan `bawah <= nilai <= atas`; jumlah `faktor` = perubahan proyeksi; seeder idempoten lintas hari; endpoint 200/401/403/400/404 pada DB hasil migrasi; `prediksi` keluar dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md). Khusus: tidak ada `import … mockXai` yang tersisa di komponen.

---

## 11-B — Model prediksi pertama *(Fase F, gelombang F-4)*

**Tujuan:** proyeksi dihasilkan model, bukan seed. Target PRD §12: MAPE ≤ 5,4%.
**Bergantung pada:** 11-A, 06-A, 05-C (cuaca).

1. **MIG**
   - [ ] `prediksi_evaluasi` (`prediksi_id`, `realisasi`, `galat_pct`, `dievaluasi_pada`) untuk MAPE nyata.
   - [ ] `model_registry`: `artefak_uri`, `catatan`.
2. **API/ML**
   - [ ] **Layanan ML terpisah** (Python/FastAPI) dengan kontrak dan versi; Go memanggilnya dan menulis ke `prediksi_harga` secara idempoten.
   - [ ] Baseline dulu (seasonal naive, ETS/ARIMA/Prophet) untuk komoditas pilot (cabai, bawang merah, beras); lalu ensemble (time-series + regresi + gradient boosting) dengan fitur harga historis, cuaca, hari raya, stok, pasokan.
   - [ ] **XAI:** nilai SHAP/kontribusi fitur per prediksi → `faktor`; interval kepercayaan pada setiap proyeksi; estimasi volume panen dan risiko inflasi.
   - [ ] Evaluasi walk-forward; skrip menyimpan metrik per versi.
3. **FE**: tampilkan metrik model (MAPE, tanggal latih, versi), peringatan model usang, label "proyeksi" bukan fakta; ekspor.
4. **TES**: kontrak Go ↔ ML; build gagal bila MAPE kandidat lebih buruk dari baseline.

---

## 11-C — MLOps dan simulasi *(Fase F, gelombang F-4)*

1. **MIG**: `model_promosi` (riwayat promosi), `skenario_simulasi` (`id`, `parameter` JSONB, `hasil` JSONB, `dibuat_oleh`, `dibuat_pada`), `model_registry`: `dipromosikan_oleh`/`dipromosikan_pada`.
2. **API**: promosi model dengan persetujuan manusia (izin `model.kelola`); retraining terjadwal dan pemantauan drift/performa; `POST /v1/prediksi/simulasi` (what-if pasokan, cuaca, kebijakan); audit keputusan ([03-A](03-audit.md)).
3. **FE**: aksi promosi di registri model; simulasi memakai endpoint.
4. **TES**: promosi hanya oleh pemegang izin; simulasi tersimpan.
