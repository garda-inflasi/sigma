# Pipeline data dan ML

Alur data nyata dari sumber resmi sampai UI:

```text
BPS, PIHPS, BMKG, data pendukung ─┐
                                  ├─> apps/crawler (harian) ─> MongoDB dataunseen
Input lapangan (PostgreSQL) ──────┘                                  │
                                                                     v
                                          apps/ml worker (tiap 1 jam, append-only)
                                                                     │
                                                                     v
                                                            MongoDB datasafe
                                                                     │
                                                                     v
                                                   apps/api (baca saja) ─> apps/web
```

Dokumen ini adalah **kontrak antarlayanan**. Perubahan nama koleksi atau field harus diperbarui di sini dan di semua pembacanya. Keputusan ini menggantikan D4 ("Postgres saja") untuk data crawling dan hasil training. PostgreSQL tetap menjadi sumber data transaksi, RBAC, audit, alur EWS, dan input lapangan.

## 1. Aturan umum

- **Tidak ada data rekaan.** Sumber yang gagal menghasilkan status `ERROR`, bukan angka pengganti. Hanya seed demo yang boleh bertanda `DEMO`, dan pipeline ini tidak pernah menulisnya.
- **Konfigurasi:** `MONGO_URI` (fallback `MONGO_URL`), `MONGO_DB_UNSEEN` (bawaan `dataunseen`), `MONGO_DB_SAFE` (bawaan `datasafe`).
- **Hak tulis:** crawler hanya menulis `dataunseen`. Worker ML membaca `dataunseen` dan hanya menulis `datasafe`. API hanya membaca keduanya.
- **Waktu:** disimpan sebagai UTC `Date`. Tanggal bisnis (`periode`, `last_available_date`) adalah tengah malam UTC dari tanggal kalender WIB. `crawl_date` berupa teks `YYYY-MM-DD` WIB.
- **Kode:** komoditas memakai `komoditas.kode` (mis. `CBM-KRT`), wilayah memakai kode BPS (`11` provinsi, `1171` Banda Aceh).

## 2. Database `dataunseen`

Koleksi: `raw_pangan_harian`, `raw_iklim_bmkg`, `raw_bps_makro`, `raw_konsumsi_pendapatan`, `raw_neraca_ekonomi`, `raw_tenaga_kerja`, `raw_sektor_primer`, `crawl_runs`.

### Dokumen `raw_*`

| Field                              | Tipe        | Keterangan                                                                                          |
| ---------------------------------- | ----------- | --------------------------------------------------------------------------------------------------- |
| `_id`                              | string      | `{sumber}:{dataset}:{kunci_seri}:{periode YYYY-MM-DD}:{crawl_date}`, sehingga crawl ulang idempoten |
| `sumber`                           | string      | `BPS`, `PIHPS`, `BMKG`, `OPEN_METEO`, `SP2KP`, `BAPANAS`, `LAPANGAN`                                |
| `dataset`                          | string      | Nama dataset di sumber                                                                              |
| `kunci_seri`                       | string      | Identitas deret, mis. `CBM-KRT:1171` atau `inflasi_yoy:11`                                          |
| `indikator`                        | string      | Nama indikator normal                                                                               |
| `komoditas_kode`, `kabupaten_kode` | string/null | Terisi bila berlaku                                                                                 |
| `satuan`                           | string      | Mis. `Rp/kg`, `persen`, `mm`                                                                        |
| `frekuensi`                        | string      | `HARIAN`, `BULANAN`, `TRIWULANAN`, `TAHUNAN`                                                        |
| `periode`                          | Date        | Tanggal yang diwakili nilai. Untuk dokumen carried-forward: tanggal crawl                           |
| `nilai`                            | number/null | `null` hanya untuk `ERROR` tanpa nilai terakhir                                                     |
| `is_carried_forward`               | bool        | `true` bila nilai diteruskan dari rilis terakhir                                                    |
| `last_available_date`              | Date/null   | Tanggal data asli terakhir                                                                          |
| `crawled_at`                       | Date        | Waktu crawl                                                                                         |
| `crawl_date`                       | string      | Tanggal crawl WIB                                                                                   |
| `source_status`                    | string      | `FRESH`, `STALE_CARRIED_FORWARD`, `ERROR`                                                           |
| `run_id`                           | string      | Rujukan `crawl_runs`                                                                                |
| `url_sumber`                       | string      | URL tanpa kunci API                                                                                 |
| `status_validasi`                  | string/null | Khusus `LAPANGAN`: `DIAJUKAN`, `DIVALIDASI`, `DIPUBLIKASI`                                          |
| `atribut`                          | object      | Detail mentah khusus sumber                                                                         |

Indeks: `(kunci_seri, periode)`, `(sumber, dataset, crawl_date)`, `(komoditas_kode, kabupaten_kode, periode)`, `(source_status, crawl_date)`.

Pelatihan **hanya** memakai dokumen `is_carried_forward: false` sebagai titik data asli. Dokumen carried-forward dipakai untuk status kesegaran dan fitur umur data.

Aturan baca untuk pelatihan dan kesegaran:

