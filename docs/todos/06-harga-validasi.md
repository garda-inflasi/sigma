# 06 — Validasi dan publikasi harga

**Prioritas:** P0 · **Status:** UI ✅ · Backend 🟡 · Terhubung 🟡 · Skema & seed ⬜ · Tes 🟡
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [harga-page.tsx](../../apps/web/src/components/harga/harga-page.tsx), [harga-table.tsx](../../apps/web/src/components/harga/harga-table.tsx), [validation-queue.tsx](../../apps/web/src/components/harga/validation-queue.tsx) di `/data/harga`: tabel entri, validasi/publikasi massal, detail dengan timeline.
- **API Go:** `GET /v1/harga`, `GET /v1/harga/tren`, `GET /v1/harga/:id`, `POST /v1/harga` (`harga.input`), `POST /v1/harga/bulk` (`harga.validasi`). Ada `usecase_test.go`.
- **Web:** `api.harga.list` → `/v1/harga?limit=500`; aksi massal lewat `/v1/{sumber}/bulk`; optimistic update + rollback di `useBulkAction`.
- **Skema:** `entri_harga` sudah kaya: `deviasi_pct`, `skor_anomali`, `dalam_geofence`, `lat/lng`, `foto_url`, `kode_pedagang`, `idempotency_key` (UNIQUE), `divalidasi_*`, `dipublikasi_*`, enum `status_entri` (`DRAF/DIAJUKAN/DIVALIDASI/DIPUBLIKASI/DITOLAK`). **Entity memakai kolom yang tidak ada:** `dibuat_pada`, `diupdate_pada`, `periode`. Tabel **kosong** setelah seed.

---

## 06-A — Harga dari DB dengan histori demo *(Fase P, urutan #6)*

**Tujuan:** `/data/harga` menampilkan histori harga dari database, dan menjadi sumber data untuk dashboard dan peta.
**Bergantung pada:** 04-A.

1. **MIG**
   - [ ] `entri_harga`: tambah `dibuat_pada` dan `diupdate_pada` (`DEFAULT now()`). Kolom `periode` dihapus dari entity (atau generated column `date_trunc('month', dicatat_pada)` bila memang dipakai query).
   - [ ] `CHECK (harga > 0)`; index parsial `WHERE status = 'DIAJUKAN'` untuk antrean validasi.
2. **SEED** (`demo`)
   - [ ] Histori ±90 hari untuk komoditas pilot × pasar utama dengan PRNG seed tetap, tanggal relatif `now`; sebaran status (`DIAJUKAN/DIVALIDASI/DIPUBLIKASI/DITOLAK/DRAF`); sisipkan outlier, duplikat, dan di luar geofence; `petugas_id` dari akun mahasiswa/koordinator.
   - [ ] Bentuk dan jumlah meniru `mockEntri` di `lib/api/mock.ts`.
3. **API**
   - [ ] Selaraskan entity dengan migrasi.
   - [ ] `GET /v1/harga`: filter (status, wilayah, komoditas, tanggal) + paginasi sisi server (UI memuat 500 baris sekaligus, tidak skala). `GET /v1/harga/tren`: agregasi harian/mingguan.
4. **FE**
   - [ ] `useEntriHarga` memakai paginasi/filter server; cocokkan bentuk respons dengan `EntriHarga`. Tren (`api.operasional.tren`) sudah memanggil `/v1/harga/tren`.
5. **TES**: integrasi list/tren/detail; hapus `harga` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 06-B — State machine, riwayat, pemisahan tugas *(Fase F, gelombang F-1)*

**Tujuan:** alur DRAF → DIAJUKAN → DIVALIDASI → DIPUBLIKASI/DITOLAK ditegakkan di backend dengan jejaknya.
**Bergantung pada:** 06-A, 03-A.

1. **MIG**
   - [ ] `alasan_penolakan`, `ditolak_oleh`, `ditolak_pada`.
   - [ ] `riwayat_status_entri` (`entri_id` FK, `dari`, `ke`, `oleh`, `waktu`, `alasan`).
   - [ ] **Pemisahan tugas di DB:** `CHECK (divalidasi_oleh IS DISTINCT FROM petugas_id)`.
2. **SEED** (`demo`): riwayat status untuk entri demo yang sudah divalidasi/dipublikasi/ditolak.
3. **API**
   - [ ] Transisi status yang sah per izin; aksi `bulk` menolak transisi ilegal dan mengembalikan `{ berhasil, gagal, rincian[] }` (id + sebab).
   - [ ] Pisahkan izin **publikasi** (`harga.publikasi`) dari validasi (`harga.validasi`); ekspor (`harga.ekspor`).
   - [ ] Alasan penolakan wajib dan terlihat oleh penginput. Row scope sesuai peran ([02-B](02-pengguna-rbac.md)). Tulis audit ([03-A](03-audit.md)).
4. **FE**: timeline detail dari `riwayat_status_entri`; dialog alasan penolakan; penanganan konflik 409 dari sinkronisasi lapangan di antrean validasi.
5. **TES**: semua kombinasi transisi sah/tidak sah; bulk dengan peran berbeda; validator tidak bisa memvalidasi entrinya sendiri.

---

## 06-C — Validasi otomatis *(Fase F, gelombang F-1)*

**Tujuan:** antrean validasi diprioritaskan oleh sinyal otomatis (PRD §1.1).
**Bergantung pada:** 06-A.

1. **MIG**
   - [ ] `batas_harga` (`komoditas_kode`, `kabupaten_kode` NULL = semua, `min`, `maks`, `berlaku_dari`).
   - [ ] `flag_validasi` JSONB pada `entri_harga` (outlier, duplikat, di-luar-geofence).
2. **SEED** (`reference`): `batas_harga` awal diturunkan dari `komoditas.harga_acuan`.
3. **API**: service validasi: rentang wajar, outlier (z-score/IQR terhadap histori dan sesama pasar), duplikasi (pasar+komoditas+pedagang+waktu), geofence terhadap `pasar.radius_meter`. Isi `skor_anomali`, `deviasi_pct`, `dalam_geofence`, `flag_validasi`.
4. **FE**: tampilkan bendera, foto, dan lokasi GPS pada panel detail; urutkan antrean menurut skor; ekspor CSV.
5. **TES**: kasus sisipan outlier/duplikat/geofence ditandai; `batas_harga` ada untuk tiap komoditas aktif.

---

## Backlog lanjutan

- [ ] Partisi bulanan `entri_harga` bila volume tumbuh (target: ribuan entri per hari se-Aceh).
- [ ] Versi entri dan lineage lengkap ([03-C](03-audit.md)).
