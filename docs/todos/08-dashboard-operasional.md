# 08 — Dashboard operasional

**Prioritas:** P0 (Fase 1 "Dashboard dasar") · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** `/operasional` memakai [kpi-grid](../../apps/web/src/components/dashboard/kpi-grid.tsx), [price-heatmap](../../apps/web/src/components/dashboard/price-heatmap.tsx), [ops-map](../../apps/web/src/components/dashboard/ops-map.tsx), [field-activity](../../apps/web/src/components/dashboard/field-activity.tsx), feed EWS, [trend-chart](../../apps/web/src/components/charts/trend-chart.tsx), antrean validasi. Polling 30 detik.
- **API Go (`dashboard`):** `GET /v1/dashboard/operasional/{kpi,heatmap}`, `GET /v1/siklus/aktif`, `GET /v1/harga/tren`.
- **API Go (`analytics`, MongoDB):** `/v1/operations/*`, `/v1/regions/status-map`, `/v1/field-activities/progress`. **Tidak dipanggil web.**
- **Kritis:** repository dashboard ([repo.go](../../apps/api/internal/dashboard/repository/postgres/repo.go)) memakai SQL mentah pada tabel `kpi_operasional` dan `heatmap_sel` yang **tidak ada di migrasi**. Entity siklus memakai tabel `siklus_ekonomi` dan tipe `siklus_ekonomi`; migrasi hanya punya enum `siklus` dan tidak punya tabelnya. Jadi `/v1/dashboard/operasional/*` dan `/v1/siklus/aktif` (warna ambient seluruh UI) error pada DB hasil migrasi.

### Kunci primer (000115, ✅)

- `siklus_ekonomi.id` kini **BIGINT identity** (dulu VARCHAR `SE-11-DEMO`; ID berurut menurut `dibuat_pada`); ID lama di `kode` (UNIQUE) menjadi kunci idempotensi seeder demo (`ON CONFLICT (kode)`). `wilayah_kode` → `provinsi` **RESTRICT** (indeks `idx_siklus_ekonomi_wilayah`); satu siklus aktif per wilayah tetap dijaga indeks unik parsial `uq_siklus_ekonomi_aktif`. Tidak ada rute `:id`: `GET /v1/siklus/aktif?wilayah_kode=` memakai kode wilayah.
- `target_kpi.id` kini **BIGINT identity** dengan `UNIQUE (indikator, periode_mulai)` (kunci alami lama) sebagai target upsert seeder `reference`.
- Agregat (`agregat_harga_harian`, `mv_heatmap_harga`, `mv_kpi_operasional`) adalah materialized view dengan kunci alaminya sendiri; tidak ada PK surrogate. `gold_harga_harian` dan `cuaca_harian` ([05](05-sumber-data.md)) dibaca tanpa bergantung pada PK.
- Tugas yang belum selesai di bawah (filter wilayah/komoditas/periode, row scope) memakai kode wilayah/komoditas sebagai parameter kueri (`kabupaten_kode`, `komoditas_kode`), bukan ID surrogate.

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
   - [x] `api.operasional.*` dan `api.siklus` sudah memanggil API; cocokkan bentuk. Ganti `USERS` statis di `field-activity` dengan nama petugas dari API (marker peta menyusul di [12-A](12-peta-spasial.md)).
     - Bentuk dicocokkan dengan **respons nyata**: fixture di `apps/web/src/lib/api/__fixtures__/` diambil dari API + DB demo (peran provinsi dan Koordinator, tanpa token/PII). Tes kontrak `dashboard-kontrak.test.ts` menjalankan pemeta produksi atas fixture dan **gagal bila backend menambah field yang tak dipetakan** (atau pemeta membaca field yang tak dikirim); yang sengaja diabaikan harus didaftarkan.
     - Ditemukan dan diperbaiki lewat data nyata: keterangan delta "poin poin", delta 0 pada kartu "waspada" tampil merah, heatmap kosong menghasilkan CSS `repeat(0, …)`, serta galat query yang tampil sebagai skeleton tanpa akhir atau "antrean kosong" (kini `PanelGalat` dengan "Coba lagi"; data yang sudah tampil tidak dihapus bila polling berikutnya gagal).
     - `field-activity`: sakelar "Per wilayah | Per petugas" (bawaan tetap per wilayah; daftar petugas baru diminta saat dipilih, terendah di atas, 20 pertama lalu "Tampilkan semua"). `GET /aktivitas-petugas` butuh `harga.lihat` selain `dashboard.operasional`: 403 ditampilkan sebagai keterangan, bukan galat. `USERS` tidak lagi diimpor komponen maupun `endpoints.ts` (masih dipakai mock/auth mock).