- Saring `nilai != null`, karena dokumen `ERROR` juga bertanda `is_carried_forward: false`.
- Saring `sumber`, karena BMKG dan OPEN_METEO memakai `kunci_seri` yang sama. Prakiraan bertanda `atribut.jenis_data: PRAKIRAAN`, `periode`-nya bisa di masa depan, dan tidak boleh menjadi fitur historis.
- Ambil `crawled_at` terbaru per `(kunci_seri, periode)`, karena revisi sumber disimpan sebagai vintage baru.
- Abaikan `crawl_runs` dengan `pemicu: BACKFILL` saat menilai kesegaran sumber.
- `LAPANGAN`: satu dokumen per entri, `kunci_seri` = `KOM:KAB:TIPE:E{id}`, deret ada di `atribut.deret`, dan `crawl_date` = tanggal sinkron pertama. Entri seed demo (`HRG-Dnn`) tidak pernah disinkronkan.
- Awalan `DILEWATI:` pada `crawl_runs.pesan` menandai adapter yang sengaja tidak dijalankan (mis. dilarang robots atau butuh kunci).
- Koleksi internal crawler: `crawl_kunci` dan `crawl_jadwal`.

### Dokumen `crawl_runs`

`_id` (`run_id`), `crawler`, `sumber`, `dataset`, `crawl_date`, `mulai`, `selesai`, `status` (`FRESH`, `STALE_CARRIED_FORWARD`, `ERROR`), `jumlah_baru`, `jumlah_diteruskan`, `last_available_date`, `pesan`, `pemicu` (`JADWAL`, `MANUAL`, `BACKFILL`).

## 3. Database `datasafe`

Semua koleksi **append-only**: setiap dokumen memuat `cycle_id` dan `cycle_timestamp`, dan siklus lama tidak ditimpa. `cycle_id` berformat `C{YYYYMMDD}T{HH}Z`, sehingga menjalankan ulang jam yang sama bersifat idempoten. Siklus baru menjadi "terbaru" hanya setelah dokumen `ml_training_cycles`-nya berstatus `BERHASIL` atau `SEBAGIAN`, jadi pembaca tidak pernah melihat siklus yang setengah tertulis.

### `ml_training_cycles`

`_id` = `cycle_id`, `cycle_id`, `cycle_timestamp`, `data_cutoff`, `duration_ms`, `status` (`BERJALAN`, `BERHASIL`, `SEBAGIAN`, `GAGAL`), `pesan`, `versi_kode`, `jumlah` (per koleksi), `source_freshness[]` (`sumber`, `dataset`, `status`, `last_available_date`, `crawled_at`, `umur_hari`, `pesan`), `target_inflasi` (`RESMI_BPS` atau `INDEKS_PANGAN`), `model_metrics`:

```json
{
  "klasifikasi": {
    "model": "lightgbm",
    "f1": 0.0,
    "roc_auc": 0.0,
    "brier": 0.0,
    "n_uji": 0
  },
  "regresi": {
    "pemenang": {
      "lightgbm_quantile": 0,
      "sarimax": 0,
      "holt_winters": 0,
      "seasonal_naive": 0
    },
    "mape_median": 0.0,
    "rmse_median": 0.0,
    "mape_baseline_median": 0.0
  }
}
```

### `datasafe_inflasi_signals`

Satu dokumen per siklus × `wilayah_kode`: `periode`, `kelas` (`INFLASI` atau `TIDAK_INFLASI`), `probabilitas`, `skor_tekanan` (0-100), `siklus` (`NORMAL`, `TEKANAN`, `SURPLUS`), `target` (`RESMI_BPS` atau `INDEKS_PANGAN`), `proyeksi` (`yoy`, `mtm`, `ytd`, boleh `null`), `aktual` (`yoy`, `mtm`, `ytd`, `periode`, boleh `null`), `penyumbang[]` (`komoditas_kode`, `nama`, `andil`), `kalibrasi` (`metode`, `brier`).

`INDEKS_PANGAN` berarti label dihitung dari indeks harga pangan PIHPS karena inflasi resmi BPS belum tersedia. UI wajib menyebutnya indeks pangan, bukan inflasi resmi.

### `datasafe_forecasts`

Satu dokumen per siklus × `komoditas_kode` × `kabupaten_kode`: `harga_terakhir`, `tanggal_terakhir`, `satuan`, `model` (`nama`, `fallback`, `alasan`), `metrik` (`rmse`, `mape`, `mape_baseline`, `n_lipatan`, `horizon`), `kurva[]` (`tanggal`, `horizon`, `p10`, `p50`, `p90`), `riwayat[]` (`tanggal`, `nilai`). Horizon wajib: 7, 14, 30.

### `datasafe_xai_explanations`

Satu dokumen per siklus × `target` (`forecast` atau `inflasi`) × `komoditas_kode` (boleh `null`) × `wilayah_kode`: `metode` (`shap` atau `dekomposisi`), `faktor[]` (`kelompok`, `label`, `kontribusi`, `arah`, `fitur[]`), `narasi`.

