import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  estimasiJalanKm,
  estimasiMenitTempuh,
  haversineKm,
  risikoFoodLossDariMenit,
} from "./aceh";

describe("perhitungan jarak spasial aceh", () => {
  it("menghitung jarak haversine Takengon ke Banda Aceh secara akurat", () => {
    const lurus = haversineKm(4.627, 96.846, 5.5577, 95.3222);
    assert.ok(lurus > 180 && lurus < 215);
    const jalan = estimasiJalanKm(lurus, 1.28);
    assert.ok(jalan > lurus);
    const menit = estimasiMenitTempuh(jalan);
    assert.ok(menit > 240);
    assert.equal(risikoFoodLossDariMenit(menit), "TINGGI");
  });

  it("mengklasifikasikan risiko rendah untuk perjalanan singkat", () => {
    const lurus = haversineKm(5.5603, 95.3237, 5.5085, 95.364);
    const jalan = estimasiJalanKm(lurus);
    const menit = estimasiMenitTempuh(jalan);
    assert.equal(risikoFoodLossDariMenit(menit), "RENDAH");
  });
});
