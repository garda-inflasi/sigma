# 10 — Peringatan dini (EWS)

**Prioritas:** P0 (PRD §2.2) · **Status:** UI ✅ · Backend 🟡 (baca, aturan, ubah status; belum rule engine) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [ews-page.tsx](../../apps/web/src/components/ews/ews-page.tsx) di `/ews` (tabel alert, aksi massal, ambang dan kanal), [ews-live-feed](../../apps/web/src/components/ews/ews-live-feed.tsx), toast, hook [use-ews-stream.ts](../../apps/web/src/components/ews/use-ews-stream.ts), store EWS.
- **API Go:** `GET /v1/ews` (filter `status`/`severity`/`jenis`/`kabupaten_kode`, paginasi `page`/`limit`, header `X-Total-Count`), `GET /v1/ews/:id`, `GET /v1/ews/aturan` (`ews.lihat`), dan `POST /v1/ews/bulk` (`ews.akui`; `eskalasi` menuntut `ews.eskalasi`; hanya transisi sah, dicatat di audit). Tes usecase, seeder, integrasi (Postgres sungguhan), dan asap.
- **Skema:** `ews_alert` (`id` VARCHAR, `severity`, `jenis`, `judul`, `pesan`, `komoditas_kode`, `kabupaten_kode`, `nilai`, `satuan_nilai`, `skor_dampak`, `status`, `sumber`, `dibuat_pada`, `diakui_*`, `aturan_id`) dan `aturan_ews` (ambang, `jendela_hari`, `kanal`, `aktif`, `keterangan`, `CHECK` waspada < siaga < kritis); entity cocok (migrasi 000030). Seeder `ews.reference` (9 aturan: pilot x lonjakan harga/stok/cuaca) dan `ews.demo` (17 alert).
- **Web:** `/ews` membaca alert dan aturan dari API (`lib/api/ews.ts`); simulasi acak hanya di `MODE_MOCK`. Ambang tampil baca-saja sampai 10-B.
- **Yang belum ada:** deteksi anomali statistik/ML, WhatsApp. Rule engine (10-B), SSE (10-C), dan notifikasi email/push (10-D) sudah ada.
- **Detail dan riwayat (✅, terhubung):** klik baris di `/ews` (atau tautan `/ews?id=…` dari feed dashboard) membuka [ews-detail-sheet.tsx](../../apps/web/src/components/ews/ews-detail-sheet.tsx): alert segar dari `GET /v1/ews/:id` (termasuk jejak diakui/dieskalasi/selesai, aturan) dan timeline `GET /v1/ews/:id/riwayat` (BARU, DIAKUI, DIESKALASI, SELESAI; siapa, kapan, catatan; pelaku kosong = Sistem). Aksi massal (akui/eskalasi/selesai) menyegarkan seluruh cache `ews`, termasuk detail dan riwayat yang terbuka. Di mode mock, riwayat disintesis dari status alert ([ews-mock.ts](../../apps/web/src/lib/api/ews-mock.ts)).
- **Matikan push (✅, terhubung):** mematikan kanal Push di kartu "Notifikasi saya" menyimpan preferensi lalu melepas langganan perangkat ini: `PushSubscription.endpoint` dikirim ke `DELETE /v1/ews/notifikasi/langganan-push`, baru `unsubscribe()` di browser (server lebih dulu agar bisa diulang bila gagal). 404 dari server (sudah tidak tersimpan) dianggap berhasil; browser tanpa dukungan push atau tanpa langganan di perangkat ini tidak memanggil apa pun ([push.ts](../../apps/web/src/lib/push.ts)). Tes: unit (`ews-push.test.ts`), API Go (`TestNotifikasi_PreferencesAndPushSubscriptionEndpoints`), e2e `detail.spec.ts`. Catatan: langganan sungguhan tidak bisa dibuat di Chromium headless tanpa layanan push, jadi e2e mencakup DELETE di level API dan toggle UI tanpa langganan.

---

## 10-A — Alert dan ambang dari DB *(Fase P, urutan #9)*

**Tujuan:** `/ews` menampilkan alert dan ambang dari database; aksi akui bekerja.
**Bergantung pada:** 04-A.

