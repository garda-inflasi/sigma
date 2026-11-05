# 10 — Peringatan dini (EWS)

**Prioritas:** P0 (PRD §2.2) · **Status:** UI ✅ · Backend 🟡 (hanya baca/ack) · Terhubung 🟡 · Skema & seed ⬜ · Tes 🟡
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [ews-page.tsx](../../apps/web/src/components/ews/ews-page.tsx) di `/ews` (tabel alert, aksi massal, ambang dan kanal), [ews-live-feed](../../apps/web/src/components/ews/ews-live-feed.tsx), toast, hook [use-ews-stream.ts](../../apps/web/src/components/ews/use-ews-stream.ts), store EWS.
- **API Go:** `GET /v1/ews`, `GET /v1/ews/:id` (`ews.lihat`), `POST /v1/ews/bulk` (`ews.akui`). Ada `usecase_test.go`.
- **Mode mock:** alert acak tiap 25–45 detik dan event `garda:simulasi-ews` dari palet perintah.
- **Skema:** `ews_alert` (`id` VARCHAR, `severity`, `jenis`, `judul`, `pesan`, `komoditas_kode`, `kabupaten_kode` NOT NULL, `nilai`, `satuan_nilai`, `skor_dampak`, `status`, `sumber`, `dibuat_pada`, `diakui_*`). **Entity tidak cocok:** `id` `int64`, `level` (tipe `level_ews` tidak ada), `skor`, `deskripsi`, `wilayah_kode`, `direkomendasikan_pada`, `ditindaklanjuti_pada`. `aturan_ews` (ambang waspada/siaga/kritis, `jendela_hari`, `kanal`, `aktif`) ada tetapi tidak dirujuk kode. Kedua tabel kosong.
- **Yang tidak ada:** pembuat alert (tidak ada deteksi lonjakan/anomali), `GET /v1/ews/stream` (SSE), Redis pub/sub di Go.

---

## 10-A — Alert dan ambang dari DB *(Fase P, urutan #9)*

**Tujuan:** `/ews` menampilkan alert dan ambang dari database; aksi akui bekerja.
**Bergantung pada:** 04-A.

1. **MIG**
   - [ ] `ews_alert`: tambah `aturan_id` (FK `aturan_ews`, nullable). Selaraskan entity ke tabel yang ada (D1): `id` string, `severity`, `kabupaten_kode`, `satuan_nilai`; buang kolom entity yang tidak ada.
   - [ ] `aturan_ews`: `CHECK (ambang_waspada < ambang_siaga AND ambang_siaga < ambang_kritis)`.
2. **SEED**
   - [ ] `reference`: `aturan_ews` default per komoditas pilot × jenis (lonjakan harga, stok, cuaca). Beri keterangan "ambang awal, harus dikalibrasi bersama BI/BPS".
   - [ ] `demo`: `ews_alert` dengan severity/status beragam, ID berformat sama dengan `mockEws`, merujuk komoditas dan kabupaten yang ada di seed master. Tidak ada alert contoh di produksi.
3. **API**
   - [ ] Entity + route `GET /v1/ews/aturan` (baca). `list`/`get`/`bulk` (akui) sesuai izin; filter dan paginasi server (UI memuat `limit=200`).
4. **FE**
   - [ ] `useEwsList` ke API; kartu ambang dari `aturan_ews`; simulasi acak hanya aktif di `MODE_MOCK`.
5. **TES**: aturan seed lolos `CHECK`; seed dua kali tidak menggandakan alert; hapus `ews` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 10-B — Rule engine dan deteksi anomali *(Fase F, gelombang F-2)*

**Tujuan:** alert dibuat otomatis dari data harga, bukan hanya seed.
**Bergantung pada:** 10-A, 06-A.

1. **MIG**
   - [ ] `ews_alert`: `dedup_key` dengan UNIQUE parsial `WHERE status <> 'SELESAI'`, `dieskalasi_pada`/`dieskalasi_oleh`, `selesai_pada`.
   - [ ] `ews_riwayat` (`alert_id`, `dari`, `ke`, `oleh`, `waktu`, `catatan`).
   - [ ] `aturan_ews`: `kabupaten_kode` opsional, `versi`, `diubah_oleh`, `diubah_pada` + riwayat perubahan ambang.
   - [ ] Index `(status, severity, dibuat_pada DESC)`, `(komoditas_kode, kabupaten_kode, dibuat_pada DESC)`.
2. **API**
   - [ ] Mesin aturan terjadwal dan dipicu publikasi harga: lonjakan harga (% harian/mingguan), kelangkaan stok cold storage, deviasi dari harga acuan; peredaman (cooldown) dan deduplikasi.
   - [ ] CRUD ambang (izin `ews.atur_ambang`); siklus hidup BARU → DIAKUI → DIESKALASI → SELESAI; eskalasi (`ews.eskalasi`).
   - [ ] Deteksi anomali statistik dulu (z-score musiman/STL); model ML belakangan. Target PRD §12: F1 ≥ 0,88, AUC-ROC ≥ 0,94.
   - [ ] Alert memicu siklus tekanan ([08-B](08-dashboard-operasional.md)) dan kartu intervensi ([14-C](14-intervensi.md)).
3. **FE**: UI ubah ambang + simulasi "bagaimana jika" pada data historis.
4. **TES**: rule engine dengan deret sintetis (lonjakan, plateau, data hilang); evaluasi F1/AUC pada data historis berlabel, hasil disimpan.

---

## 10-C — Real-time (SSE) *(Fase F, gelombang F-2)*

1. **MIG**: tabel `ews_outbox` (pola outbox) bila memilih `LISTEN/NOTIFY` alih-alih Redis. Redis sudah ada di `compose.infra.yaml`.
2. **API**: `GET /v1/ews/stream` (SSE, event `alert`) dengan autentikasi cookie/token, heartbeat, `Last-Event-ID`; pub/sub antar instance (Redis).
3. **FE**: uji `use-ews-stream` dengan SSE nyata (reconnect, tab tidak aktif); feed dashboard ([08-B](08-dashboard-operasional.md)).
4. **TES**: koneksi, reconnect, event tidak terduplikasi.

---

## 10-D — Notifikasi multi-kanal *(Fase F, gelombang F-4)*

1. **MIG**: `ews_notifikasi` (`alert_id`, `kanal`, `tujuan`, `status`, `percobaan`, `dikirim_pada`, `galat`), `preferensi_notifikasi` (`user_id`, `kanal`, `severity_min`, `aktif`).
2. **API**: email (`pkg/email` sudah ada), push (PWA), WhatsApp; retry; preferensi per pengguna/peran; eskalasi otomatis bila tidak diakui dalam N menit.
3. **FE**: halaman preferensi notifikasi.
4. **TES**: pengiriman dengan penyedia palsu, retry, eskalasi.
