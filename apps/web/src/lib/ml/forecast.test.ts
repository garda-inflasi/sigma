import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { toLampiranMl } from "@/lib/api/ml";
import {
  PREDIKSI_DETAIL_C05,
  PREDIKSI_RINGKAS_C05,
} from "@/lib/api/__fixtures__/ml/nyata";
import {
  akhiranSatuan,
  barisDariForecast,
  hariIniWib,
  ringkasForecastPerWilayah,
  seriGrafikForecast,
} from "./forecast";

const detail = toLampiranMl(PREDIKSI_DETAIL_C05).forecast!.item!;
const ringkas = toLampiranMl(PREDIKSI_RINGKAS_C05).forecastRingkas!;

describe("seriGrafikForecast", () => {
  it("riwayat asli, jangkar harga terakhir, lalu P50 dan pita P10–P90 tiap horizon", () => {
    const seri = seriGrafikForecast(detail);
    const ts = seri.map((s) => s.ts);
    assert.deepEqual(
      ts,
      [...ts].sort((a, b) => a - b),
    );
    const jangkar = seri.find((s) => s.tanggal === detail.tanggalTerakhir)!;
    assert.equal(jangkar.aktual, detail.hargaTerakhir);
    assert.deepEqual(jangkar.rentang, [
      detail.hargaTerakhir,
      detail.hargaTerakhir,
    ]);
    const proyeksi = seri.filter((s) => s.aktual === undefined);
    assert.equal(proyeksi.length, 3);
    for (const [i, t] of detail.kurva.entries()) {
      assert.equal(proyeksi[i].p50, t.p50);
      assert.deepEqual(proyeksi[i].rentang, [t.p10, t.p90]);
    }
  });

  it("titik tanpa P50 tidak digambar dan pita tanpa P10/P90 dikosongkan", () => {
    const seri = seriGrafikForecast({
      riwayat: [{ tanggal: "2026-11-22", nilai: null }],
      hargaTerakhir: null,
      tanggalTerakhir: null,
      kurva: [
        {
          horizon: 7,
          tanggal: "2026-12-01",
          p10: null,
          p50: 10,
          p90: 12,
          perubahanP50Pct: null,
        },
        {
          horizon: 14,
          tanggal: "2026-12-08",
          p10: 9,
          p50: null,
          p90: 11,
          perubahanP50Pct: null,
        },
      ],
    });
    assert.equal(seri.length, 1);
    assert.equal(seri[0].rentang, undefined);
  });
});

describe("olahan forecast", () => {
  it("ringkasan per wilayah memakai kesegaran harga terakhir dan menandai persistensi", () => {
    const rows = ringkasForecastPerWilayah(ringkas.items, "2026-11-24");
    assert.equal(rows.length, 1);
    const r = rows[0];
    assert.equal(r.kabupatenKode, "1171");
    assert.equal(r.jumlahKomoditas, ringkas.items.length);
    assert.equal(r.tanggalTerakhir, "2026-11-23");
    assert.equal(r.umurHari, 1);
    assert.equal(
      r.jumlahPersistensi,
      ringkas.items.filter((f) => f.model.adalahBaseline).length,
    );
  });

  it("baris tabel lama dari forecast H+14", () => {
    const b = barisDariForecast(detail);
    const t14 = detail.kurva.find((t) => t.horizon === 14)!;
    assert.equal(b.id, "CBM-KRT");
    assert.equal(b.proyeksi, Math.round(t14.p50!));
    assert.equal(b.pct, t14.perubahanP50Pct);
    assert.equal(b.demo, false);
  });

  it("satuan dan hari WIB", () => {
    assert.equal(akhiranSatuan("Rp/kg"), "/kg");
    assert.equal(akhiranSatuan("persen"), " persen");
    assert.equal(hariIniWib(new Date("2026-11-24T17:30:00Z")), "2026-11-25");
  });
});
