# 12 — Peta surplus–defisit & spasial

**Prioritas:** P1 (PRD §2.3) · **Status:** UI ✅ · Backend 🟡 (baca-saja; belum geometri/optimasi) · Terhubung ✅ · Skema & seed ✅ · Tes ✅
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [peta-page.tsx](../../apps/web/src/components/spasial/peta-page.tsx) di `/peta`; [region-map](../../apps/web/src/components/map/region-map.tsx) dan versi Mapbox, [marker-pin](../../apps/web/src/components/map/marker-pin.tsx). Surplus–defisit, rute distribusi, kandidat cold storage.
- **API Go (`spasial`, izin `spasial.lihat`, baca-saja):** `GET /v1/spasial/wilayah` (marker + status dari neraca terbaru), `/neraca` (filter `komoditas`, `periode`), `/rute`, `/kandidat-cold-storage`.
- **Skema:** view `wilayah` di atas `kabupaten`, tabel `neraca_wilayah`, `rute_distribusi`, `kandidat_cold_storage` (migrasi 000032); entity cocok. Seeder `spasial.demo` (184 neraca, 5 rute, 3 kandidat; semua berlabel `sumber = 'DEMO'`, titik panas selaras dengan alert `ews.demo`). Tidak ada lapis `reference`: neraca pangan tidak boleh di-seed di produksi. Image Postgres tetap `postgres:16-alpine` (tanpa PostGIS).
- **Web:** `peta-page.tsx` membaca marker, rute, dan kandidat dari API (`lib/api/spasial.ts`); mode mock memakai `mockRute`/`mockKandidatColdStorage`. Mapbox opsional lewat `NEXT_PUBLIC_MAPBOX_TOKEN` (tanpanya peta memakai skema SVG Aceh; lihat `apps/web/README.md`).

---

## 12-A — Peta dari DB *(Fase P, urutan #12)*

**Tujuan:** marker, surplus–defisit, rute, dan kandidat cold storage di `/peta` tampil dari database (data demo untuk rute/kandidat).
**Bergantung pada:** 06-A, 13-A.

1. **MIG**
   - [x] `CREATE VIEW wilayah AS SELECT kode, nama, lat, lng, 'kabupaten' AS level FROM kabupaten` (atau ubah entity ke `kabupaten`).
   - [x] `neraca_wilayah` (`komoditas_kode`, `wilayah_kode`, `periode`, `surplus_defisit_ton`), `rute_distribusi` (`asal`, `tujuan`, `komoditas_kode`, `volume_ton`, `biaya`, `jarak_km`, `status`), `kandidat_cold_storage` (`lokasi`, `skor`, `komponen_skor` JSONB, `alasan`, `dibuat_pada`).
2. **SEED** (`demo`)
   - [x] Neraca (diturunkan dari data harga/stok demo), rute dan kandidat yang meniru generator di `lib/data/generators.ts`. Koordinat fasilitas cold storage sudah dari seeder cold-storage.
3. **API**
   - [x] `GET /v1/spasial/wilayah` dari view; tambah `GET /v1/spasial/neraca`, `/rute`, `/kandidat-cold-storage` (izin `spasial.lihat`).
4. **FE**
   - [x] `peta-page.tsx` mengganti generator statis dengan hook API; token Mapbox opsional dengan fallback SVG (dokumentasikan env).
5. **TES**: usecase/handler spasial (0 tes); hapus `spasial` dari `belumSelaras`.
6. **DoD** — checklist [00 §4](00-cara-kerja.md).

---

## 12-B — Geometri wilayah (PostGIS) *(Fase F, gelombang F-5)*

1. **MIG**
   - [ ] Ganti image ke `postgis/postgis:16-*` di `compose.infra.yaml` / `compose.prod.yaml`; `CREATE EXTENSION postgis`.
   - [ ] `wilayah_geometri` (`kode` PK, `level`, `geom geometry(MultiPolygon, 4326)`) + index GIST; simpan versi hasil `ST_SimplifyPreserveTopology` per tingkat zoom.
2. **SEED**: skrip impor batas kabupaten Aceh dari sumber resmi (BPS/Geoportal), catat lisensi; unduh saat seed, jangan commit berkas besar.
3. **API/FE**: layani geometri terkompresi; peta memakainya; klaster marker atau vector tile bila titik pasar ribuan.
4. **TES**: setiap kabupaten punya geometri; titik fasilitas berada di dalam poligon kabupatennya.

---

## 12-C — Optimasi rute dan skenario *(Fase F, gelombang F-5, PRD P1)*

- [ ] Matriks jarak jalan nyata (OSRM/OpenRouteService) di-cache di `jarak_antar_wilayah` (migration).
- [ ] Optimasi rute dan biaya logistik: model transportasi/min-cost flow (OR-Tools), lalu kendala kapasitas dan waktu.
- [ ] Skor kandidat cold storage: jarak ke sentra produksi, volume surplus, kapasitas listrik, akses jalan.
- [ ] Simulasi skenario pasokan, cuaca, dan kebijakan pada peta (what-if); filter komoditas dan periode; klik wilayah → detail (tren, EWS, stok terdekat).
- [ ] Tes optimasi dengan kasus kecil yang jawabannya diketahui; tes komponen peta (data kosong dan sangat banyak titik).