Nilai `kelompok`: `CUACA`, `PASOKAN`, `KONSUMSI_PENDAPATAN`, `TENAGA_KERJA`, `NERACA_EKONOMI`, `TREN_HARGA_LAPANGAN`, `KALENDER`.

### `datasafe_ews_signals`

`_id` = `{cycle_id}:{sinyal_kunci}`, `sinyal_kunci` (stabil lintas siklus untuk kondisi yang sama), `severity` (`KRITIS`, `SIAGA`, `WASPADA`, `INFO`), `jenis` (`ANOMALI_HARGA`, `PROYEKSI_LONJAKAN`, `TEKANAN_INFLASI`, `SUMBER_DATA`), `komoditas_kode`, `kabupaten_kode`, `nilai`, `ambang`, `metode`, `judul`, `pesan`, `aktif`. Sinyal `KRITIS` tidak pernah disaring berdasarkan cooldown di pipeline.

### `datasafe_macro_indicators`

`domain` (`KONSUMSI_PENDAPATAN`, `NERACA_EKONOMI`, `TENAGA_KERJA`, `PERTANIAN`, `KEHUTANAN`, `PERIKANAN`, `INFLASI`), `indikator`, `wilayah_kode`, `periode`, `nilai`, `satuan`, `perubahan`, `sumber`, `is_carried_forward`, `last_available_date`.

Indeks semua koleksi hasil: `(cycle_id)`, `(cycle_timestamp)`, dan kunci bisnis koleksi + `cycle_timestamp` menurun.

### Tambahan yang disahkan

- `ml_worker_locks`: koleksi operasional worker, satu-satunya koleksi `datasafe` yang boleh berubah.
- `model_metrics.regresi.pemenang` memakai kunci `hgb_quantile` bila LightGBM tidak tersedia, dan `naive` bila persistensi menang.
- `datasafe_forecasts.model.n_kandidat`, `metrik.evaluasi`, `metrik.baseline` (baseline terbaik antara `naive` dan `seasonal_naive`), dan `metrik.mengalahkan_baseline` (`null` bila tidak ada holdout).
- `model_metrics.regresi` memuat `mengalahkan_baseline`, `n_seri`, dan `n_dievaluasi_holdout`. Metrik dihitung pada lipatan holdout yang tidak ikut memilih model.
- Model non-baseline hanya dipublikasikan bila mengalahkan baseline terbaik di setiap lipatan seleksi dan unggul minimal 5%. Bila tidak, baseline yang dipublikasikan.
- Walk-forward memakai 4 lipatan seleksi + 1 holdout murni, masing-masing berjarak 30 hari dan memuat 4 titik asal mingguan. Target satu lipatan tidak melewati awal lipatan berikutnya. MAPE lipatan adalah rata-rata MAPE per horizon. `metrik.n_lipatan_seleksi` mencatat jumlah lipatan yang benar-benar dipakai (minimal 2, di bawahnya baseline).
- `faktor[].fitur[]` berbentuk `{nama, kontribusi}`. Setiap dokumen XAI memuat `satuan_kontribusi`: `poin_persen_harga` (target `forecast`, perubahan P50 terhadap harga terakhir) atau `poin_persen_probabilitas` (target `inflasi`, dihitung pada probabilitas terkalibrasi yang dipublikasikan sehingga jumlah kontribusi = probabilitas − probabilitas dasar).
- **Kelayakan klasifikasi.** `model_metrics.klasifikasi` memuat `brier_baseline` (Brier tebakan konstan laju dasar pada holdout), `layak`, dan `alasan`. `layak` bernilai `true` hanya bila ROC-AUC holdout ≥ 0,60 dan Brier lebih rendah dari `brier_baseline`. Bila tidak layak, `datasafe_inflasi_signals.kelas`, `skor_tekanan`, dan `siklus` = `null` dengan `layak: false` dan `alasan`, `probabilitas` tetap dicatat untuk transparansi, XAI inflasi tidak diterbitkan, dan sinyal EWS `TEKANAN_INFLASI` tidak dibuat. Label klasifikasi: `MA7(indeks)[t+30] > indeks[t]`. Dokumen XAI inflasi memuat `probabilitas` dan `probabilitas_dasar`. `model_metrics.regresi` memuat `n_baseline` dan `porsi_baseline`. UI wajib menampilkannya sebagai "belum dapat diandalkan", bukan sebagai prediksi.
- `target_inflasi` bernilai `null` bila tidak ada sinyal, dan `INDEKS_PANGAN` bila target antarwilayah campuran.
- Asumsi rilis bila tanggal rilis tidak diketahui: bulanan +35 hari, triwulanan +60 hari, tahunan +120 hari. Fitur cuaca memakai data sampai t-2.

## 4. Arsitektur ML

