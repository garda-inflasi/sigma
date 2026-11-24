import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it } from "node:test";
import { COLD_STORAGE, KABUPATEN, PASAR } from "@/lib/data/reference";
import {
  BATAS_ACEH,
  BATAS_GESER,
  diDalamBatasAceh,
  formatDelta,
  lngLatKeLatLng,
} from "./aceh";

type Cincin = [number, number][];
type Geometri =
  | { type: "Polygon"; coordinates: Cincin[] }
  | { type: "MultiPolygon"; coordinates: Cincin[][] };

const geojson = JSON.parse(
  readFileSync(
    join(process.cwd(), "public/geojson/aceh-kabupaten.geojson"),
    "utf8",
  ),
) as { features: { properties: { kode: string }; geometry: Geometri }[] };

const poligon = (g: Geometri) =>
  g.type === "Polygon" ? [g.coordinates] : g.coordinates;

function dalamCincin(r: Cincin, lat: number, lng: number) {
  let dalam = false;
  for (let i = 0, j = r.length - 1; i < r.length; j = i++) {
    const [xi, yi] = r[i];
    const [xj, yj] = r[j];
    if (
      yi > lat !== yj > lat &&
      lng < ((xj - xi) * (lat - yi)) / (yj - yi) + xi
    )
      dalam = !dalam;
  }
  return dalam;
}

const wilayahDi = (lat: number, lng: number) =>
  geojson.features
    .filter((f) =>
      poligon(f.geometry).some(
        (p) =>
          dalamCincin(p[0], lat, lng) &&
          !p.slice(1).some((h) => dalamCincin(h, lat, lng)),
      ),
    )
    .map((f) => f.properties.kode);

describe("geografi Aceh", () => {
  it("GeoJSON memuat tepat 23 kabupaten/kota dengan sumbu [lng, lat]", () => {
    assert.equal(geojson.features.length, 23);
    assert.equal(
      new Set(geojson.features.map((f) => f.properties.kode)).size,
      23,
    );
    for (const f of geojson.features)
      for (const p of poligon(f.geometry))
        for (const r of p)
          for (const [lng, lat] of r) {
            assert.ok(
              lng > 94 && lng < 99,
              `${f.properties.kode}: bujur ${lng}`,
            );
            assert.ok(
              lat > 1.5 && lat < 6.5,
              `${f.properties.kode}: lintang ${lat}`,
            );
          }
  });

  it("konversi GeoJSON ke Leaflet menukar sumbu menjadi [lat, lng]", () => {
    assert.deepEqual(lngLatKeLatLng([95.32, 5.55]), [5.55, 95.32]);
  });

  it("pusat kabupaten/kota berada tepat di dalam poligon wilayahnya", () => {
    for (const k of KABUPATEN)
      assert.deepEqual(wilayahDi(k.lat, k.lng), [k.kode], k.nama);
  });

  it("pasar dan cold storage berada di kabupaten/kota yang tercatat", () => {
    for (const p of PASAR)
      assert.deepEqual(wilayahDi(p.lat, p.lng), [p.kabupatenKode], p.nama);
    for (const c of COLD_STORAGE)
      assert.deepEqual(wilayahDi(c.lat, c.lng), [c.kabupatenKode], c.nama);
  });

  it("kotak Aceh memuat semua pusat wilayah dan berada di dalam batas geser", () => {
    for (const k of KABUPATEN)
      assert.ok(diDalamBatasAceh(k.lat, k.lng), k.nama);
    assert.ok(
      BATAS_GESER[0][0] < BATAS_ACEH[0][0] &&
        BATAS_GESER[0][1] < BATAS_ACEH[0][1],
    );
    assert.ok(
      BATAS_GESER[1][0] > BATAS_ACEH[1][0] &&
        BATAS_GESER[1][1] > BATAS_ACEH[1][1],
    );
    assert.equal(
      diDalamBatasAceh(95.32, 5.55),
      false,
      "sumbu tertukar harus ditolak",
    );
  });

  it("memformat perubahan harga bertanda", () => {
    assert.equal(formatDelta(12.4), "+12,4%");
    assert.equal(formatDelta(-3), "−3,0%");
    assert.equal(formatDelta(0), "0,0%");
  });
});