1. **MIG**
   - [x] `ews_alert`: tambah `aturan_id` (FK `aturan_ews`, nullable). Selaraskan entity ke tabel yang ada (D1): `id` string, `severity`, `kabupaten_kode`, `satuan_nilai`; buang kolom entity yang tidak ada.
   - [x] `aturan_ews`: `CHECK (ambang_waspada < ambang_siaga AND ambang_siaga < ambang_kritis)`.
2. **SEED**
   - [x] `reference`: `aturan_ews` default per komoditas pilot × jenis (lonjakan harga, stok, cuaca). Beri keterangan "ambang awal, harus dikalibrasi bersama BI/BPS".
   - [x] `demo`: `ews_alert` dengan severity/status beragam, ID berformat sama dengan `mockEws`, merujuk komoditas dan kabupaten yang ada di seed master. Tidak ada alert contoh di produksi.
3. **API**
   - [x] Entity + route `GET /v1/ews/aturan` (baca). `list`/`get`/`bulk` (akui) sesuai izin; filter dan paginasi server (UI memuat `limit=200`).
4. **FE**
   - [x] `useEwsList` ke API; kartu ambang dari `aturan_ews`; simulasi acak hanya aktif di `MODE_MOCK`.
5. **TES**: aturan seed lolos `CHECK`; seed dua kali tidak menggandakan alert; hapus `ews` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 10-B — Rule engine dan deteksi anomali *(Fase F, gelombang F-2)* — ✅ rule engine (z-score/ML belum)

**Tujuan:** alert dibuat otomatis dari data harga, bukan hanya seed.
**Bergantung pada:** 10-A, 06-A.

1. **MIG** (`000070_10b_ews_rule_engine.sql`)
   - [x] `ews_alert`: `dedup_key` dengan UNIQUE parsial `WHERE status <> 'SELESAI'`, `dieskalasi_pada`/`dieskalasi_oleh_id`, `selesai_pada`, sequence id `EWS-2000+`.
   - [x] `ews_riwayat` (`alert_id`, `dari`, `ke`, `oleh`, `waktu`, `catatan`); aksi massal akui/eskalasi/selesai ikut mencatat.
   - [x] `aturan_ews`: `kabupaten_kode` opsional, `cooldown_jam`, `versi` (optimistic locking), `diubah_oleh_id`, `diubah_pada`; `aturan_ews_riwayat` menyimpan foto tiap versi.
   - [x] Index `(status, severity, dibuat_pada DESC)`, `(komoditas_kode, kabupaten_kode, dibuat_pada DESC)`.
2. **API**
   - [x] `internal/ews/engine`: mengukur data nyata (harga konsumen tervalidasi/dipublikasi: kenaikan 24 jam terakhir vs rata-rata jendela; stok cold storage layak simpan: penurunan vs awal jendela; curah hujan kumulatif) menjadi alert idempoten per kondisi (`jenis|komoditas|kabupaten`), naik tingkat bila memburuk, peredaman (`cooldown_jam`) setelah ditutup. Terjadwal di `cmd/api` (`EWS_EVAL_INTERVAL`, bawaan 15m, `off` mematikan) dan manual `POST /v1/ews/evaluasi` (`ews.atur_ambang`).
   - [x] `PUT /v1/ews/aturan/:id` (`ews.atur_ambang`, 409 bila versi usang, audit), `GET /v1/ews/:id/riwayat`. Siklus hidup BARU, DIAKUI, DIESKALASI, SELESAI lewat aksi massal (`ews.akui`/`ews.eskalasi`).
   - [ ] Deteksi anomali statistik (z-score musiman/STL), model ML, evaluasi F1/AUC. Pemicu saat harga dipublikasi, serta pemicu siklus tekanan ([08-B](08-dashboard-operasional.md)): belum.
   - [x] Kartu intervensi otomatis dari alert: [14-C](14-intervensi.md).
3. **FE**: dialog ubah ambang (+kanal, jendela, peredaman, aktif) dan tombol "Evaluasi sekarang" untuk peran `ews.atur_ambang`. Simulasi "bagaimana jika": belum.
4. **TES**: unit engine (ambang, pesan, idempoten, galat), integrasi Postgres (lonjakan, naik tingkat, cooldown, stok, cuaca, aturan nonaktif, optimistic locking, audit, riwayat), seeder (riwayat, reseed tidak menimpa aturan yang sudah diubah).

---

## 10-C — Real-time (SSE) *(Fase F, gelombang F-2)* — ✅

