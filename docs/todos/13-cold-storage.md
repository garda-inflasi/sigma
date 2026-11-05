# 13 — Cold storage & food loss

**Prioritas:** P1 (PRD §2.4) · **Status:** UI ✅ · Backend ✅ · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

Fitur paling matang: migrasi, backend, BFF, seeder, dan UI sudah terhubung.

## Yang sudah ada

- **UI:** [cs-page.tsx](../../apps/web/src/components/cold-storage/cs-page.tsx), [facility-card](../../apps/web/src/components/cold-storage/facility-card.tsx), [stock-table](../../apps/web/src/components/cold-storage/stock-table.tsx) (paginasi, sort, pencarian, aksi massal), [lot-cells](../../apps/web/src/components/cold-storage/lot-cells.tsx).
- **BFF:** `app/api/cold-storage` dan `.../bulk`; klien [cold-storage.ts](../../apps/web/src/lib/api/cold-storage.ts).
- **API Go:** `GET /v1/cold-storage` (`coldstorage.lihat`), `POST /v1/cold-storage/bulk` (`coldstorage.kelola` atau `logistik.kelola`).
- **Skema:** `cold_storage` dan `stok_cold_storage` (000002); 000007 menambah `suhu_aktual_c`, `kedaluwarsa_pada` (diisi dari `komoditas.shelf_life_hari`), `CHECK` volume dan `risiko_food_loss` 0–1, dua index; `Down` ada. Entity cocok.
- **Seeder** `internal/cold-storage/seeder`, terdaftar di registry (`cmd/seed/register_coldstorage.go`): `coldstorage.reference` (5 fasilitas, aman di produksi) dan `coldstorage.demo` (23 lot; tanggal digeser mengikuti `now`, ditolak di produksi). Idempoten, diuji termasuk lewat PostgreSQL (`seeder_db_test.go`, coverage 94%).
- **Tes:** usecase 100%, handler 93,9%, seeder 94%; integrasi memakai seeder (`master.reference`, `auth.demo`, fasilitas, lot), bukan data migrasi; kasus asap `/v1/cold-storage`; tes web lolos.

---

## 13-A — Rapikan seeder ke registry *(Fase P, urutan #11; tanpa migration)*

**Tujuan:** fitur ini ikut aturan seeder (lapis, dependensi, guard produksi) tanpa mengubah perilakunya.
**Bergantung pada:** F0-2, F0-4.

1. **MIG**: tidak ada.
2. **SEED**
   - [x] Pisahkan isi seeder: fasilitas (`cold_storage`) = `reference`; lot dan umur stok yang digeser mengikuti `now` = `demo`. Seeder sekarang **menggeser tanggal lot setiap dijalankan**, jadi tidak boleh jalan di produksi (guard `APP_ENV`).
   - [x] Pindahkan 2 komoditas yang disisipkannya ke seed master ([04-A](04-master-data.md)).
   - [x] Naikkan coverage seeder (28,6%) dengan tes `BuildFasilitas` dan `BuildStok`.
3. **API/FE**: tidak ada perubahan perilaku.
4. **TES**
   - [x] Setelah F0-2, tes integrasi tidak bergantung pada akun dari 000006: pakai fixture atau seeder `demo` ([01-A](01-auth.md)).
5. **DoD** — checklist [00 §4](00-cara-kerja.md); tes asap F0-5 mencakup `/v1/cold-storage`.

---

## 13-B — Telemetri dan transaksi stok *(Fase F, gelombang F-5)*