| Tugas                 | Metode                                                                                            | Alasan                                                                                                                                                           |
| --------------------- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Inflasi atau tidak    | LightGBM Classifier + `CalibratedClassifierCV` (fallback `HistGradientBoostingClassifier`)        | Kuat untuk data tabular campuran harian dan bulanan, toleran nilai kosong, cepat dilatih tiap jam, probabilitasnya terkalibrasi, dan bisa dijelaskan dengan SHAP |
| Harga H+7, H+14, H+30 | LightGBM Quantile (P10, P50, P90) berkompetisi dengan SARIMAX, Holt-Winters, dan `seasonal_naive` | Interval ketidakpastian langsung dari model. Model statistik menjadi fallback untuk seri pendek                                                                  |
| Pemilihan model       | Walk-forward time-series CV per komoditas × wilayah                                               | Mencegah kebocoran data dan memastikan model harus mengalahkan baseline                                                                                          |
| Penjelasan            | SHAP `TreeExplainer`, dipetakan ke kelompok faktor kebijakan                                      | Dipahami pengambil keputusan                                                                                                                                     |
| Anomali               | Isolation Forest + Z-score musiman adaptif                                                        | Menangkap lonjakan yang belum pernah terjadi dan yang menyimpang dari pola musimannya                                                                            |

**Pencegahan kebocoran:** fitur makro digabung as-of berdasarkan tanggal rilis. Bila tanggal rilis tidak diketahui, dipakai asumsi konservatif akhir periode + 35 hari untuk data bulanan. Lag, rolling, dan skala hanya dihitung dari data sebelum titik potong.

**Bobot input lapangan:** `DIPUBLIKASI` 1,0, `DIVALIDASI` 0,8, `DIAJUKAN` 0,3. Entri ditolak tidak pernah dipakai.

## 5. API

`apps/api` membaca kedua database dengan modul `internal/ml` (klien `pkg/platform/mongodb`). API tidak pernah menulis ke Mongo (dijaga tes). Semua waktu kejadian RFC3339 UTC; tanggal bisnis (`periode`, `tanggal` kurva, `last_available_date`, `tanggal_terakhir`) berbentuk `YYYY-MM-DD`.

### 5.1 Ketersediaan

- Env `api` dan `worker`: `MONGO_URI` (fallback `MONGO_URL`), `MONGO_DB_UNSEEN`, `MONGO_DB_SAFE`. Tanpa URI, API tetap start dan semua endpoint lama berperilaku seperti sebelumnya.
- Batas waktu operasi Mongo 1,5 detik. Lampiran pada endpoint lama dibatasi 1,5 detik total. Setelah Mongo gagal, bacaan ditahan 20 detik (pemutus) agar halaman tidak menunggu timeout berulang.
- Hanya siklus final (`BERHASIL`/`SEBAGIAN`) yang isinya dibaca. "Terbaru" adalah siklus final dengan `cycle_timestamp` terbesar. Siklus `BERJALAN`/`GAGAL` hanya tampil sebagai metadata di `/v1/ml/cycles`.
- Setiap blok membawa `tersedia` dan, bila `false`, `alasan`: `TIDAK_DIKONFIGURASI`, `KONFIGURASI_TIDAK_VALID`, `MONGO_TIDAK_TERSEDIA`, `MONGO_LAMBAT`, `BELUM_ADA_SIKLUS`, `CYCLE_ID_TIDAK_VALID`, `SIKLUS_TIDAK_DITEMUKAN`, `SIKLUS_BELUM_FINAL`, `TANPA_IZIN`, `CAKUPAN_TIDAK_DAPAT_DIPERIKSA`, `TIDAK_ADA_DATA` (siklus tidak punya data untuk blok ini), `WILAYAH_TANPA_DATA` (siklus punya data, tetapi tidak untuk wilayah/komoditas yang diminta), `LUAR_CAKUPAN`. Blok yang `tersedia: false` tidak boleh ditampilkan sebagai angka.

### 5.2 Endpoint baru

