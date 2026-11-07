# 06 — Validasi dan publikasi harga

**Prioritas:** P0 · **Status:** UI ✅ · Backend 🟡 · Terhubung 🟡 · Skema & seed ⬜ · Tes 🟡
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [harga-page.tsx](../../apps/web/src/components/harga/harga-page.tsx), [harga-table.tsx](../../apps/web/src/components/harga/harga-table.tsx), [validation-queue.tsx](../../apps/web/src/components/harga/validation-queue.tsx) di `/data/harga`: tabel entri, validasi/publikasi massal, detail dengan timeline.
- **API Go:** `GET /v1/harga`, `GET /v1/harga/tren`, `GET /v1/harga/:id`, `GET /v1/harga/:id/riwayat`, `GET /v1/harga/:id/versi`, `GET /v1/harga/:id/lineage`, `POST /v1/harga` (`harga.input`), `POST /v1/harga/bulk` (`harga.validasi`). Ada `usecase_test.go`. `:id` adalah ID BIGINT atau kode lama, lihat [Kunci primer](#kunci-primer-entri_harga-bigint-000111).
- **Web:** `api.harga.list` → `/v1/harga?limit=500`; aksi massal lewat `/v1/{sumber}/bulk`; optimistic update + rollback di `useBulkAction`.
- **Skema:** `entri_harga` sudah kaya: `deviasi_pct`, `skor_anomali`, `dalam_geofence`, `lat/lng`, `foto_url`, `kode_pedagang`, `idempotency_key` (UNIQUE bersama `petugas_id`), `divalidasi_*`, `dipublikasi_*`, enum `status_entri` (`DRAF/DIAJUKAN/DIVALIDASI/DIPUBLIKASI/DITOLAK`). Sejak migrasi 000111 `id` adalah BIGINT identity dan ID lama ada di `kode`. **Entity memakai kolom yang tidak ada:** `dibuat_pada`, `diupdate_pada`, `periode`. Tabel **kosong** setelah seed.

---

## 06-A — Harga dari DB dengan histori demo *(Fase P, urutan #6)*

**Tujuan:** `/data/harga` menampilkan histori harga dari database, dan menjadi sumber data untuk dashboard dan peta.
**Bergantung pada:** 04-A.

1. **MIG**
   - [x] `entri_harga`: tambah `dibuat_pada` dan `diupdate_pada` (`DEFAULT now()`). Kolom `periode` dihapus dari entity (atau generated column `date_trunc('month', dicatat_pada)` bila memang dipakai query).
   - [x] `CHECK (harga > 0)`; index parsial `WHERE status = 'DIAJUKAN'` untuk antrean validasi.
2. **SEED** (`demo`)
   - [x] Histori ±90 hari untuk komoditas pilot × pasar utama dengan PRNG seed tetap, tanggal relatif `now`; sebaran status (`DIAJUKAN/DIVALIDASI/DIPUBLIKASI/DITOLAK/DRAF`); sisipkan outlier, duplikat, dan di luar geofence; `petugas_id` dari akun mahasiswa/koordinator.
   - [x] Bentuk dan jumlah meniru `mockEntri` di `lib/api/mock.ts`.
3. **API**
   - [x] Selaraskan entity dengan migrasi.
   - [x] `GET /v1/harga`: filter (status, wilayah, komoditas, tanggal) + paginasi sisi server (UI memuat 500 baris sekaligus, tidak skala). `GET /v1/harga/tren`: agregasi harian/mingguan.
4. **FE**
   - [x] `useEntriHarga` memakai paginasi/filter server; cocokkan bentuk respons dengan `EntriHarga`. Tren (`api.operasional.tren`) sudah memanggil `/v1/harga/tren`.
5. **TES**: integrasi list/tren/detail; hapus `harga` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 06-B — State machine, riwayat, pemisahan tugas *(Fase F, gelombang F-1)*

**Tujuan:** alur DRAF → DIAJUKAN → DIVALIDASI → DIPUBLIKASI/DITOLAK ditegakkan di backend dengan jejaknya.
**Bergantung pada:** 06-A, 03-A.

1. **MIG**
   - [x] `alasan_penolakan`, `ditolak_oleh`, `ditolak_pada`.
   - [x] `riwayat_status_entri` (`entri_id` FK, `dari`, `ke`, `oleh`, `waktu`, `alasan`).
   - [x] **Pemisahan tugas di DB:** `CHECK (divalidasi_oleh IS DISTINCT FROM petugas_id)`.
2. **SEED** (`demo`): riwayat status untuk entri demo yang sudah divalidasi/dipublikasi/ditolak.
3. **API**
   - [x] Transisi status yang sah per izin (termasuk aksi `ajukan` DRAF → DIAJUKAN oleh pencatat); aksi `bulk` menolak transisi ilegal dan mengembalikan `{ berhasil, gagal, rincian[] }` (id + sebab).
   - [x] Pisahkan izin **publikasi** (`harga.publikasi`) dari validasi (`harga.validasi`); ekspor (`harga.ekspor`).
   - [x] Alasan penolakan wajib dan terlihat oleh penginput. Tulis audit ([03-A](03-audit.md)).
   - [ ] Row scope sesuai peran ([02-B](02-pengguna-rbac.md)); menunggu 02-B. Entri di luar scope dijawab `404` (sama dengan tidak ada) pada `GET /v1/harga/:id` dan `GET /v1/harga/:id/riwayat`; `:id` tetap BIGINT dengan alias kode lama (`idx.ParamIDAtauKode`), dan aksi massal melewatkan entri di luar scope sebagai `tidak_ditemukan` di `rincian`.
4. **FE**: timeline detail dari `riwayat_status_entri`; dialog alasan penolakan; penanganan konflik 409 di antrean validasi menyusul bersama bendera `bentrok` di 06-C.
5. **TES**: semua kombinasi transisi sah/tidak sah; bulk dengan peran berbeda; validator tidak bisa memvalidasi entrinya sendiri.

---

## 06-C — Validasi otomatis *(Fase F, gelombang F-1)*

**Tujuan:** antrean validasi diprioritaskan oleh sinyal otomatis (PRD §1.1).
**Bergantung pada:** 06-A.

1. **MIG**
   - [x] `batas_harga` (`komoditas_kode`, `kabupaten_kode` NULL = semua, `batas_min`, `batas_maks`, `berlaku_dari`).
   - [x] `flag_validasi` JSONB pada `entri_harga` (outlier, duplikat, di-luar-geofence).
2. **SEED** (`reference`): `batas_harga` awal diturunkan dari `komoditas.harga_acuan`.
3. **API** (selesai): service validasi: rentang wajar, outlier (z-score robust, median dan MAD histori resmi 30 hari sekabupaten), duplikasi (pasar+komoditas+pedagang+waktu), geofence terhadap `pasar.radius_meter`. Isi `skor_anomali`, `deviasi_pct`, `dalam_geofence`, `flag_validasi`.
4. **FE** (selesai; foto belum tampil, unggahan di 07-C): tampilkan bendera, foto, dan lokasi GPS pada panel detail; urutkan antrean menurut skor; ekspor CSV.
5. **TES** (selesai): kasus sisipan outlier/duplikat/geofence ditandai; `batas_harga` ada untuk tiap komoditas aktif.

---

## Kunci primer: `entri_harga` BIGINT (000111)

Migrasi `000111_pk_harga_lapangan_master.sql`; kontrak lengkap di [SKEMA-DATABASE](../SKEMA-DATABASE.md).

- **`entri_harga.id`** adalah `BIGINT GENERATED ALWAYS AS IDENTITY` (log transaksional time-series). Di JSON ditulis sebagai **string desimal** (`"1024"`); tipe Go `idx.ID`. ID baru menaik menurut `dicatat_pada`, lalu `dibuat_pada`, lalu ID lama. Aplikasi tidak lagi membuat ID (tidak ada `uuid.NewString()`): basis data yang memberi, dan GORM membacanya lewat `RETURNING`.
- **`kode varchar(64) UNIQUE`** menyimpan ID lama (`HRG-D00-00-01-K`, atau UUID untuk entri buatan aplikasi lama) dan dikirim sebagai field `kode`; entri baru `kode` NULL. UI menampilkan `kode ?? id` (ekspor CSV kolom "ID"). Seeder demo memakai `kode` sebagai kunci idempoten (`ON CONFLICT (kode)`), jadi basis data hasil migrasi dan basis data baru memberi hasil yang sama.
- **Rute `:id`**: `GET /v1/harga/:id` dan `GET /v1/harga/:id/riwayat` memakai `idx.ParamIDAtauKode` (maks. 64 karakter): ID BIGINT **atau** kode lama (tautan dan bookmark lama tetap berlaku). Teks kosong, lebih dari 64 karakter, atau ID bentuk sah yang tidak ada dijawab `404` tanpa membocorkan alasan. Aksi per entri yang direncanakan (`POST /v1/harga/:id/validasi|tolak|publikasi`, `GET /v1/harga/:id/audit`) memakai `:id` yang sama (BIGINT, alias kode lama); sampai ada, validasi, tolak, kembalikan, dan publikasi berjalan lewat `POST /v1/harga/bulk`.
- **Aksi massal** (`ids` di body): daftar ID BIGINT sebagai **string angka** (`["1024","1025"]`), dibaca `idx.ParseDaftar` (spasi dirapikan, ganda dibuang). Satu ID yang salah bentuk, termasuk kode lama `HRG-...`, menolak seluruh permintaan dengan `400` (tidak ada entri yang berubah); ID bentuk sah yang tidak ada dilaporkan `tidak_ditemukan` di `rincian`. Tampilan yang masih membawa ID lama harus dimuat ulang.
- **Idempotensi sinkron offline** tetap lewat `Idempotency-Key` → `entri_harga.idempotency_key`, `UNIQUE (petugas_id, idempotency_key)`; tidak berubah.
- **FK keluar**: `pasar_id` → `pasar` `RESTRICT`, `komoditas_kode` → `komoditas` `RESTRICT`, pelaku → `pengguna` `RESTRICT` (000110: akun yang masih menjadi bukti asal data dijawab `409` saat dihapus dan disarankan dinonaktifkan). **FK masuk**: `riwayat_status_entri.entri_id` (`bigint`, `CASCADE`) dan `tugas_entri.entri_id` (`bigint`, `CASCADE`). Entri tidak punya endpoint hapus (entri yang sudah terbit tidak boleh hilang); bila suatu saat ada, hapus yang dirujuk `RESTRICT` dijawab `409`.
- **Indeks**: BRIN `idx_entri_harga_dicatat_brin (dicatat_pada)`; `idx_entri_harga_komoditas_pasar (komoditas_kode, pasar_id, dicatat_pada)` untuk filter komoditas+wilayah+tanggal dan tren; `idx_entri_harga_status (status, dicatat_pada)`; `idx_entri_harga_petugas_dicatat (petugas_id, dicatat_pada DESC)` untuk `/v1/harga/saya`; parsial antrean validasi `WHERE status = 'DIAJUKAN'`: `idx_entri_harga_antrean (dicatat_pada DESC)` dan `idx_entri_harga_antrean_skor (skor_anomali DESC, dicatat_pada DESC, id)` untuk `urut=skor`; `idx_entri_harga_bentrok` untuk deteksi bentrok.
- **Rujukan lunak**: `audit_log.entitas_id` pada `entitas = 'entri_harga'` dipetakan ke ID baru (`id::text`) bila sama dengan `kode`; nilai lain (`ENT-xxxx`, `3 entri`) dibiarkan. Audit baru mencatat ID BIGINT. Materialized view `agregat_harga_harian` dan `mv_kpi_operasional` tidak memakai `entri_harga.id`, jadi tidak tersentuh.

## Backlog lanjutan

- [ ] Partisi bulanan `entri_harga` bila volume tumbuh (target: ribuan entri per hari se-Aceh). Catatan kunci: partisi deklaratif mewajibkan kolom partisi (`dicatat_pada`) masuk PK, sehingga PK menjadi `(id, dicatat_pada)` dan FK anak ke `entri_harga(id)` (`riwayat_status_entri`, `tugas_entri`) harus dirancang ulang (mis. FK komposit); BRIN dan indeks di atas cukup sampai puluhan juta baris, jadi tinjau ukuran dulu.
- [x] Versi entri dan lineage lengkap ([03-C](03-audit.md)). Migrasi `000118_03c_lineage_versi_entri_harga.sql` menambah `entri_harga_versi` (PK `BIGINT` identity, `entri_id bigint NOT NULL REFERENCES entri_harga(id) ON DELETE CASCADE`, indeks FK lewat `uq_entri_harga_versi_entri_versi (entri_id, versi)`, `oleh` → `pengguna` RESTRICT). Pemicu `trg_entri_harga_versi` menulis satu versi tiap perubahan isi `entri_harga` (aksi INPUT, AJUKAN, VALIDASI, PUBLIKASI, TOLAK, KEMBALIKAN, UBAH; MIGRASI untuk backfill baris yang sudah ada), dengan `perubahan` (kolom non-waktu yang berubah) dan salinan baris; pergeseran kolom waktu saja (seeder demo) tidak membuat versi, jadi `make db/seed/demo` dua kali tidak menambah baris versi. Pelaku dari GUC `garda.aktor_id` yang diisi `Repository.Transisi` (KEMBALIKAN tidak punya kolom pelaku), selain itu diturunkan dari baris. Tabel append-only (UPDATE, DELETE, TRUNCATE ditolak; hanya CASCADE dari `entri_harga`). Rute `GET /v1/harga/:id/versi` dan `GET /v1/harga/:id/lineage` (`harga.lihat` atau `harga.input` seperti `/riwayat`; `:id` BIGINT beralias kode lama; entri petugas lain `404`). Lineage memuat asal, cek otomatis, blok validasi/publikasi/penolakan, versi, riwayat status, dan rujukan `audit_log` (maks. 100). Panel detail `/data/harga` menampilkan bagian "Lineage data" dengan tautan ke `/audit?entitas=entri_harga&entitas_id=<id>`. Catatan: entri seed demo bermula dari satu versi INPUT yang memuat status akhirnya (seeder menulis baris sekali jalan), riwayat status tetap lengkap di `riwayat_status_entri`.