5. **TES**
   - [x] Setelah `reference + demo + refresh`, KPI dan heatmap mengembalikan baris dan cocok dengan tipe web; `EXPLAIN` pada data ratusan ribu baris; hapus entri dashboard/siklus dari `belumSelaras`.
     - Cocok dengan tipe web: tes kontrak fixture nyata (lihat FE di atas) dan tes kontrak OpenAPI sisi Go (`dashboard/integration/kontrak_test.go`).
     - `EXPLAIN`/benchmark pada 2,4 juta entri harga (jauh di atas ratusan ribu): `mv_kpi_operasional` 91,7 ribu baris (11 MB), `REFRESH … CONCURRENTLY` ±1,3 s; kueri KPI 2,9 ms (20 hari terfilter) sampai 53 ms (109 hari tanpa filter); heatmap seluruh Aceh langsung 13 ms.
     - `belumSelaras` (kini `pendingModules`) sudah kosong dan diuji tidak basi; `schematest` mencakup `siklus_ekonomi`, `aturan_siklus`, dan bentuk baru `mv_kpi_operasional`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 08-B — Hasil ingestion di dashboard operasional, filter, dan penentuan siklus *(Fase F)*

Dikerjakan di gelombang F-3 (data dari ingestion/gold) dan penyelesaian filter, cakupan baris, ringkasan operasional, cache, dan penentuan siklus otomatis.

