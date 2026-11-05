import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, it } from "node:test";
import { KABUPATEN, KOMODITAS, PASAR } from "./reference";

/** Cermin seed master.reference di API Go (lihat internal/master/seeder/seeder_test.go). */
const seed = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../../packages/contracts/master-reference.json"),
    "utf8",
  ),
) as {
  kabupaten: {
    kode: string;
    nama: string;
    jenis: string;
    ibukota: string;
    lat: number;
    lng: number;
  }[];
  pasar: {
    id: string;
    nama: string;
    kabupaten_kode: string;
    lat: number;
    lng: number;
    radius_meter: number;
  }[];
  komoditas: {
    kode: string;
    nama: string;
    satuan: string;
    kategori: string;
    harga_acuan: number;
    pilot: boolean;
    shelf_life_hari: number;
  }[];
};

describe("reference.ts sama dengan seed master.reference", () => {
  it("kode kabupaten/kota sama persis", () => {
    assert.deepEqual(
      KABUPATEN.map((k) => k.kode).sort(),
      seed.kabupaten.map((k) => k.kode).sort(),
    );
  });

  it("jenis, ibu kota, dan koordinat kabupaten sama; nama resmi = awalan jenis + nama", () => {
    for (const k of KABUPATEN) {
      const s = seed.kabupaten.find((x) => x.kode === k.kode);
      assert.ok(s, `kabupaten ${k.kode} tidak ada di seed`);
      assert.equal(s.jenis, k.jenis, k.kode);
      assert.equal(s.ibukota, k.ibukota, k.kode);
      assert.equal(s.lat, k.lat, k.kode);
      assert.equal(s.lng, k.lng, k.kode);
      assert.equal(
        s.nama,
        `${k.jenis === "Kota" ? "KOTA" : "KABUPATEN"} ${k.nama.toUpperCase()}`,
        k.kode,
      );
    }
  });

  it("kode komoditas sama persis", () => {
    assert.deepEqual(
      KOMODITAS.map((k) => k.kode).sort(),
      seed.komoditas.map((k) => k.kode).sort(),
    );
  });

  it("nama, satuan, kategori, harga acuan, pilot, dan masa simpan komoditas sama", () => {
    for (const k of KOMODITAS) {
      const s = seed.komoditas.find((x) => x.kode === k.kode);
      assert.ok(s, `komoditas ${k.kode} tidak ada di seed`);
      assert.deepEqual(
        {
          nama: s.nama,
          satuan: s.satuan,
          kategori: s.kategori,
          hargaAcuan: s.harga_acuan,
          pilot: s.pilot,
          shelfLifeHari: s.shelf_life_hari,
        },
        {
          nama: k.nama,
          satuan: k.satuan,
          kategori: k.kategori,
          hargaAcuan: k.hargaAcuan,
          pilot: k.pilot,
          shelfLifeHari: k.shelfLifeHari,
        },
        k.kode,
      );
    }
  });

  it("setiap pasar di reference.ts ada di seed dengan kabupaten, koordinat, dan geofence sama", () => {
    for (const p of PASAR) {
      const s = seed.pasar.find((x) => x.id === p.id);
      assert.ok(s, `pasar ${p.id} tidak ada di seed`);
      assert.deepEqual(
        {
          nama: s.nama,
          kabupatenKode: s.kabupaten_kode,
          lat: s.lat,
          lng: s.lng,
          radiusMeter: s.radius_meter,
        },
        {
          nama: p.nama,
          kabupatenKode: p.kabupatenKode,
          lat: p.lat,
          lng: p.lng,
          radiusMeter: p.radiusMeter,
        },
        p.id,
      );
    }
  });

  it("seed menutup seluruh kabupaten dengan minimal satu pasar", () => {
    const berpasar = [
      ...new Set(seed.pasar.map((p) => p.kabupaten_kode)),
    ].sort();
    assert.deepEqual(berpasar, KABUPATEN.map((k) => k.kode).sort());
  });
});