| Endpoint                      | Izin             | Parameter                                                                                                                                                                                      | Isi `data`                                                                                                                                                   |
| ----------------------------- | ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GET /v1/ml/cycles`           | `prediksi.lihat` | `page` (1), `limit` (24, maks 168), `dari`/`sampai` (`YYYY-MM-DD` tanggal WIB, `sampai` inklusif; atau RFC3339), `status` (dipisah koma: `BERJALAN`, `BERHASIL`, `SEBAGIAN`, `GAGAL`, `FINAL`) | `{tersedia, alasan?, items: Siklus[], total, page, limit}`, terbaru dahulu, semua status. Header `X-Total-Count`. Tanpa Mongo: 200 dengan `tersedia: false`. |
| `GET /v1/ml/cycles/:cycle_id` | `prediksi.lihat` | `cycle_id` `C{YYYYMMDD}T{HH}Z` (400 bila salah format)                                                                                                                                         | `Siklus` apa pun statusnya. `source_freshness` hanya untuk pemegang `sumber.lihat`. 404 bila tidak ada, 503 bila Mongo tidak tersedia.                       |
| `GET /v1/ml/freshness`        | `sumber.lihat`   | -                                                                                                                                                                                              | Blok `kesegaran` (§5.3). Tanpa Mongo: 200 dengan `tersedia: false`.                                                                                          |

Peran bawaan: `prediksi.lihat` dimiliki ADMIN_SISTEM, STRATEGIS__, MANAJER_DATA, MANAJER_KEBIJAKAN, MANAJER_MUTU, dan MANAJER_PROGRAM. `sumber.lihat` dimiliki ADMIN_SISTEM, STRATEGIS__, MANAJER_DATA, dan MANAJER_PROGRAM. Peran OPS_* dan MANAJER_PEMBIAYAAN menerima 403.

`Siklus`: `cycle_id`, `cycle_timestamp`, `data_cutoff`, `status`, `final`, `duration_ms`, `pesan`, `versi_kode`, `jumlah` (per koleksi), `target_inflasi`, `label_target`, `model_metrics`, dan `source_freshness[]` (`sumber`, `dataset`, `status`, `last_available_date`, `crawled_at`, `umur_hari`, `pesan`, `dilewati`). `model_metrics`:

- `klasifikasi`: `model`, `f1`, `roc_auc`, `brier`, `brier_baseline`, `n_uji`, `kalibrasi`, `layak` (null pada dokumen lama), `kelayakan` (`LAYAK`, `TIDAK_LAYAK`, `TIDAK_DIKETAHUI`), dan `alasan`.
- `regresi`: `pemenang` (jumlah seri per model), `mape_median`, `rmse_median`, `mape_baseline_median`, `n_seri`, `mengalahkan_baseline`, `n_dievaluasi_holdout`, `n_dilewati`, `n_gagal`, `model_global`, dan `tingkat`. `params` tidak dipublikasikan.

```json
{
  "cycle_id": "C20261124T04Z",
  "cycle_timestamp": "2026-11-24T04:00:00Z",
  "status": "BERHASIL",
  "final": true,
  "target_inflasi": "INDEKS_PANGAN",
  "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
  "jumlah": {
    "datasafe_forecasts": 30,
    "datasafe_ews_signals": 14,
    "datasafe_inflasi_signals": 3
  },
  "model_metrics": {
    "klasifikasi": {
      "model": "lightgbm",
      "roc_auc": 0.538,
      "brier": 0.248,
      "brier_baseline": null,
      "layak": null,
      "kelayakan": "TIDAK_DIKETAHUI",
      "alasan": null
    },
    "regresi": {
      "pemenang": { "naive": 22, "seasonal_naive": 8, "lightgbm_quantile": 0 },
      "mape_median": 0.97,
      "rmse_median": 363.48,
      "mape_baseline_median": 0.97,
      "n_seri": 30
    }
  }
}
```

### 5.3 Lampiran `ml` di endpoint lama

Respons **sukses** GET pada rute di bawah membawa kunci `ml` di samping `data` (`{code, message, data, ml}`). Respons galat dan ekspor CSV tidak berubah. **`data` tidak pernah diubah** (`ml.data_utama: "POSTGRES"`): angka seed tetap membawa penanda `DEMO` miliknya, dan hasil training hanya ada di `ml`. Semua rute menerima `?cycle_id=` opsional (kosong berarti siklus final terbaru). `cycle_id` yang salah, tidak ada, atau belum final tidak menggagalkan endpoint; `ml.tersedia` bernilai `false` dengan alasannya.

| Rute GET                                                             | Blok                                                                                |
| -------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `/v1/dashboard/eksekutif`                                            | `inflasi`, `model`, `ews` (ringkas, 5 sinyal), `kesegaran`, `makro`                 |
| `/v1/dashboard/operasional/*` (semua)                                | `forecast_ringkas` (filter `kabupaten_kode`, `komoditas_kode`), `ews` (ringkas)     |
| `/v1/prediksi`                                                       | `forecast_ringkas` (kabupaten = `kabupaten`, bawaan `1171` seperti `data`), `model` |
| `/v1/prediksi/registri`, `/v1/prediksi/ml`                           | `model`                                                                             |
| `/v1/prediksi/:kode`                                                 | `forecast` (detail komoditas `:kode` di `kabupaten`, bawaan `1171`)                 |
| `/v1/ews`, `/v1/ews/:id`                                             | `ews` (maks 200; pada `:id` hanya sinyal yang dijembatani ke alert itu)             |
| `/v1/spasial/wilayah`, `/v1/spasial/neraca`, `/v1/spasial/geometri`  | `wilayah`                                                                           |
| `/v1/sumber-data`, `/v1/sumber-data/pipeline`, `/v1/sumber-data/:id` | `kesegaran`                                                                         |
| `/v1/kpi`, `/v1/kpi/:kode/riwayat`                                   | `model`                                                                             |
| `/v1/policy-brief`, `/v1/policy-brief/:id`                           | `inflasi`, `makro`                                                                  |

Rute `/v1/eksekutif/*` dan `/v1/operasional/*` tidak ada: dashboard eksekutif dan operasional berada di `/v1/dashboard/...`. Isi policy brief yang disusun (`POST /generate`) belum memakai data ML.

Izin per blok mengikuti data yang dibuka, terpisah dari izin rute. `inflasi`, `forecast*`, `model`, `wilayah`, dan `makro` memerlukan `prediksi.lihat`. `ews` memerlukan `ews.lihat`. `kesegaran` memerlukan `sumber.lihat`. Tanpa izin, blok berisi `{tersedia: false, alasan: "TANPA_IZIN"}`. Cakupan baris (02-B) berlaku: peran berkabupaten (OPS_KOORDINATOR, OPS_AGREGATOR) hanya melihat wilayah tugasnya. Sinyal tanpa kabupaten (tingkat sistem) dan sinyal provinsi `11` hanya terlihat oleh peran tingkat provinsi.

Bentuk lampiran:

- **Atas:** `tersedia`, `alasan`, `data_utama`, `siklus` (`cycle_id`, `cycle_timestamp`, `data_cutoff`, `status`, `target_inflasi`, `label_target`).
- **Setiap blok:** `tersedia`, `alasan`, `sumber_data` (`DATASAFE`, atau `DATAUNSEEN` untuk `kesegaran`), `cycle_id`, `cycle_timestamp`.
- **`inflasi`:** `kelayakan` dan `alasan_kelayakan` (dari klasifikasi siklus), `target`, `label_target`, `bukan_inflasi_resmi`, dan `wilayah[]`. Setiap elemen `wilayah[]` memuat:
  - `wilayah_kode`, `tingkat` (`PROVINSI`/`KABUPATEN`), dan `periode`;
  - `kelas` (null bila tidak layak), `layak`, `kelayakan`, `alasan_kelayakan`, `probabilitas`, `skor_tekanan`, dan `siklus`;
  - `target`, `label_target`, dan `bukan_inflasi_resmi`;
  - `proyeksi` dan `aktual` (`yoy`/`mtm`/`ytd`, boleh null), `penyumbang[]`, `kalibrasi`, dan `penjelasan` (null bila XAI inflasi tidak terbit).

  Wilayah tanpa sinyal tidak dicantumkan. Data saat ini hanya punya 1107, 1171, dan 1174, sedangkan provinsi `11` tidak punya sinyal.

- **`forecast_ringkas`:** `kabupaten_kode` (filter), `wilayah_tersedia[]`, dan `items[]` berupa `Forecast`.
- **`forecast`:** `komoditas_kode`, `kabupaten_kode`, `wilayah_tersedia[]` (kabupaten yang punya forecast komoditas ini), dan `item`. `item` berupa `Forecast` ditambah `riwayat[]` (`tanggal`, `nilai`; titik harga asli) dan `penjelasan`.
- **`Forecast`:**
  - `komoditas_kode`, `kabupaten_kode`, `harga_terakhir`, `tanggal_terakhir`, dan `satuan`;
  - `kurva[]` (`horizon` 7/14/30 berurutan, `tanggal`, `p10`, `p50`, `p90`, `perubahan_p50_pct` terhadap harga terakhir);
  - `model` (`nama`, `adalah_baseline`, `fallback`, `alasan`, `n_kandidat`);
  - `metrik` (`mape`, `rmse`, `baseline`, `mape_baseline`, `mengalahkan_baseline` (null tanpa holdout), `evaluasi`, `n_lipatan`, `n_lipatan_seleksi`, `horizon[]`).
- **`penjelasan`:** `metode` (`shap`/`dekomposisi`), `satuan_kontribusi` (`poin_persen_harga`/`poin_persen_probabilitas`, null pada dokumen lama), `narasi`, dan `faktor[]` (`kelompok`, `label`, `kontribusi`, `arah`, `fitur[]` `{nama, kontribusi}`). `faktor` boleh kosong (baseline persistensi).
- **`model`:** `status`, `target_inflasi`, `label_target`, `klasifikasi`, dan `regresi` (seperti `model_metrics` §5.2).
- **`ews`:** `ringkasan` (`aktif`, `kritis`, `siaga`, `waspada`, `info`), `sinyal[]`, dan `terpotong`. `sinyal[]` berisi sinyal aktif, paling parah dahulu. Setiap sinyal memuat `sinyal_kunci`, `severity`, `jenis`, `komoditas_kode`, `kabupaten_kode`, `nilai`, `ambang`, `metode`, `judul`, `pesan`, `aktif`, `menjadi_alert`, dan `alert` (`{id, kode, status}` alert Postgres hasil jembatan; null di dashboard ringkas). INFO ikut dicantumkan dengan `menjadi_alert: false`.
- **`wilayah`:** `items[]` hanya untuk wilayah yang punya hasil. Setiap item memuat `wilayah_kode`, `tingkat`, `komoditas_forecast[]`, `inflasi` (`kelas`, `kelayakan`, `probabilitas`, `skor_tekanan`, `siklus`, `target`, `label_target`), dan `sinyal_aktif`.
- **`kesegaran`:** `dihitung_pada`, `siklus` (null bila belum ada; `alasan_siklus`), `ringkasan` (`segar`, `basi`, `gagal`, `dilewati`, `belum_ada_run_terjadwal`), `sumber[]`, dan `sinyal_sumber[]`.
  - `sumber[]` memuat `sumber`, `dataset`, `kategori`, `status`, `pesan`, `last_available_date`, `umur_hari` (hari WIB, prakiraan di masa depan = 0), `run_terakhir`, `backfill_terakhir`, dan `menurut_siklus`.
  - `kategori` dinilai dari run terakhir **non-BACKFILL**: `SEGAR` (FRESH), `BASI` (STALE_CARRIED_FORWARD), `GAGAL` (ERROR nyata), `DILEWATI` (pesan `DILEWATI:`), atau `BELUM_ADA_RUN_TERJADWAL` (hanya pernah di-backfill).
  - `sinyal_sumber[]` adalah sinyal `SUMBER_DATA` siklus terbaru, termasuk INFO. Blok ini tidak dipengaruhi `cycle_id`.
- **`makro`:** `items[]` (`domain`, `indikator`, `wilayah_kode`, `periode`, `nilai`, `satuan`, `perubahan`, `sumber`, `is_carried_forward`, `last_available_date`). Saat ini kosong (`TIDAK_ADA_DATA`) karena belum ada `BPS_API_KEY`.

Contoh `GET /v1/prediksi/CBM-KRT?kabupaten=1171` (ringkas, data nyata siklus C20261124T04Z):

```json
{
  "code": "200",
  "message": "OK",
  "data": {
    "komoditas_kode": "CBM-KRT",
    "sumber": "DEMO",
    "...": "tidak berubah"
  },
  "ml": {
    "tersedia": true,
    "data_utama": "POSTGRES",
    "siklus": {
      "cycle_id": "C20261124T04Z",
      "status": "BERHASIL",
      "target_inflasi": "INDEKS_PANGAN",
      "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)"
    },
    "forecast": {
      "tersedia": true,
      "sumber_data": "DATASAFE",
      "cycle_id": "C20261124T04Z",
      "cycle_timestamp": "2026-11-24T04:00:00Z",
      "komoditas_kode": "CBM-KRT",
      "kabupaten_kode": "1171",
      "wilayah_tersedia": ["1107", "1171", "1174"],
      "item": {
        "harga_terakhir": 73750,
        "tanggal_terakhir": "2026-11-23",
        "satuan": "Rp/kg",
        "kurva": [
          {
            "horizon": 7,
            "tanggal": "2026-12-01",
            "p10": 51980.27,
            "p50": 71250,
            "p90": 97663.25,
            "perubahan_p50_pct": -3.4
          },
          {
            "horizon": 14,
            "tanggal": "2026-12-08",
            "p10": 48423.99,
            "p50": 71250,
            "p90": 104835.69,
            "perubahan_p50_pct": -3.4
          },
          {
            "horizon": 30,
            "tanggal": "2026-12-24",
            "p10": 44035.9,
            "p50": 72500,
            "p90": 119362.84,
            "perubahan_p50_pct": -1.7
          }
        ],
        "model": {
          "nama": "seasonal_naive",
          "adalah_baseline": true,
          "fallback": false,
          "alasan": "tidak ada kandidat yang mengalahkan baseline; data asli terakhir 1 hari lalu",
          "n_kandidat": 5
        },
        "metrik": {
          "mape": 3.8,
          "rmse": 2894.99,
          "baseline": "seasonal_naive",
          "mape_baseline": 3.8,
          "mengalahkan_baseline": false,
          "evaluasi": "holdout_lipatan_terakhir",
          "n_lipatan": 5,
          "n_lipatan_seleksi": 4,
          "horizon": [7, 14, 30]
        },
        "riwayat": [{ "tanggal": "2026-11-23", "nilai": 73750 }],
        "penjelasan": {
          "metode": "dekomposisi",
          "satuan_kontribusi": null,
          "faktor": [],
          "narasi": "Harga Cabai Merah Keriting di wilayah 1171 ..."
        }
      }
    }
  }
}
```

Contoh blok `inflasi` (`GET /v1/dashboard/eksekutif`):

```json
{
  "tersedia": true,
  "sumber_data": "DATASAFE",
  "cycle_id": "C20261124T04Z",
  "cycle_timestamp": "2026-11-24T04:00:00Z",
  "kelayakan": "TIDAK_DIKETAHUI",
  "alasan_kelayakan": null,
  "target": "INDEKS_PANGAN",
  "label_target": "Indeks harga pangan PIHPS (bukan inflasi resmi)",
  "bukan_inflasi_resmi": true,
  "wilayah": [
    {
      "wilayah_kode": "1171",
      "tingkat": "KABUPATEN",
      "periode": "2026-12-24",
      "kelas": "INFLASI",
      "layak": null,
      "kelayakan": "TIDAK_DIKETAHUI",
      "alasan_kelayakan": null,
      "probabilitas": 0.508,
      "skor_tekanan": 49.54,
      "siklus": "NORMAL",
      "target": "INDEKS_PANGAN",
      "bukan_inflasi_resmi": true,
      "proyeksi": { "yoy": null, "mtm": -0.54, "ytd": null },
      "aktual": {
        "yoy": 1.46,
        "mtm": 2.09,
        "ytd": -1.57,
        "periode": "2026-11-24"
      },
      "penyumbang": [
        {
          "komoditas_kode": "CBM-KRT",
          "nama": "Cabai Merah Keriting",
          "andil": 1.57
        }
      ],
      "kalibrasi": { "metode": "sigmoid", "brier": 0.248 },
      "penjelasan": {
        "metode": "shap",
        "satuan_kontribusi": null,
        "faktor": ["...2 kelompok..."],
        "narasi": "..."
      }
    }
  ]
}
```

Tanpa Mongo: `"ml": {"tersedia": false, "alasan": "TIDAK_DIKONFIGURASI", "data_utama": "POSTGRES", "forecast": {"tersedia": false, "alasan": "TIDAK_DIKONFIGURASI", ...}}`.

**Aturan tampilan untuk UI:**

1. Pakai blok `ml` hanya bila `tersedia: true`. Selain itu tampilkan `data` lama dengan penanda sumbernya (`DEMO` tetap `DEMO`).
2. `target` `INDEKS_PANGAN` (`bukan_inflasi_resmi: true`) disebut "indeks harga pangan", bukan inflasi resmi.
3. `kelas` ditampilkan sebagai prediksi hanya bila `kelayakan` = `LAYAK`. `TIDAK_LAYAK` dan `TIDAK_DIKETAHUI` (dokumen lama tanpa `layak`) ditampilkan "belum dapat diandalkan", dengan probabilitas boleh terlihat sebagai transparansi.
4. `model.adalah_baseline: true` berarti tidak ada model yang mengalahkan baseline, dan yang dipublikasikan adalah baseline (`naive`/`seasonal_naive`).
5. Wilayah di luar `wilayah_tersedia` tidak punya forecast.

**Header `X-Garda-Data-Env`** (atau `?data_env=`), ditambahkan di luar slice ini:

- `staging`/`demo` membuat lampiran selalu `{tersedia: false, alasan: "MODE_STAGING"}`.
- `prod`/`real` membuang baris dan blok bertanda `DEMO` dari `data`, lalu mengisi `data_utama: "DATASAFE"` bila siklus tersedia. Walau begitu, `data` tetap berasal dari PostgreSQL.
- Respons membawa `data_env`.

Tanpa header, perilakunya seperti yang diuraikan di atas.

### 5.4 Jembatan EWS

Job worker `ml_jembatan_ews` (`JOB_ML_JEMBATAN_EWS_INTERVAL`, bawaan `5m`, `off` mematikan, advisory lock seperti job lain) hanya terdaftar bila `MONGO_URI`/`MONGO_URL` diisi. Tiap putaran membaca sinyal **siklus final terbaru** saja; sinyal siklus lama, termasuk KRITIS basi, tidak diterapkan.

- **Menjadi alert:** sinyal `aktif` bertingkat WASPADA/SIAGA/KRITIS. INFO, termasuk `SUMBER_DATA` untuk sumber yang sengaja dilewati, tidak pernah menjadi alert, tetapi tetap tampil di `/v1/ml/freshness` dan blok `kesegaran`. Kegagalan crawler nyata (ERROR atau basi) menjadi alert `ANOMALI_DATA` tingkat sistem.
- **Pemetaan `jenis`:** `ANOMALI_HARGA` naik menjadi `LONJAKAN_HARGA`, `ANOMALI_HARGA` turun menjadi `ANOMALI_DATA` (tidak diklaim sebagai over-supply), `PROYEKSI_LONJAKAN` dan `TEKANAN_INFLASI` menjadi `LONJAKAN_HARGA`, dan `SUMBER_DATA` menjadi `ANOMALI_DATA`. Asal alert ditandai `sumber` `"Pipeline ML"`, `aturan_id` null, dan pesan yang menyebut siklus serta metodenya.
- **Wilayah:** sinyal tanpa kabupaten dan sinyal provinsi `11` menjadi alert dengan `kabupaten_kode` NULL (migrasi 000131). Di `/v1/ews`, alert ini tampil dengan `kabupaten_kode: ""` dan `wilayah: "PROVINSI ACEH"`, dan hanya untuk peran tingkat provinsi.
- **Idempoten:** `dedup_key` = `ML|<sinyal_kunci>` menjaga satu alert aktif per kondisi. Tabel `ews_sinyal_ml` menyimpan siklus terakhir yang sudah diterapkan per kunci, sehingga putaran berulang dalam satu siklus tidak menulis apa pun dan siklus berikutnya dengan kondisi sama tidak membuat duplikat.
- **KRITIS:** mengikuti aturan modul EWS. KRITIS tidak kena cooldown (sinyal yang masih KRITIS di siklus berikutnya setelah alertnya ditutup membuat alert baru). Naik ke KRITIS membuka alert kembali ke BARU. Tidak ada alert yang ditutup otomatis. Non-KRITIS yang baru ditutup diredam 24 jam.
- **Pulih:** sinyal `aktif: false` hanya menambah catatan riwayat pada alert yang masih terbuka, tanpa mengubah statusnya.
- **Alur:** riwayat, `ews_event` (SSE), notifikasi (kanal EMAIL dan PUSH karena tidak ada aturan, tunduk preferensi pengguna kecuali KRITIS), dan audit `BUAT_OTOMATIS`/`NAIK_TINGKAT`. Alert pipeline tidak otomatis menjadi kartu intervensi dan tidak dihitung mesin siklus ekonomi (08-B).