1. **MIG** (`000071_10c_ews_event.sql`): `ews_event` (outbox peristiwa alert, id berurut = `Last-Event-ID`), ditulis dalam transaksi yang sama dengan alert, diserialkan dengan kunci advisory agar id tak pernah terlihat tidak berurutan. Redis tidak jadi sumber kebenaran: hanya membangunkan pembaca.
2. **API**: `GET /v1/ews/stream` (`ews.lihat`; Bearer atau cookie `garda_access`): event `alert` (alert baru atau naik tingkat), `retry: 5000`, heartbeat `: ping` 25 detik, `Last-Event-ID` (tanpa header mulai dari sekarang), batas tulis server dimatikan per koneksi. Antar-instance: Redis pub/sub (`REDIS_URL`), cadangan polling DB (30 detik dengan Redis, 3 detik tanpa). Peristiwa dibuang setelah 7 hari. Caddy: `flush_interval -1` pada `/api/v1/*`.
3. **FE**: `use-ews-stream` memetakan payload lewat `toEwsAlert` (`lib/api/ews-stream.ts`), mengganti alert yang sudah ada (naik tingkat) alih-alih menduplikasi, mengumumkan hanya alert baru/naik tingkat, dan menyegarkan daftar setelah tersambung ulang. Feed dashboard [08-B](08-dashboard-operasional.md): belum.
4. **TES**: Streamer (frame, resume, tanpa duplikat, polling, heartbeat, paginasi), Hub, RedisBus dengan miniredis (dua instance, Redis mati), integrasi HTTP nyata (cookie, 401/403, resume, melewati `WriteTimeout`), logika FE.

---

## 10-D — Notifikasi multi-kanal *(Fase F, gelombang F-4)* — ✅ email dan push; WhatsApp belum

1. **MIG** (`000072_10d_ews_notifikasi.sql`): `ews_notifikasi` (outbox; UNIQUE `alert x pengguna x kanal` membuat antrean idempoten; status `MENUNGGU`, `TERKIRIM`, `GAGAL`, `DICATAT`), `preferensi_notifikasi` (`pengguna_id`, `kanal`, `severity_min`, `aktif`), `langganan_push`.
2. **API** (`internal/ews/notifikasi`):
   - [x] Penerima = pengguna aktif berperan dengan izin `ews.akui`, dibatasi kanal aturan alert (`aturan_ews.kanal`), preferensi (bawaan aktif, minimum SIAGA), dan tingkat alert. Alert yang naik tingkat hanya menotifikasi yang baru memenuhi syarat.
   - [x] Adaptor yang dapat diganti (`domain.Pengirim`): email SMTP (`pkg/email`, aktif bila `SMTP_HOST`) dan Web Push (aktif bila `VAPID_PUBLIC_KEY`/`VAPID_PRIVATE_KEY`). Tanpa konfigurasi, tidak ada layanan luar yang dipanggil: baris outbox berstatus `DICATAT`.
   - [x] Pekerja pengiriman di `cmd/api` (`EWS_NOTIF_INTERVAL`, 30s): klaim dengan lease (aman multi-instance), retry dengan backoff 1m..1j, `GAGAL` setelah 5 percobaan; langganan push yang ditolak (404/410) dibuang.
   - [x] Eskalasi otomatis alert BARU >= SIAGA yang tidak diakui dalam N menit (`EWS_ESKALASI_MENIT`, bawaan 0 = mati): status DIESKALASI, riwayat oleh sistem, audit.
   - [x] `GET/PUT /v1/ews/notifikasi/preferensi`, `PUT/DELETE /v1/ews/notifikasi/langganan-push` (`ews.akui`, diaudit).
   - [ ] WhatsApp: kanal ada di kosakata tetapi belum ada adaptor maupun nomor telepon pengguna. Preferensi per peran (selain per pengguna): belum.
3. **FE**: kartu "Notifikasi saya" di `/ews` (kanal, tingkat minimum, penanda bila server belum mengonfigurasi pengiriman, tombol aktifkan push + handler push di `sw.js`).
4. **TES**: service dengan adaptor palsu (kirim, DICATAT, retry/backoff, GAGAL, eskalasi, preferensi), adaptor email (escape, tautan, timeout) dan Web Push terhadap endpoint localhost (enkripsi, VAPID, 410), integrasi Postgres (penerima per peran/preferensi/tingkat, idempoten, lease, eskalasi otomatis, endpoint 401/403/400).
