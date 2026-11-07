import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ENTITAS, adalahEntitas } from "./master";

describe("ENTITAS", () => {
  it("memuat tab referensi baru dan tidak memuat 'perubahan' (rute statis tersendiri)", () => {
    assert.deepEqual(Object.keys(ENTITAS), [
      "komoditas",
      "wilayah",
      "pasar",
      "kecamatan",
      "satuan",
      "pedagang",
    ]);
    assert.equal("perubahan" in ENTITAS, false);
  });

  it("adalahEntitas menolak 'perubahan' dan nama dari prototipe Object", () => {
    assert.equal(adalahEntitas("kecamatan"), true);
    assert.equal(adalahEntitas("perubahan"), false);
    assert.equal(adalahEntitas("constructor"), false);
    assert.equal(adalahEntitas("toString"), false);
    assert.equal(adalahEntitas(""), false);
  });
});
