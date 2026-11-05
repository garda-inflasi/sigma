# 12 — Peta surplus–defisit & spasial

**Prioritas:** P1 (PRD §2.3) · **Status:** UI ✅ · Backend ⬜ · Terhubung 🟡 · Skema & seed ⬜ · Tes ⬜
**Cara kerja & DoD:** [00-cara-kerja.md](00-cara-kerja.md) · **Konteks data:** [19](19-migration-seeder.md)

## Yang sudah ada

- **UI:** [peta-page.tsx](../../apps/web/src/components/spasial/peta-page.tsx) di `/peta`; [region-map](../../apps/web/src/components/map/region-map.tsx) dan versi Mapbox, [marker-pin](../../apps/web/src/components/map/marker-pin.tsx). Surplus–defisit, rute distribusi, kandidat cold storage.
- **API Go (`spasial`):** satu endpoint `GET /v1/spasial/wilayah` (`spasial.lihat`).
- **Web:** `useMarkerWilayah` memakai API. Rute, kandidat cold storage, dan nama wilayah dari `lib/data/generators.ts` dan `reference.ts` (statis).
- **Skema:** entity `Wilayah` memetakan tabel **`wilayah` yang tidak ada**; data lokasi ada di `kabupaten` (`lat`, `lng`) dan `pasar`. Tidak ada geometri. Image Postgres di compose adalah `postgres:16-alpine` (tanpa PostGIS).

---

## 12-A — Peta dari DB *(Fase P, urutan #12)*

**Tujuan:** marker, surplus–defisit, rute, dan kandidat cold storage di `/peta` tampil dari database (data demo untuk rute/kandidat).
**Bergantung pada:** 06-A, 13-A.

1. **MIG**
   - [ ] `CREATE VIEW wilayah AS SELECT kode, nama, lat, lng, 'kabupaten' AS level FROM kabupaten` (atau ubah entity ke `kabupaten`).
   - [ ] `neraca_wilayah` (`komoditas_kode`, `wilayah_kode`, `periode`, `surplus_defisit_ton`), `rute_distribusi` (`asal`, `tujuan`, `komoditas_kode`, `volume_ton`, `biaya`, `jarak_km`, `status`), `kandidat_cold_storage` (`lokasi`, `skor`, `komponen_skor` JSONB, `alasan`, `dibuat_pada`).
2. **SEED** (`demo`)
   - [ ] Neraca (diturunkan dari data harga/stok demo), rute dan kandidat yang meniru generator di `lib/data/generators.ts`. Koordinat fasilitas cold storage sudah dari seeder cold-storage.
3. **API**
   - [ ] `GET /v1/spasial/wilayah` dari view; tambah `GET /v1/spasial/neraca`, `/rute`, `/kandidat-cold-storage` (izin `spasial.lihat`).
4. **FE**
   - [ ] `peta-page.tsx` mengganti generator statis dengan hook API; token Mapbox opsional dengan fallback SVG (dokumentasikan env).
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