1. **MIG** (`000100_13b_cold_storage_telemetri_transaksi.sql`)
   - [x] `telemetri_suhu` (`fasilitas_id`, `waktu`, `suhu_c`, `kelembapan_pct`, `sumber`; unik per fasilitas+waktu); `suhu_aktual_c`, `kelembapan_aktual_pct`, `telemetri_pada` pada `cold_storage` diturunkan dari baris terbaru oleh aplikasi.
   - [ ] Partisi bulanan dan retensi `telemetri_suhu` (belum; tabel biasa, perlu sebelum volume sensor nyata).
   - [x] `transaksi_stok` (`id`, `lot_id`, `lot_baru_id`, `jenis` MASUK/KELUAR/PINDAH, `volume_ton`, asal/tujuan, `oleh_id`, `waktu`, `alasan`); `stok_cold_storage.status` (AKTIF, PRIORITAS_JUAL, HABIS).
   - [x] `cold_storage`: batas alarm `suhu_min_c`/`suhu_maks_c` (NULL = target ± 2 °C). Kolom `aktif` tidak ditambah: status fasilitas sudah ada (`OPERASIONAL`/`PERAWATAN`/`RENCANA`). `Down` menghapus tabel telemetri/transaksi; fasilitas dan lot tidak tersentuh.
2. **SEED** (`coldstorage.demo`)
   - [x] Telemetri sintetis 48 jam (96 titik per fasilitas beroperasi, grid 30 menit; CS-04 punya lonjakan suhu yang sudah pulih) dan satu transaksi MASUK per lot demo. Jumlah baris tetap pada pengulangan (baris demo di luar jendela dibuang). Seeder `reference` tidak lagi menimpa `suhu_aktual_c`.
3. **API**
   - [x] `POST /v1/cold-storage/telemetri` (HTTP, idempoten, `coldstorage.kelola`; pembacaan di luar batas dilaporkan di respons dan audit).
   - [ ] Ingest MQTT dan alert otomatis ke EWS (menunggu rule engine [10-B](10-ews.md)).
   - [x] `POST /v1/cold-storage/transaksi` (MASUK/KELUAR/PINDAH, atomik dengan penguncian baris; 409 bila stok/kapasitas tak cukup atau fasilitas tak beroperasi), `PATCH /v1/cold-storage/fasilitas/:id` (kapasitas, batas alarm, status), `GET /v1/cold-storage/fasilitas/:id` (detail).
   - [ ] Membuat/menghapus fasilitas lewat API (hanya seeder/migrasi sejauh ini).
   - [x] Aksi `bulk` nyata: `tandai_prioritas_jual`, `buang` (hasil per lot di `rincian`). `pindahkan` massal belum ada (butuh tujuan per lot); pindah dilakukan per lot lewat `/transaksi`.
4. **FE**
   - [x] Panel detail fasilitas (riwayat suhu terhadap batas alarm, lot dengan peringatan kedaluwarsa, transaksi terbaru, formulir transaksi), lencana alarm suhu di kartu, aksi massal baru di tabel stok. BFF: `app/api/cold-storage/fasilitas/[id]` dan `.../transaksi`.
   - [x] Ekspor CSV hasil filter sudah ada di aksi massal tabel stok.
5. **TES**
   - [x] Konkurensi (keluar serentak dari satu lot, pindah sebagian serentak, masuk serentak melewati kapasitas, pindah silang tanpa deadlock); buku besar konsisten dengan volume lot; migrasi Up/Down dengan data; kontrak OpenAPI; otorisasi per jenis transaksi; unit validasi usecase.

---

## 13-C — Analitik pascapanen *(Fase F, Tahap 3)*

- [ ] Model shelf-life (target RMSE ≤ 1 hari, PRD §12) dan prediksi food loss + rekomendasi penyelamatan/olahan turunan (tabel `prediksi_food_loss` bila model dibuat; lihat [11](11-prediksi-xai.md)).
- [ ] Rekomendasi lokasi cold storage baru ([12-C](12-peta-spasial.md)).
- [ ] Integrasi siklus **Surplus**: deteksi over-supply → arahkan ke cold storage → kartu intervensi ([14-C](14-intervensi.md)).
- [ ] **Computer vision QC** (P1, Fase 3): klasifikasi mutu dari foto.
