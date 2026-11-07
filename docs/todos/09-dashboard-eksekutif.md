# 09 — Dashboard eksekutif

**Prioritas:** P0 (Fase 2) · **Status:** UI ✅ · Backend 🟡 (data resmi di 09-B) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** `/eksekutif` memakai [inflation-hero](../../apps/web/src/components/eksekutif/inflation-hero.tsx) (koridor inflasi), [food-balance](../../apps/web/src/components/eksekutif/food-balance.tsx) (neraca pangan), [fiscal-space](../../apps/web/src/components/eksekutif/fiscal-space.tsx) (ruang fiskal), [xai-panel](../../apps/web/src/components/eksekutif/xai-panel.tsx), [exec-side](../../apps/web/src/components/eksekutif/exec-side.tsx).
- **API Go (`dashboard`):** `GET /v1/dashboard/eksekutif` (izin `dashboard.eksekutif`) → `RingkasanEksekutif`.
- **API Go (`analytics`, tidak dipakai web):** `/v1/dashboard/summary`, `/v1/inflation/trends`, `/v1/food-balance`, `/v1/fiscal-space`.
- **Kritis:** repository membaca tabel `ringkasan_eksekutif`, `komoditas_sumbangan`, `daerah_inflasi`, `tren_inflasi`, **semuanya tidak ada di migrasi**; `/v1/dashboard/eksekutif` error pada DB hasil migrasi. Tabel `ruang_fiskal` (`tahun`, `instrumen`, `pagu`, `realisasi`, `per_tanggal`, UNIQUE) ada tetapi tidak dirujuk kode.

### Kunci primer (000115, ✅)

- Tabel agregat dan time-series kini ber-PK **BIGINT identity**, dengan kunci alami lama menjadi **UNIQUE** dan target `ON CONFLICT` upsert impor dan seeder: `inflasi_bulanan` `UNIQUE (wilayah_kode, periode)`, `andil_inflasi` `UNIQUE (periode, wilayah_kode, komoditas_kode)`, `neraca_pangan` `UNIQUE (komoditas_kode, wilayah_kode, periode)`, `ruang_fiskal` `UNIQUE (tahun, instrumen, per_tanggal)`. `ruang_fiskal.id` dulu VARCHAR (`DEMO-RF-202609-1`): ID baru berurut menurut `diimpor_pada`, ID lama di `kode` (UNIQUE).
- FK: `andil_inflasi.komoditas_kode` dan `neraca_pangan.komoditas_kode` → `komoditas` **RESTRICT** (indeks `idx_andil_inflasi_komoditas_kode`; `neraca_pangan` terlayani awalan UNIQUE). `wilayah_kode` sengaja tanpa FK: berisi kode provinsi (2 digit) atau kabupaten/kota (4 digit), dijaga CHECK panjang.
- **Tidak ada rute `:id`**: dashboard eksekutif satu endpoint (`GET /v1/dashboard/eksekutif`) dan ID surrogate tidak pernah keluar di respons. Impor resmi (09-B) wajib upsert menurut UNIQUE alami di atas, bukan menurut `id`.

---

## 09-A — Eksekutif dari tabel yang benar, berlabel demo *(Fase P, urutan #14)*

**Tujuan:** `/eksekutif` tampil dari database. Data demo **berlabel jelas**; angka resmi tidak pernah di-seed.
**Bergantung pada:** 08-A. Panel XAI menunggu [11-A](11-prediksi-xai.md).

1. **MIG**
   - [x] `inflasi_bulanan` (`wilayah_kode`, `periode` DATE, `ihk`, `inflasi_mtm`, `inflasi_yoy`, `inflasi_ytd`, `sumber`, UNIQUE `(wilayah_kode, periode)`) menggantikan `ringkasan_eksekutif` dan `tren_inflasi`.
   - [x] `andil_inflasi` (`periode`, `wilayah_kode`, `komoditas_kode`, `andil`, `inflasi_pct`) menggantikan `komoditas_sumbangan`. `daerah_inflasi` tidak perlu tabel: tertinggi/terendah dihitung dari `inflasi_bulanan`.
   - [x] `neraca_pangan` (`komoditas_kode`, `wilayah_kode`, `periode`, `produksi_ton`, `konsumsi_ton`, `stok_ton`, kolom turunan surplus/defisit).
   - [x] `target_inflasi` (`tahun`, `batas_bawah`, `batas_atas`).
   - [x] `ruang_fiskal`: tambah `sumber`, `diimpor_pada`. Kolom `sumber` (`BPS`, `BI`, `DJPB`, `DEMO`) pada semua tabel di atas agar data demo dan resmi tidak tercampur.
2. **SEED**
   - [x] `reference`: `target_inflasi`.
   - [x] `demo`: inflasi, andil, neraca, dan ruang fiskal tiruan ber-`sumber='DEMO'`, bentuk mengikuti `mockEksekutif`. UI menampilkan banner "Data demo".
   - [x] **Produksi:** angka resmi hanya lewat impor (lihat 09-B di bawah); jangan seed angka yang tampak resmi.
3. **API**
   - [x] Tulis ulang query ke tabel baru; izin `fiskal.lihat` untuk ruang fiskal; selaraskan DTO dengan `RingkasanEksekutif`.
   - [x] Keputusan D4: hapus atau gabungkan route `analytics` yang menduplikasi.
4. **FE**: `api.eksekutif.ringkasan` sudah memanggil API; cocokkan bentuk; banner data demo. Panel XAI (`useXai`) menunggu 11-A.
5. **TES**: tidak ada baris `DEMO` setelah seed produksi; bentuk respons cocok; hapus entri dashboard dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 09-B — Data resmi dan fitur eksekutif *(Fase F, gelombang F-3)*

- [x] Impor inflasi/andil dari BPS lewat ingestion ([05-C](05-sumber-data.md)); angka resmi **didahulukan** atas baris `DEMO` di dashboard (tidak ada pencampuran dalam satu blok), baris `DEMO` tidak pernah menimpa baris resmi, dan label sumber serta waktu impor tampil di hero. Banner demo menyebut blok mana yang masih contoh dan mana yang sudah resmi. **Konektor BPS belum diverifikasi terhadap endpoint nyata** dan butuh `BPS_API_KEY` serta id variabel di konfigurasi sumber.
- [ ] Impor ruang fiskal dari DJPb dan neraca pangan: belum ada konektor (repository sudah mendahulukan baris resmi bila tabelnya terisi). Upsert impor memakai `ON CONFLICT (tahun, instrumen, per_tanggal)` untuk `ruang_fiskal` dan `ON CONFLICT (komoditas_kode, wilayah_kode, periode)` untuk `neraca_pangan`; kolom `kode` boleh NULL untuk baris impor.
- [ ] Koridor inflasi dan proyeksi ke depan memakai model ([11-B](11-prediksi-xai.md)).
- [ ] Filter wilayah, komoditas, periode (PRD §3.2); ringkasan EWS, rekomendasi AI, status intervensi (dengan cache).
- [ ] Ekspor tampilan ke PDF/gambar untuk rapat; "data per tanggal/jam" baru ada untuk inflasi (`diimpor_pada`).
- [x] Tes: usecase (golongan seri/penyumbang), integrasi (resmi menggantikan contoh walau lebih tua, semua blok resmi menghapus label demo, seeder demo tidak menimpa resmi). Tes izin (hanya peran strategis/manajerial) sudah dari 09-A.