1. **MIG**: `000124_08b_kpi_operasional_grain_halus.sql` (`mv_kpi_operasional` dibangun ulang: indikator × tanggal × kabupaten × komoditas dengan pembilang/penyebut, riwayat 120 hari, UNIQUE tanpa NULL agar `REFRESH CONCURRENTLY` jalan; Down memulihkan definisi 000122) dan `000125_08b_siklus_otomatis.sql` (`aturan_siklus` + kolom `otomatis`, `alasan`, `pulih_sejak` pada `siklus_ekonomi`). Membaca juga `gold_harga_harian` dan `cuaca_harian` dari [05-C](05-sumber-data.md).
2. **API**
   - [x] Heatmap dilengkapi sel dari harga eksternal bila belum ada data lapangan (`sumber: EKSTERNAL`); sel lapangan tidak digantikan.
   - [x] `GET /v1/dashboard/operasional/pembanding-harga`: validasi silang harga lapangan vs eksternal (selisih terbesar, jumlah pasangan sepakat).
   - [x] `GET /v1/dashboard/operasional/cuaca`: cuaca harian terbaru per kabupaten/kota dengan penanda hujan lebat.
   - [x] Filter wilayah/komoditas/periode pada KPI dan heatmap; row scope Koordinator/Agregator ([02-B](02-pengguna-rbac.md)).
     - **Filter:** query opsional `kabupaten_kode`, `komoditas_kode`, `dari`, `sampai` (tanggal WIB; bawaan = perilaku lama: seluruh Aceh, 14 hari berakhir hari ini). Rentang maksimum 90 hari dan harus dalam 90 hari terakhir, tidak boleh di masa depan; format salah atau `dari > sampai` dijawab 400 tanpa memantulkan isian; kode yang tak dikenal menghasilkan hasil kosong. Persentase (validasi, `cs`) selalu dari pembilang dan penyebut yang dijumlahkan dulu, bukan rata-rata persentase (dibuktikan tes hitung-tangan dan perbandingan dengan SQL independen pada 14 kombinasi filter).
     - **Heatmap:** tanpa `dari`/`sampai` dibaca dari `mv_heatmap_harga` (jalur cepat); dengan salah satunya dihitung langsung dari `agregat_harga_harian` dengan rumus yang sama relatif terhadap `sampai`. `dari` pada heatmap membatasi sel yang harga terakhirnya jatuh pada atau setelah tanggal itu (perluasan di luar spesifikasi; ubah bila tidak diinginkan). Sel eksternal mengikuti tanggal acuan, cakupan, dan filter yang sama.
     - **Nada kartu:** `target_kpi` tidak punya kolom wilayah (target provinsi), jadi nada normal/waspada/defisit hanya dipakai untuk seluruh Aceh tanpa filter; selain itu `data` (netral). Akibatnya Koordinator/Agregator selalu melihat kartu netral. Bila kelak ada target per wilayah, ubah `usecase.TingkatAceh`.
     - **Row scope (02-B):** peran provinsi ke atas melihat seluruh Aceh; Koordinator/Agregator hanya kabupaten tugasnya, **dibaca dari DB per request** (klaim token `kab` hanya tampilan dan bisa basi); Mahasiswa dan peran tak dikenal tertutup. Klasifikasi peran bersumber tunggal `harga/domain.CakupanPeran`. Cakupan tertutup atau filter kabupaten di luar cakupan → hasil kosong (bukan 403); galat DB saat membaca cakupan → 500 (gagal-tutup). Berlaku untuk kelima endpoint lama dan empat endpoint baru; tes penjaga rute gagal bila ada rute `/v1/dashboard/operasional/*` yang belum masuk tes cakupan.
   - [x] Endpoint ringkasan aktivitas per petugas; stok real-time, agregasi produsen, pergerakan logistik (PRD §3.1). Semuanya `GET /v1/dashboard/operasional/…`, izin `dashboard.operasional`, row scope, cache, dan larik selalu `[]` (DB kosong → 200 dengan nol).
     - `/aktivitas-petugas`: mahasiswa aktif berkabupaten tugas; nama tampilan + UUID saja (tanpa email/telepon); entri hari ini vs target harian (sama dengan `aktivitas-lapangan`), menunggu validasi, ditolak 7 hari, entri terakhir, tugas aktif/selesai/terlambat (rumus identik `StatusEfektif`, ada tes paritas). **Butuh `harga.lihat` juga**: daftar entri harga sudah membuka nama petugas dan dijaga izin itu; ke-12 peran bawaan pemegang `dashboard.operasional` sudah memegangnya (dikunci tes SQL atas `role_permissions`).
     - `/stok`: lot belum HABIS di fasilitas OPERASIONAL yang berlaku menurut tanggal Aceh (definisi sama dengan kartu KPI `cs`; `utilisasi_pct` identik), per komoditas, plus mendekati kedaluwarsa (≤ 7 hari), risiko tinggi (> 0,6), lot kedaluwarsa, dan stok di fasilitas non-operasional (total + kedaluwarsa + di luar operasional = total halaman cold-storage).
     - `/produsen`: harga produsen vs konsumen 7 hari dari `agregat_harga_harian` (DIVALIDASI/DIPUBLIKASI, tertimbang jumlah entri; konsumen hanya dari kabupaten yang juga punya harga produsen), margin hanya bila kedua sisi ada (`persen_produsen` dan `persen_konsumen`, yang kedua sama dengan KPI `EKO-MARGIN`), produksi dari `neraca_pangan` satu wilayah saja (provinsi untuk seluruh Aceh, kabupaten untuk tampilan kabupaten; tidak pernah dijumlahkan; angka resmi mengalahkan DEMO). Jumlah pasar pelapor tidak tersedia (agregat hanya menyimpan kabupaten).
     - `/logistik`: transaksi stok 7 hari per jenis (MASUK/KELUAR/PINDAH) dan komoditas, 10 pergerakan terbaru (tanpa pelaku dan alasan), dan rute distribusi aktif (`status <> 'DIGANTIKAN'`). Cakupan: pergerakan bila fasilitas asal ATAU tujuan di kabupaten itu; rute bila kabupaten asal ATAU tujuan.
   - [x] **Penentuan siklus otomatis** dari ambang EWS dan stok (kaitan [10-B](10-ews.md), [14-C](14-intervensi.md)); sekarang hanya membaca baris.
     - **Aturan sebagai data:** `aturan_siklus` (`kode` UNIQUE, indikator `ALERT_EWS` atau `UTILISASI_STOK`, ambang, prioritas, `pulih_jam`, `aktif`). Seed `reference` (`14_siklus_aturan.sql`, `ON CONFLICT (kode) DO NOTHING`, jadi tidak menimpa aturan yang diubah operator): lonjakan harga, kelangkaan stok, dan anomali cuaca ≥ SIAGA → TEKANAN (prioritas 10, pulih 48 jam); over-supply ≥ SIAGA dan utilisasi cold storage ≥ 90% → SURPLUS (prioritas 20, pulih 72 jam). Pemetaan jenis→siklus dikunci tes terhadap `workflow.SiklusUntuk` (14-C).
     - **Keputusan** (`siklus/engine.Putuskan`, fungsi murni): TEKANAN diutamakan atas SURPLUS (salah mengabaikan tekanan lebih mahal; pemicu yang kalah tercatat di `alasan.ditekan`). Masuk ke siklus terpicu langsung; keluar memakai histeresis (`pulih_sejak`, masa pulih sengaja lebih panjang dari `cooldown_jam` EWS 24 jam agar tidak berkedip). Tanpa aturan aktif sama sekali evaluasi dilewati (bukan dibaca "normal"); galat mengukur satu aturan membatalkan putaran (gagal-tutup).
     - **Atomik dan aman multi-instance:** satu transaksi dengan `pg_advisory_xact_lock` per wilayah + `FOR UPDATE`, tutup dan buka pada instan yang sama, `uq_siklus_ekonomi_aktif` sebagai pengaman terakhir; waktu dari jam DB. Tiap pergantian diaudit (`GANTI_SIKLUS`, `EVALUASI`).
     - **Penjadwalan dan API:** job `siklus_evaluasi` di `pkg/worker` (`JOB_SIKLUS_EVALUASI_INTERVAL`, bawaan 5 menit, `off` mematikan) ditambah pemicu setelah evaluasi EWS; `POST /v1/siklus/evaluasi` (`ews.atur_ambang`), `GET /v1/siklus/aturan` (`ews.lihat`); `GET /v1/siklus/aktif` bertambah `otomatis` dan `alasan` (aditif). Seed demo `SE-11-DEMO` tidak lagi melawan mesin (tidak membuka ulang siklus yang sudah ditutup).
     - Hanya Aceh (`11`) yang dievaluasi. Alert EWS hanya ditutup manusia, jadi TEKANAN bertahan selama ada alert ≥ SIAGA. Jalur SURPLUS dari data nyata praktis hanya aturan utilisasi sampai rule engine EWS mendeteksi over-supply.
   - [x] Cache untuk agregasi berat.
     - Cache memori per proses (`dashboard/cache`), TTL 20 detik (di bawah polling UI 30 detik), maksimum 256 entri, singleflight per kunci, galat tidak di-cache. **Kunci memuat endpoint, cakupan, semua filter, dan hari WIB**, jadi hasil satu cakupan tidak pernah tersaji ke cakupan lain (tes untuk kesembilan endpoint). Refresher menaikkan epoch setiap penyegaran MV sehingga hasil lama dibuang segera setelah publikasi harga.
     - Konsekuensi multi-instance: epoch hanya melihat penyegaran di proses yang sama; bila penyegaran berkala dijalankan worker terpisah (`API_BACKGROUND_JOBS=false`) atau instance lain, hasil baru terlihat setelah TTL (≤ 20 detik). Kebenaran cakupan tidak bergantung pada ini.
   - [ ] Keterkinian stok/logistik di panel masih ≤ TTL 20 detik + polling 30 detik (± 50 detik). Target PRD "< 5 detik" butuh penulisan stok di modul cold-storage menaikkan epoch cache.
   - [ ] Indeks `transaksi_stok (waktu DESC, id DESC)` (migrasi berikutnya) bila buku besar mendekati 1 juta baris: tanpa indeks, agregat seluruh Aceh 116 ms dan "terbaru" 109 ms pada 1 juta baris (dengan indeks 16 ms dan 0,06 ms); pada skala realistis seq scan ±6–12 ms.
   - [ ] Ubah aturan siklus lewat API/UI (sekarang hanya SQL; matikan dengan `aktif = false`, bukan DELETE, karena seed menanam ulang aturan yang dihapus). Daftar petugas belum berpaginasi (aman sampai ± 1000 petugas). Rute distribusi "aktif" tidak pernah kedaluwarsa sendiri sehingga `rute.jumlah/biaya` menumpuk lintas bulan.
