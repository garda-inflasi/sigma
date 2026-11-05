# 07 — Aplikasi lapangan (PWA offline-first)

**Prioritas:** P0 ("Input telemetri mobile") · **Status:** UI ✅ · Backend 🟡 · Terhubung ⬜ · Skema & seed ⬜ · Tes ⬜
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** `/lapangan/input` ([price-input-form.tsx](../../apps/web/src/components/lapangan/price-input-form.tsx)), `/lapangan/riwayat` ([riwayat-list.tsx](../../apps/web/src/components/lapangan/riwayat-list.tsx)), `/lapangan/tugas` ([tugas-list.tsx](../../apps/web/src/components/lapangan/tugas-list.tsx)), shell dengan indikator sinkron.
- **Offline:** antrean Zustand ([field-queue-store.ts](../../apps/web/src/lib/stores/field-queue-store.ts)), IndexedDB, [sync-engine.ts](../../apps/web/src/lib/offline/sync-engine.ts) idempoten (Idempotency-Key = `localId`, 409 → status KONFLIK), data acuan offline, [sw.js](../../apps/web/public/sw.js), [manifest.ts](../../apps/web/src/app/manifest.ts), halaman `/offline`.
- **API Go:** hanya `GET /v1/lapangan/tugas/saya`, `GET /v1/lapangan/tugas/:id`, `POST /v1/lapangan/tugas/bulk`. Route `lapangan` meminta izin `lapangan.lihat` yang **tidak dimiliki peran manapun**, jadi selalu 403 (diperbaiki di [02-A](02-pengguna-rbac.md)).
- **Skema:** `tugas_lapangan` (`id` VARCHAR, `judul`, `pasar_id`, `petugas_id`, `komoditas` TEXT[], `tipe`, `tenggat`, `status`, `target`). Entity tidak cocok: `id` `int64`, kolom `dibuat_pada`, `lokasi`, `pic` tidak ada. Tabel kosong. Sisi baik: `entri_harga.idempotency_key`, `foto_url`, `lat/lng`, `dalam_geofence` sudah tersedia.
- **Yang hilang:** sync engine memanggil **`POST /v1/lapangan/harga`, yang tidak ada**.

---

## 07-A — Tugas lapangan dari DB *(Fase P, urutan #7)*

**Tujuan:** `/lapangan/tugas` menampilkan tugas dari database untuk akun lapangan.
**Bergantung pada:** 06-A, 02-A.

1. **MIG**
   - [x] `tugas_lapangan`: tambah `dibuat_pada`, `dibuat_oleh_id`, `selesai_pada`, `jumlah_selesai`. Entity memakai `id` string; buang `lokasi`/`pic` (lokasi = pasar).
2. **SEED** (`demo`)
   - [x] Tugas untuk koordinator dan 10 akun mahasiswa (`mahasiswa_1`…`mahasiswa_10`, kabupaten sesuai seed pengguna) dengan campuran `BELUM/BERJALAN/SELESAI/TERLAMBAT` dan tenggat relatif `now`, meniru `mockTugas`.
3. **API**
   - [x] Selaraskan entity; `GET /v1/lapangan/tugas/saya` mengembalikan bentuk `TugasLapangan` (progres dihitung); izin sesuai hasil 02-A.
4. **FE**
   - [x] `useTugasSaya` ke API; cocokkan bentuk respons.
5. **TES**: tiap akun lapangan demo punya ≥ 1 tugas aktif; hapus `lapangan` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 07-B — Kirim harga dari lapangan: idempoten + konflik *(Fase F, gelombang F-1)*

**Tujuan:** sync engine yang sudah ada benar-benar menyimpan entri ke server dan menangani konflik.
**Bergantung pada:** 07-A, 06-A.

1. **MIG**
   - [x] Ganti `UNIQUE (idempotency_key)` global menjadi `UNIQUE (petugas_id, idempotency_key)` (migrasi 000060).
   - [x] `tugas_entri` (`tugas_id`, `entri_id`) agar progres tugas dihitung dari entri yang masuk.
   - [x] Index untuk deteksi bentrok `(pasar_id, komoditas_kode, tipe, dicatat_pada)`. Jendela bentrok: ±3 jam, petugas berbeda, harga berbeda, entri DRAF/DITOLAK diabaikan.
2. **SEED**: tidak ada (memakai histori 06-A).
3. **API**
   - [x] **`POST /v1/lapangan/harga`**: izin `harga.input`; hormati `Idempotency-Key` (kiriman ulang mengembalikan hasil yang sama); **409** bila bentrok beserta data pembanding.
   - [x] Validasi referensi, rentang waktu, dan koordinat di server; validasi otomatis (rentang, outlier, duplikat, geofence) di [06-C](06-harga-validasi.md). Foto belum diunggah (07-C): klien tidak lagi mengirim data URL foto.
   - [x] `GET /v1/lapangan/acuan` (komoditas, pasar+geofence, median resmi 7 hari per kabupaten) dengan ETag/`If-None-Match` (304). Unduhan inkremental parsial: seluruh paket diunduh ulang bila versi berubah.
4. **FE**
   - [x] Hubungkan sync engine dan UI penyelesaian konflik di riwayat ke payload 409.
   - [x] Pastikan `MODE_MOCK` tidak lagi aktif untuk alur ini.
5. **TES** (selesai): tes `sync-engine` (sukses, 409, 5xx, offline, idempotensi), tes `field-queue-store`, integrasi `POST` (kunci ganda, konflik, validasi). Saat ini nol tes.

---

## 07-C — Foto, notifikasi, kode etik, kinerja *(Fase F, gelombang F-1/F-4)*

1. **MIG**: `lampiran_entri` (`entri_id`, `lokasi_objek`, `mime`, `ukuran`, `checksum`), `perangkat_push` (`user_id`, `endpoint`, `p256dh`, `auth`), `persetujuan_kode_etik` (`user_id`, `versi`, `disetujui_pada`), view kinerja petugas (jumlah entri, ketepatan waktu, rasio penolakan).
2. **SEED** (`reference`): versi kode etik awal.
3. **API**: unggah foto (multipart/presigned URL, batas ukuran, pemindaian tipe, penyimpanan objek di luar DB); notifikasi tugas/pengingat; penugasan otomatis dan penyelesaian otomatis saat entri masuk.
4. **FE/PWA**: kompres foto di perangkat; batasi antrean IndexedDB; Background Sync saat jaringan kembali; hapus data lokal saat logout/ganti akun; persetujuan kode etik eksplisit; uji perangkat nyata (Android low-end, 2G/3G), Lighthouse (installable, offline).
5. **TES**: unggah foto, push, kode etik.
