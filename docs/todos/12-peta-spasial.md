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

> **Keputusan (12-B): GeoJSON `jsonb`, bukan PostGIS.** Image bersama tetap `postgres:16-alpine`: demo Docker yang berjalan, `compose.infra.yaml`, dan `pgtest` memakai image itu; mengganti ke `postgis/postgis` menambah unduhan image, mengubah lingkungan bersama, dan hanya bernilai bila perlu query spasial berat (yang belum ada). Validitas dijaga CHECK basis data + validator Go (`domain.ParseGeoJSON`); jarak (haversine), titik-di-poligon, kotak pembatas, dan penyederhanaan (Douglas-Peucker) ada di `internal/spasial/domain/geo.go`. Bila nanti perlu `ST_*`, `wilayah_geometri.geojson` dapat dimigrasikan ke kolom `geometry` tanpa mengubah API.

1. **MIG** (`000101_12b_wilayah_geometri.sql`)
   - [x] `wilayah_geometri` (`kabupaten_kode` PK, `geojson` jsonb, kotak pembatas, `sumber` DEMO/RESMI, `keterangan`) + CHECK tipe/bbox/sumber.
   - [ ] Versi geometri tersimpan per tingkat zoom: tidak dibuat; penyederhanaan dihitung saat dibaca (23 poligon, murah).
2. **SEED** (`spasial.demo`)
   - [x] Poligon **perkiraan** (selimut cembung di sekitar titik pusat, pasar, dan cold storage), berlabel `DEMO`; **bukan batas administratif**. Data batas resmi (BPS/Geoportal) tidak tersedia offline dan tidak diklaim.
   - [ ] Impor batas resmi: seeder/impor `RESMI` belum ada (perlu sumber dan lisensi yang dicatat; unduh saat impor, jangan commit berkas besar).
3. **API/FE**
   - [x] `GET /v1/spasial/geometri?zoom=&bbox=` (FeatureCollection, disederhanakan menurut zoom, `perkiraan` per fitur) dan `GET /v1/spasial/lokasi?lat=&lng=` (wilayah untuk satu titik).
   - [x] Peta skema SVG menggambar poligon berwarna menurut status marker; lapisan Mapbox (fill + line) bila token ada (lapisan Mapbox tidak teruji otomatis, tanpa token). Keterangan "perkiraan" tampil di peta.
   - [ ] Klaster marker / vector tile (belum perlu: pasar puluhan titik).
4. **TES**
   - [x] Setiap kabupaten/kota punya geometri valid; semua pasar dan cold storage berada di dalam poligon kabupatennya; CHECK menolak GeoJSON rusak; unit geometri (valid/invalid, lubang, bbox, haversine, penyederhanaan, selimut cembung); integrasi izin, zoom, bbox, parameter salah.

---

## 12-C — Optimasi rute dan skenario *(Fase F, gelombang F-5, PRD P1)*

- [x] Cache matriks jarak `jarak_antar_wilayah` (migrasi `000102_12c_optimasi_rute.sql`): `ESTIMASI` (garis lurus × faktor jalan 1,35, 45 km/jam) diisi saat rencana disimpan; `OSRM` disediakan untuk impor jarak jalan nyata dan tidak ditimpa estimasi. **Impor OSRM/OpenRouteService belum ada** (butuh layanan routing eksternal).
- [x] Optimasi rute dan biaya logistik: model transportasi min-cost flow eksak (successive shortest path, murni Go, `internal/spasial/domain/optimasi.go`; bukan OR-Tools). Kapasitas = surplus/defisit `neraca_wilayah`; kendala waktu = `maks_jam_tempuh`. Volume tersalur dimaksimalkan lebih dulu, lalu ton-km diminimalkan. Kendala kapasitas kendaraan belum dimodelkan. Setiap perhitungan juga menghitung rencana pembanding serakah (`domain.Serakah`, pasangan terdekat dulu) dan melaporkan `penghematan_persen`; biayanya disimpan di `optimasi_rute.biaya_pembanding` (migrasi `000103`, dasar KPI efisiensi biaya di [16-B](16-kpi.md)).
- [x] API: `POST /v1/spasial/optimasi-rute` (simulasi `spasial.lihat`; `simpan` butuh `logistik.kelola`, menulis `optimasi_rute` + rute usulan `sumber = OPTIMASI`, menggantikan rencana lama komoditas/periode yang sama, audit `OPTIMASI_RUTE`), `GET /v1/spasial/optimasi-rute` (riwayat).
- [ ] Skor kandidat cold storage: jarak ke sentra produksi, volume surplus, kapasitas listrik, akses jalan (data kapasitas listrik/akses jalan belum ada; skor demo tetap statis).
- [x] Simulasi skenario pasokan (what-if) lewat `penyesuaian` per wilayah (ton) di panel optimasi `/peta`; skenario tidak dapat disimpan. Skenario cuaca/kebijakan, filter periode di UI, dan klik wilayah → detail (tren, EWS, stok terdekat) belum ada.
- [x] Tes optimasi: kasus hitung tangan (kalahkan serakah 40 vs 1010 ton-km, 450 ton-km seimbang, pasokan kurang, batas waktu, resolusi 0,1 ton), 300 instans acak dibandingkan brute force, kendala kapasitas; usecase dan integrasi (izin, validasi, simpan/gantikan, cache jarak, migrasi Up/Down). Tes komponen peta (data kosong/banyak titik) belum ada.