3. **FE**: kartu "Validasi silang harga" dan "Cuaca wilayah" di `/operasional`; sel heatmap bersumber eksternal diberi garis putus-putus dan keterangan; keadaan kosong/galat untuk tiap kartu lama (`PanelGalat`); panel petugas di `field-activity`; tooltip pil siklus di `Topbar` ("Ditentukan otomatis sejak … " bila `otomatis`); klien, pemeta, hook, dan mock untuk `aktivitas-petugas`, `stok`, `produsen`, `logistik`, serta parameter filter opsional pada `kpi` dan `heatmap`.
   - [x] Kartu UI stok (`ops-stock-card.tsx`), produsen (`ops-producer-card.tsx`), dan logistik (`ops-logistics-card.tsx`) di `/operasional` serta bar filter wilayah/komoditas/periode (`ops-filter-bar.tsx` + `useFilterOperasional`) yang tersinkronisasi dengan URL query string, lengkap dengan `PanelGalat` dan uji komponen (`ops-08b-cards.test.tsx`).
   - [ ] Peta Mapbox dan feed EWS via SSE di dashboard ([10-C](10-ews.md)). Tata letak sakelar petugas, mode gelap, dan animasi belum diverifikasi di browser.
4. **TES**: ✅ usecase (urutan/penghitungan, kunci cache, tabel keputusan siklus), integrasi Postgres dengan seed reference + demo + refresh (nilai hitung-tangan, perbandingan dengan SQL independen, ingestion contoh, otorisasi 401/403, cakupan per peran termasuk kabupaten dibaca dari DB bukan klaim token, DB kosong tidak galat), kontrak OpenAPI, smoke, uji mutasi (predikat cakupan/kunci cache dimatikan → tes gagal), dan tes web (kontrak fixture nyata, komponen). Mode `go test -short` kini melewati (bukan gagal) tes yang butuh Docker di semua paket integrasi.
