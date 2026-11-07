# 07 — Aplikasi lapangan (PWA offline-first)

**Prioritas:** P0 ("Input telemetri mobile") · **Status:** UI ✅ · Backend 🟡 · Terhubung ⬜ · Skema & seed ⬜ · Tes ⬜
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** `/lapangan/input` ([price-input-form.tsx](../../apps/web/src/components/lapangan/price-input-form.tsx)), `/lapangan/riwayat` ([riwayat-list.tsx](../../apps/web/src/components/lapangan/riwayat-list.tsx)), `/lapangan/tugas` ([tugas-list.tsx](../../apps/web/src/components/lapangan/tugas-list.tsx)), shell dengan indikator sinkron.
- **Offline:** antrean Zustand ([field-queue-store.ts](../../apps/web/src/lib/stores/field-queue-store.ts)), IndexedDB, [sync-engine.ts](../../apps/web/src/lib/offline/sync-engine.ts) idempoten (Idempotency-Key = `localId`, 409 → status KONFLIK), data acuan offline, [sw.js](../../apps/web/public/sw.js), [manifest.ts](../../apps/web/src/app/manifest.ts), halaman `/offline`.
- **API Go:** hanya `GET /v1/lapangan/tugas/saya`, `GET /v1/lapangan/tugas/:id`, `POST /v1/lapangan/tugas/bulk`. Route `lapangan` meminta izin `lapangan.lihat` yang **tidak dimiliki peran manapun**, jadi selalu 403 (diperbaiki di [02-A](02-pengguna-rbac.md)).
- **Skema:** `tugas_lapangan` (`id`, `judul`, `pasar_id`, `petugas_id`, `komoditas` TEXT[], `tipe`, `tenggat`, `status`, `target`). Sejak migrasi 000111 `id` adalah BIGINT identity dan ID lama (`TGS-<slug>-<n>`) ada di `kode`; lihat [Kunci primer](#kunci-primer-tugas_lapangan-bigint-000111). Entity tidak cocok: `id` `int64`, kolom `dibuat_pada`, `lokasi`, `pic` tidak ada. Tabel kosong. Sisi baik: `entri_harga.idempotency_key`, `foto_url`, `lat/lng`, `dalam_geofence` sudah tersedia.
- **Yang hilang:** sync engine memanggil **`POST /v1/lapangan/harga`, yang tidak ada**.
- **Detail tugas (✅, terhubung):** `GET /v1/lapangan/tugas/:id` kini mengembalikan `rincian_komoditas` (kode, nama, `terisi` = ada entri harga tertaut lewat `tugas_entri`). Pemilik tugas atau pemegang `tugas.kelola` boleh membaca; tugas petugas lain 404 bagi mahasiswa. UI: tombol judul tugas di `/lapangan/tugas` membuka lembar bawah [tugas-detail-sheet.tsx](../../apps/web/src/components/lapangan/tugas-detail-sheet.tsx) (progres, tenggat, daftar komoditas sudah/belum diisi, tautan "Mulai isi" ke `/lapangan/input` dengan parameter yang sama dengan daftar lewat `hrefIsiTugas`). Sengaja lembar, bukan rute baru: saat offline navigasi ke rute yang belum pernah dibuka jatuh ke `/offline`, sedangkan lembar memakai baris daftar dan tetap menampilkan komoditas serta tautan isi. Catatan data demo: `jumlah_selesai` hasil seed hanya penghitung, tanpa entri tertaut, sehingga rincian per komoditas pada tugas seed menampilkan "Belum diisi" sampai petugas benar-benar mengirim harga.

---

## Kunci primer: `tugas_lapangan` BIGINT (000111)

Migrasi `000111_pk_harga_lapangan_master.sql`; kontrak lengkap di [SKEMA-DATABASE](../SKEMA-DATABASE.md).

- **`tugas_lapangan.id`** adalah `BIGINT GENERATED ALWAYS AS IDENTITY` (operasional). Di JSON ditulis sebagai **string desimal** (`"42"`), tipe Go `idx.ID`. ID baru menaik menurut `dibuat_pada`. **`kode varchar(64) UNIQUE`** menyimpan ID lama (`TGS-<slug>-<n>`) dan dikirim sebagai field `kode`; tugas yang dibuat sesudah migrasi berkode NULL (belum ada fitur penugasan yang memberi kode tampilan). UI menampilkan `kode ?? id` (label "Tugas ..." di form input). Seeder demo memakai `kode` sebagai kunci idempoten (`ON CONFLICT (kode)`).
- **Rute `:id`**: `GET /v1/lapangan/tugas/:id` memakai `idx.ParamIDAtauKode` (maks. 64 karakter): ID BIGINT **atau** kode lama. Teks kosong, terlalu panjang, atau ID yang tidak ada/bukan milik pemanggil dijawab `404`.
- **Aksi massal** `POST /v1/lapangan/tugas/bulk`: `ids` berisi ID BIGINT (string angka) **atau kode lama** `TGS-...` (klien yang masih memegang ID lama, mis. daftar tugas yang di-cache sebelum migrasi, tetap dilayani; hasil dihitung per rujukan). Rujukan lebih dari 64 karakter atau daftar efektif kosong ditolak `400`. Audit `TUGAS_<AKSI>` mencatat ID BIGINT yang berubah.
- **Idempotensi** kiriman harga tetap lewat `Idempotency-Key` = `localId` → `entri_harga.idempotency_key` (`UNIQUE (petugas_id, idempotency_key)`), tidak terkait ID tugas.
- **Kompatibilitas antrean offline lama**: antrean IndexedDB (`EntriLokal`) hanya menyimpan `localId` dan kolom entri (komoditas, pasar `PSR-...`, tipe, harga, ...); tidak ada ID tugas dan tidak ada ID entri di dalamnya, dan `POST /v1/lapangan/harga` tidak membawa ID tugas (server menautkan entri ke tugas menurut petugas, pasar, tipe, dan komoditas). Jadi antrean lama tersinkron tanpa perubahan, dan respons server (`id` entri string angka, `versi_server.id`) tidak disimpan klien. Satu-satunya jejak ID tugas di klien adalah parameter `?tugas=` pada `/lapangan/input` (label teks) dan cache daftar tugas; keduanya aman karena rute memakai alias kode lama.
- **FK**: `pasar_id` → `pasar` `RESTRICT`; `petugas_id`, `dibuat_oleh_id` → `pengguna` `RESTRICT` (000110). Anak: `tugas_entri(tugas_id, entri_id)`, PK komposit, kedua kolom `bigint`, keduanya `ON DELETE CASCADE` (tugas atau entri dihapus, tautannya ikut hilang). Tugas tidak punya endpoint hapus; bila ada, penghapusan tugas hanya menghapus tautan, bukan entri.
- **Indeks**: `idx_tugas_lapangan_petugas (petugas_id, status)` untuk `GET /v1/lapangan/tugas/saya`; parsial tugas aktif `idx_tugas_lapangan_aktif (petugas_id, tenggat) WHERE status <> 'SELESAI'` untuk penautan entri ke tugas dan tandai selesai; `idx_tugas_lapangan_pasar_tenggat (pasar_id, tenggat)`.
- **Rujukan lunak**: `audit_log.entitas_id` pada `entitas = 'tugas_lapangan'` dipetakan ke ID baru bila sama dengan `kode`.

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

1. **MIG**: `lampiran_entri` (PK `BIGINT` identity; `entri_id bigint NOT NULL REFERENCES entri_harga(id) ON DELETE CASCADE` + indeks `idx_lampiran_entri_entri_id`; `lokasi_objek`, `mime`, `ukuran`, `checksum`), `perangkat_push` (sudah dipenuhi `langganan_push` di 000110: `pengguna_id uuid` `CASCADE`, `UNIQUE (endpoint)`; tidak perlu tabel baru), `persetujuan_kode_etik` (PK `BIGINT` identity; `user_id uuid NOT NULL REFERENCES pengguna(id) ON DELETE CASCADE`, `versi`, `disetujui_pada`, `UNIQUE (user_id, versi)`), view kinerja petugas (jumlah entri, ketepatan waktu, rasio penolakan; memakai `entri_harga.petugas_id uuid`). Penyimpanan objek foto merujuk `lampiran_entri.id` (BIGINT, string di JSON).
2. **SEED** (`reference`): versi kode etik awal.
3. **API**: unggah foto (multipart/presigned URL, batas ukuran, pemindaian tipe, penyimpanan objek di luar DB; rute `POST /v1/lapangan/harga/:id/foto` dengan `:id` = `entri_harga.id` BIGINT, ID salah bentuk `404`, bukan pemilik entri `404`); notifikasi tugas/pengingat; penugasan otomatis (tugas baru: `kode` NULL kecuali ada kebutuhan kode tampilan, lihat bagian Kunci primer) dan penyelesaian otomatis saat entri masuk.
4. **FE/PWA**: kompres foto di perangkat; batasi antrean IndexedDB; Background Sync saat jaringan kembali; hapus data lokal saat logout/ganti akun; persetujuan kode etik eksplisit; uji perangkat nyata (Android low-end, 2G/3G), Lighthouse (installable, offline).
5. **TES**: unggah foto, push, kode etik.
